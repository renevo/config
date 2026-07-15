package config

import (
	"errors"
	"fmt"
	"io"
	"reflect"
	"strings"
	"sync"
	"text/tabwriter"
)

// Set is a hierarchical collection of configuration settings.
type Set struct {
	name       string
	path       string
	root       *Set
	parent     *Set
	mu         sync.RWMutex
	locked     bool
	loaded     bool
	revision   uint64
	validators []SetValidator
	children   sync.Map
	settings   sync.Map
	notifiers  sync.Map
}

type committedChange struct {
	setting *Setting
	old     string
}

func (s *Set) stateRoot() *Set {
	if s.root == nil {
		return s
	}
	return s.root
}

// Lock freezes the schema and prevents changes to lockable settings.
func (s *Set) Lock() {
	root := s.stateRoot()
	root.mu.Lock()
	root.locked = true
	root.mu.Unlock()
}

// Locked reports whether the root configuration tree is locked.
func (s *Set) Locked() bool {
	root := s.stateRoot()
	root.mu.RLock()
	defer root.mu.RUnlock()
	return root.locked
}

// Loaded reports whether the initial configuration has been loaded.
func (s *Set) Loaded() bool {
	root := s.stateRoot()
	root.mu.RLock()
	defer root.mu.RUnlock()
	return root.loaded
}

// Revision returns the committed revision number of the root configuration.
func (s *Set) Revision() uint64 {
	root := s.stateRoot()
	root.mu.RLock()
	defer root.mu.RUnlock()
	return root.revision
}

func canonicalPath(path string) (string, error) {
	path = strings.TrimSpace(strings.TrimPrefix(path, "./"))
	if path == "" {
		return "", ErrInvalidPath
	}

	parts := strings.Split(path, ".")
	for i, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" || part == "." || part == ".." {
			return "", ErrInvalidPath
		}
		parts[i] = canonicalPathPart(part)
	}

	return strings.Join(parts, "."), nil
}

func canonicalPathPart(part string) string {
	result := strings.Builder{}
	upperNext := true
	for _, r := range strings.ToLower(part) {
		if r == '-' {
			result.WriteRune(r)
			upperNext = true
			continue
		}
		if upperNext {
			r = []rune(strings.ToUpper(string(r)))[0]
			upperNext = false
		}
		result.WriteRune(r)
	}
	return result.String()
}

// Get returns a setting by relative-first, case-insensitive path.
func (s *Set) Get(name string) *Setting {
	root := s.stateRoot()
	root.mu.RLock()
	defer root.mu.RUnlock()

	path := fmt.Sprintf("%s.%s", s.path, name)
	if canonical, err := canonicalPath(path); err == nil {
		if setting, found := root.settings.Load(strings.ToLower(canonical)); found {
			return setting.(*Setting)
		}
	}

	if canonical, err := canonicalPath(name); err == nil {
		if setting, found := root.settings.Load(strings.ToLower(canonical)); found {
			return setting.(*Setting)
		}
	}

	return nil
}

// Update parses and updates one setting. The boolean reports whether the path was found.
func (s *Set) Update(name, value string) (bool, error) {
	setting := s.Get(name)
	if setting == nil {
		return false, ErrNotFound
	}

	return true, setting.Set(value)
}

// AddValidator adds a validator for complete configuration candidates.
func (s *Set) AddValidator(validator SetValidator) error {
	if validator == nil {
		return ErrUnsupported
	}
	root := s.stateRoot()
	root.mu.Lock()
	defer root.mu.Unlock()
	if root.locked {
		return ErrSchemaLocked
	}
	root.validators = append(root.validators, validator)
	return nil
}

// Validate checks the currently committed configuration.
func (s *Set) Validate() error {
	root := s.stateRoot()
	root.mu.RLock()
	defer root.mu.RUnlock()

	values := make(map[string]any)
	var validationErrors []error
	root.settings.Range(func(key, value any) bool {
		setting := value.(*Setting)
		values[key.(string)] = setting.value
		if setting.validator != nil {
			if err := setting.validator(setting.value); err != nil {
				validationErrors = append(validationErrors, &SettingError{Path: setting.Path, Err: err})
			}
		}
		return true
	})
	candidate := Candidate{values: values}
	for _, validator := range root.validators {
		if err := validator(candidate); err != nil {
			validationErrors = append(validationErrors, err)
		}
	}
	return errors.Join(validationErrors...)
}

// Apply parses, validates, and commits multiple setting updates atomically.
func (s *Set) Apply(values map[string]string) error {
	root := s.stateRoot()
	root.mu.Lock()

	parsed := make(map[*Setting]any, len(values))
	candidateValues := make(map[string]any)
	var applyErrors []error
	root.settings.Range(func(key, value any) bool {
		candidateValues[key.(string)] = value.(*Setting).value
		return true
	})

	for path, text := range values {
		canonical, err := canonicalPath(path)
		if err != nil {
			applyErrors = append(applyErrors, &SettingError{Path: path, Err: err})
			continue
		}
		value, found := root.settings.Load(strings.ToLower(canonical))
		if !found {
			applyErrors = append(applyErrors, &SettingError{Path: canonical, Err: ErrNotFound})
			continue
		}
		setting := value.(*Setting)
		if setting.Lockable && root.locked {
			applyErrors = append(applyErrors, &LockedError{Path: setting.Path})
			continue
		}
		parsedValue, parseErr := setting.settingCodec().Parse(text)
		if parseErr != nil {
			applyErrors = append(applyErrors, &SettingError{Path: setting.Path, Err: parseErr})
			continue
		}
		parsed[setting] = parsedValue
		candidateValues[strings.ToLower(setting.Path)] = parsedValue
	}

	candidate := Candidate{values: candidateValues}
	for setting, value := range parsed {
		if setting.validator != nil {
			if err := setting.validator(value); err != nil {
				applyErrors = append(applyErrors, &SettingError{Path: setting.Path, Err: err})
			}
		}
		_ = value
	}
	for _, validator := range root.validators {
		if err := validator(candidate); err != nil {
			applyErrors = append(applyErrors, err)
		}
	}
	if err := errors.Join(applyErrors...); err != nil {
		root.mu.Unlock()
		return err
	}

	changed := make([]committedChange, 0, len(parsed))
	for setting, value := range parsed {
		if setting.equalsMust(value) {
			continue
		}
		oldValue := setting.stringValue()
		updated, err := setParsedValue(setting.value, value)
		if err != nil {
			root.mu.Unlock()
			return &SettingError{Path: setting.Path, Err: err}
		}
		setting.value = updated
		changed = append(changed, committedChange{setting: setting, old: oldValue})
	}
	if len(changed) != 0 {
		root.revision++
	}
	root.mu.Unlock()
	root.notifySettings(changed)
	return nil
}

// Subset returns an existing child set or creates a new child set.
func (s *Set) Subset(name string) *Set {
	root := s.stateRoot()
	canonicalName, err := canonicalPath(name)
	if err != nil {
		return nil
	}

	subsetPath := fmt.Sprintf("%s.%s", s.path, canonicalName)
	if s.path == "" {
		subsetPath = canonicalName
	}
	canonicalSubsetPath, err := canonicalPath(subsetPath)
	if err != nil {
		return nil
	}

	root.mu.Lock()
	defer root.mu.Unlock()
	if root.locked {
		return nil
	}

	if set, found := root.children.Load(strings.ToLower(canonicalSubsetPath)); found {
		return set.(*Set)
	}

	set := &Set{
		name:   canonicalName,
		path:   canonicalSubsetPath,
		root:   root,
		parent: s,
	}

	root.children.Store(strings.ToLower(canonicalSubsetPath), set)

	return set
}

// Path returns the canonical path of the set.
func (s *Set) Path() string {
	return s.path
}

// Name returns the canonical name of the set.
func (s *Set) Name() string {
	return s.name
}

// Root returns the root configuration set.
func (s *Set) Root() *Set {
	if s.root == nil {
		return s
	}

	return s.root
}

// Parent returns the parent set, or s for a root set.
func (s *Set) Parent() *Set {
	if s.parent == nil {
		return s
	}

	return s.parent
}

// Setting registers a setting in s. The name must not be empty and value must not be nil.
func (s *Set) Setting(name string, value any, description string, options ...SettingOption) *Setting {
	if name == "" {
		panic("name can not be empty")
	}
	if value == nil {
		panic("value can not be nil")
	}

	root := s.stateRoot()
	canonicalName, err := canonicalPath(name)
	if err != nil {
		panic(err)
	}

	settingPath := fmt.Sprintf("%s.%s", s.path, canonicalName)
	if s.path == "" {
		settingPath = canonicalName
	}
	canonicalSettingPath, err := canonicalPath(settingPath)
	if err != nil {
		panic(err)
	}

	root.mu.Lock()
	locked := true
	defer func() {
		if locked {
			root.mu.Unlock()
		}
	}()
	if root.locked {
		panic(ErrSchemaLocked)
	}

	setting := &Setting{
		Name:        canonicalName,
		Description: description,
		Path:        canonicalSettingPath,
		value:       value,
		owner:       root,
	}
	for _, option := range options {
		if option != nil {
			option(setting)
		}
	}

	// cheeky allows the underlying thing to actually map it properly
	setting.DefaultValue = setting.stringValue()

	_, exists := root.settings.LoadOrStore(strings.ToLower(canonicalSettingPath), setting)
	if exists {
		panic(fmt.Sprintf("setting %q already exists", canonicalSettingPath))
	}

	// get notified when the setting changes - we won't stop notifications as long as it is a child, and since there is no remove.... we just discard the Close handler
	_ = setting.Notify(NotifyFunc(s.notifyChanged))

	root.mu.Unlock()
	locked = false
	// Notify after releasing the registry lock so callbacks can safely inspect or update the set.
	s.notifyChanged(setting)
	return setting
}

// Range visits settings below s in canonical path order-independent registry order.
func (s *Set) Range(fn func(string, *Setting) bool) {
	root := s.stateRoot()
	root.mu.RLock()
	defer root.mu.RUnlock()

	root.settings.Range(func(k, v any) bool {
		key := k.(string)
		setting := v.(*Setting)

		prefix := strings.ToLower(s.path)
		if prefix != "" && key != prefix && !strings.HasPrefix(key, prefix+".") {
			return true
		}

		return fn(setting.Path, setting)
	})
}

// Bind binds a pointer to a struct into s.
//
// Fields can use setting, description, mask, and flag tags. Bound fields are
// package-owned after binding and should be treated as read-only by callers.
// Flags are only added when the FlagSet option is provided.
func (s *Set) Bind(value any, bindOptions ...BindOption) error {
	if s.Locked() {
		return ErrSchemaLocked
	}
	options := BindOptions{}
	for _, option := range bindOptions {
		if option != nil {
			option(&options)
		}
	}
	rvalue := reflect.ValueOf(value)

	if !rvalue.IsValid() || rvalue.Kind() != reflect.Pointer || rvalue.IsNil() {
		return errors.New("value must be a non-nil pointer")
	}

	rvalue = rvalue.Elem()

	if rvalue.Kind() != reflect.Struct {
		return errors.New("value must be a pointer to a struct")
	}

	for i := 0; i < rvalue.NumField(); i++ {
		fieldType := rvalue.Type().Field(i)
		fieldValue := rvalue.Field(i)

		if !fieldValue.CanSet() {
			continue
		}

		description := fieldType.Tag.Get("description")
		name := fieldType.Name
		masked := fieldType.Tag.Get("mask") == "true"
		flagName := fieldType.Tag.Get("flag")

		if tagName := fieldType.Tag.Get("setting"); tagName != "" {
			name = tagName
		}

		if name == "-" {
			continue
		}

		switch rvalue.Field(i).Kind() {
		case reflect.Invalid, reflect.Chan, reflect.Func:
			// do nothing

		case reflect.Pointer:
			if fieldValue.IsNil() {
				fieldValue.Set(reflect.New(fieldValue.Type().Elem()))
			}
			if fieldValue.Type().Elem().Kind() == reflect.Struct {
				child := s.Subset(name)
				if child == nil {
					return ErrSchemaLocked
				}
				if err := child.Bind(fieldValue.Interface(), bindOptions...); err != nil {
					return err
				}
			} else {
				setting := s.Setting(name, fieldValue.Interface(), description)
				setting.Mask = masked
				if flagName != "" {
					setting.SetFlag(flagName, options.FlagSet)
				}
			}

		case reflect.Struct:
			if options.FlattenAnonymous && fieldType.Anonymous {
				if err := s.Bind(fieldValue.Addr().Interface(), bindOptions...); err != nil {
					return err
				}
			} else {
				child := s.Subset(name)
				if child == nil {
					return ErrSchemaLocked
				}
				if err := child.Bind(fieldValue.Addr().Interface(), bindOptions...); err != nil {
					return err
				}
			}

		default:
			// all other field types we pass in the pointer to the value as a setting so that it is "bound"
			setting := s.Setting(name, fieldValue.Addr().Interface(), description)
			setting.Mask = masked

			// does it have a flag?
			if flagName != "" {
				setting.SetFlag(flagName, options.FlagSet)
			}
		}
	}

	return nil
}

// Dump writes the current settings as a tab-separated report.
func (s *Set) Dump(w io.Writer) error {
	tw := tabwriter.NewWriter(w, 10, 10, 5, ' ', 0)
	snapshot, err := s.Snapshot()
	if err != nil {
		return err
	}

	// print header
	if _, err := fmt.Fprintln(tw, "Path\tType\tValue\tDefault Value\tDescription"); err != nil {
		return err
	}

	// print items
	for _, setting := range snapshot.Values {
		if _, err := fmt.Fprintf(tw, "%s\t%s\t%q\t%q\t%s\n", setting.Path, setting.Type, setting.Value, setting.DefaultValue, setting.Description); err != nil {
			return err
		}
	}

	return tw.Flush()
}

// Notify subscribes to setting additions and changes in s and its children.
func (s *Set) Notify(n Notifier) *NotifyHandle {
	if n == nil {
		return &NotifyHandle{}
	}

	handle := &NotifyHandle{
		stopFunc: s.notifiers.Delete,
	}

	s.notifiers.Store(handle, n)

	return handle
}

// notifyChanged is attached to all settings so that we can get notified of when they are added
func (s *Set) notifyChanged(setting *Setting) {
	s.notifiers.Range(func(k, v any) bool {
		notifier := v.(Notifier)
		notifier.Notify(setting)
		return true
	})

	// call the parent to notify if they exist to propagate upward the notification
	if s.parent != nil {
		s.parent.notifyChanged(setting)
	}
}
