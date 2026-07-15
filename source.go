package config

import (
	"context"
	"errors"
	"fmt"
	"iter"
	"reflect"
	"slices"
	"strings"
	"sync"
)

// SettingMetadata describes a registered setting without exposing its value or mutation APIs.
type SettingMetadata struct {
	// Path is the canonical setting path.
	Path string
	// Name is the setting name relative to its containing set.
	Name string
	// Description is the setting's user-facing description.
	Description string
	// Type is the registered Go type.
	Type reflect.Type
	// Masked reports whether the setting hides its value.
	Masked bool
	// Lockable reports whether locking the set prevents changes to the setting.
	Lockable bool
}

// Source supplies raw configuration values for Load and Reload.
type Source interface {
	// Load returns the raw values currently supplied by the source. Settings is
	// a replayable, order-unspecified snapshot of registered setting metadata.
	Load(context.Context, iter.Seq[SettingMetadata]) ([]RawValue, error)
}

// RawValue is one source-provided configuration value.
type RawValue struct {
	// Path is the source-provided setting path.
	Path string
	// Value is the unparsed setting text. An empty value is explicit.
	Value string
	// Source identifies the source for diagnostics.
	Source string
}

// SourceFunc adapts a function to Source.
type SourceFunc func(context.Context, iter.Seq[SettingMetadata]) ([]RawValue, error)

// Load implements Source.
func (source SourceFunc) Load(ctx context.Context, settings iter.Seq[SettingMetadata]) ([]RawValue, error) {
	return source(ctx, settings)
}

// ValuesSource creates a source from a map. Map iteration order does not affect precedence.
func ValuesSource(name string, values map[string]string) Source {
	return SourceFunc(func(context.Context, iter.Seq[SettingMetadata]) ([]RawValue, error) {
		result := make([]RawValue, 0, len(values))
		for path, value := range values {
			result = append(result, RawValue{Path: path, Value: value, Source: name})
		}
		return result, nil
	})
}

// RuntimeSource is a named, mutable source layer intended to survive reloads.
type RuntimeSource struct {
	name   string
	mu     sync.RWMutex
	values map[string]string
}

// NewRuntimeSource creates a named runtime override source.
func NewRuntimeSource(name string) *RuntimeSource {
	return &RuntimeSource{name: name, values: make(map[string]string)}
}

// Set assigns an override in the runtime source.
func (source *RuntimeSource) Set(path, value string) error {
	canonical, err := canonicalPath(path)
	if err != nil {
		return err
	}
	source.mu.Lock()
	source.values[strings.ToLower(canonical)] = value
	source.mu.Unlock()
	return nil
}

// Delete removes an override from the runtime source.
func (source *RuntimeSource) Delete(path string) error {
	canonical, err := canonicalPath(path)
	if err != nil {
		return err
	}
	source.mu.Lock()
	delete(source.values, strings.ToLower(canonical))
	source.mu.Unlock()
	return nil
}

// Load implements Source.
func (source *RuntimeSource) Load(context.Context, iter.Seq[SettingMetadata]) ([]RawValue, error) {
	source.mu.RLock()
	defer source.mu.RUnlock()
	result := make([]RawValue, 0, len(source.values))
	for path, value := range source.values {
		result = append(result, RawValue{Path: path, Value: value, Source: source.name})
	}
	return result, nil
}

// Load establishes the initial configuration from defaults and sources.
func (s *Set) Load(ctx context.Context, sources ...Source) error {
	root := s.stateRoot()
	root.mu.Lock()
	if root.loaded {
		root.mu.Unlock()
		return ErrAlreadyLoaded
	}
	root.mu.Unlock()

	values, err := root.sourceValues(ctx, sources...)
	if err != nil {
		return err
	}
	root.mu.Lock()
	if root.loaded {
		root.mu.Unlock()
		return ErrAlreadyLoaded
	}
	changed, err := root.commitSourceValues(values, true)
	if err == nil {
		root.loaded = true
	}
	root.mu.Unlock()
	if err != nil {
		return err
	}
	root.notifySettings(changed)
	return nil
}

// Reload replaces the loaded configuration from defaults and sources atomically.
func (s *Set) Reload(ctx context.Context, sources ...Source) error {
	root := s.stateRoot()
	root.mu.RLock()
	loaded := root.loaded
	root.mu.RUnlock()
	if !loaded {
		return ErrNotLoaded
	}

	values, err := root.sourceValues(ctx, sources...)
	if err != nil {
		return err
	}
	root.mu.Lock()
	if !root.loaded {
		root.mu.Unlock()
		return ErrNotLoaded
	}
	changed, err := root.commitSourceValues(values, false)
	root.mu.Unlock()
	if err != nil {
		return err
	}
	root.notifySettings(changed)
	return nil
}

func (s *Set) sourceValues(ctx context.Context, sources ...Source) (map[string]RawValue, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	values := make(map[string]RawValue)
	metadata := make([]SettingMetadata, 0)
	root := s.stateRoot()
	root.mu.RLock()
	root.settings.Range(func(key, value any) bool {
		setting := value.(*Setting)
		values[key.(string)] = RawValue{Path: setting.Path, Value: setting.DefaultValue, Source: "default"}
		metadata = append(metadata, SettingMetadata{
			Path:        setting.Path,
			Name:        setting.Name,
			Description: setting.Description,
			Type:        setting.ValueType(),
			Masked:      setting.Mask,
			Lockable:    setting.Lockable,
		})
		return true
	})
	root.mu.RUnlock()

	for index, source := range sources {
		if source == nil {
			continue
		}
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		rawValues, err := source.Load(ctx, slices.Values(metadata))
		if err != nil {
			return nil, fmt.Errorf("source %d: %w", index, err)
		}
		for _, raw := range rawValues {
			canonical, err := canonicalPath(raw.Path)
			if err != nil {
				return nil, &SettingError{Path: raw.Path, Source: raw.Source, Err: err}
			}
			raw.Path = canonical
			if raw.Source == "" {
				raw.Source = fmt.Sprintf("source-%d", index)
			}
			key := strings.ToLower(canonical)
			if _, found := values[key]; !found {
				return nil, &SettingError{Path: canonical, Source: raw.Source, Err: ErrNotFound}
			}
			values[key] = raw
		}
	}
	return values, nil
}

func (s *Set) commitSourceValues(values map[string]RawValue, initial bool) ([]committedChange, error) {
	var applyErrors []error
	parsed := make(map[*Setting]any)
	candidate := make(map[string]any)
	s.settings.Range(func(key, value any) bool {
		setting := value.(*Setting)
		candidate[key.(string)] = setting.value
		raw := values[key.(string)]
		parsedValue, err := setting.settingCodec().Parse(raw.Value)
		if err != nil {
			applyErrors = append(applyErrors, &SettingError{Path: setting.Path, Source: raw.Source, Err: err})
			return true
		}
		if setting.Lockable && s.locked && !setting.equalsMust(parsedValue) {
			applyErrors = append(applyErrors, &LockedError{Path: setting.Path})
			return true
		}
		parsed[setting] = parsedValue
		candidate[key.(string)] = parsedValue
		return true
	})
	candidateView := Candidate{values: candidate}
	for setting, value := range parsed {
		if setting.validator != nil {
			if err := setting.validator(value); err != nil {
				applyErrors = append(applyErrors, &SettingError{Path: setting.Path, Err: err})
			}
		}
	}
	for _, validator := range s.validators {
		if err := validator(candidateView); err != nil {
			applyErrors = append(applyErrors, err)
		}
	}
	if err := errors.Join(applyErrors...); err != nil {
		return nil, err
	}

	changed := make([]committedChange, 0, len(parsed))
	for setting, value := range parsed {
		if setting.equalsMust(value) {
			continue
		}
		oldValue := setting.stringValue()
		updated, err := setParsedValue(setting.value, value)
		if err != nil {
			return nil, &SettingError{Path: setting.Path, Err: err}
		}
		setting.value = updated
		changed = append(changed, committedChange{setting: setting, old: oldValue})
	}
	if initial || len(changed) > 0 {
		s.revision++
	}
	return changed, nil
}

func (s *Set) notifySettings(changes []committedChange) {
	for _, change := range changes {
		change.setting.notifyChange(change.old)
		change.setting.notify()
	}
}
