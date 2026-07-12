package config

import (
	"fmt"
	"sort"
)

// SettingSnapshot is an immutable textual view of one setting at a revision.
type SettingSnapshot struct {
	// Path is the canonical setting path.
	Path string
	// Type is the registered Go type.
	Type string
	// Value is the formatted current value, masked when requested.
	Value string
	// DefaultValue is the formatted default value, masked when requested.
	DefaultValue string
	// Description is the setting's user-facing description.
	Description string
	// Masked reports whether the value was masked.
	Masked bool
}

// Snapshot contains settings observed at one committed revision.
type Snapshot struct {
	// Revision is the configuration revision represented by Values.
	Revision uint64
	// Values contains settings sorted by canonical path.
	Values []SettingSnapshot
}

// Snapshot returns a coherent view of the configuration at one revision.
func (s *Set) Snapshot() (Snapshot, error) {
	root := s.stateRoot()
	root.mu.RLock()
	defer root.mu.RUnlock()

	values := make([]SettingSnapshot, 0)
	root.settings.Range(func(_, value any) bool {
		setting := value.(*Setting)
		current := setting.stringValue()
		defaultValue := setting.DefaultValue
		if setting.Mask {
			current = "*****"
			defaultValue = "*****"
		}
		values = append(values, SettingSnapshot{
			Path:         setting.Path,
			Type:         fmt.Sprintf("%T", setting.value),
			Value:        current,
			DefaultValue: defaultValue,
			Description:  setting.Description,
			Masked:       setting.Mask,
		})
		return true
	})
	sort.Slice(values, func(i, j int) bool { return values[i].Path < values[j].Path })
	return Snapshot{Revision: root.revision, Values: values}, nil
}

// Value returns a setting's value when its dynamic type exactly matches T.
func Value[T any](setting *Setting) (T, error) {
	var zero T
	if setting == nil {
		return zero, ErrNotFound
	}
	if setting.owner != nil {
		setting.owner.mu.RLock()
		defer setting.owner.mu.RUnlock()
	}
	if setting.value == nil {
		return zero, ErrNotFound
	}
	value, ok := setting.value.(T)
	if !ok {
		return zero, fmt.Errorf("%w: setting %s has type %T, requested %T", ErrUnsupported, setting.Path, setting.value, zero)
	}
	return value, nil
}

// Lookup finds a setting using relative-first, case-insensitive path lookup.
func (s *Set) Lookup(path string) (*Setting, error) {
	setting := s.Get(path)
	if setting == nil {
		return nil, &SettingError{Path: path, Err: ErrNotFound}
	}
	return setting, nil
}

// LookupRelative finds a setting relative to s.
func (s *Set) LookupRelative(name string) (*Setting, error) {
	path := name
	if s.path != "" {
		path = s.path + "." + name
	}
	return s.Lookup(path)
}

// LookupAbsolute finds a setting from the root configuration path.
func (s *Set) LookupAbsolute(path string) (*Setting, error) {
	return s.stateRoot().Lookup(path)
}
