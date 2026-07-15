package config

import (
	"flag"
	"fmt"
	"strings"
	"sync"
)

// Setting describes one registered configuration value.
type Setting struct {
	// Mask causes String and snapshots to hide the current and default values.
	Mask bool

	// Name is the local name of the setting.
	Name string

	// Description is user-facing text suitable for help output.
	Description string

	// DefaultValue is the formatted value used before sources are applied.
	DefaultValue string

	// Path is the canonical dot-separated path of the setting.
	Path string

	// Flag is the command-line flag name for the setting. It is optional.
	Flag string

	// Lockable marks a setting that cannot change after its owning set is locked.
	Lockable bool

	value           any
	validator       Validator
	codec           Codec
	notifiers       sync.Map
	changeNotifiers sync.Map
	owner           *Set
}

// IsDefault reports whether the current value equals DefaultValue.
func (s *Setting) IsDefault() bool {
	return s.Equals(s.DefaultValue)
}

// Notify subscribes to setting change notifications.
func (s *Setting) Notify(n Notifier) *NotifyHandle {
	if n == nil {
		return &NotifyHandle{}
	}
	handle := &NotifyHandle{stopFunc: s.notifiers.Delete}
	s.notifiers.Store(handle, n)
	return handle
}

func (s *Setting) equalsMust(value any) bool {
	return s.settingCodec().Equal(s.value, value)
}

func (s *Setting) settingCodec() Codec {
	if s.codec != nil {
		return s.codec
	}
	return codecFor(s.value)
}

func (s *Setting) notify() {
	s.notifiers.Range(func(key, value any) bool {
		notifier, ok := value.(Notifier)
		if !ok || notifier == nil {
			s.notifiers.Delete(key)
			return true
		}
		notifier.Notify(s)
		return true
	})
}

// NotifyChange subscribes to stable old/new change records.
func (s *Setting) NotifyChange(n ChangeNotifier) *NotifyHandle {
	if n == nil {
		return &NotifyHandle{}
	}
	handle := &NotifyHandle{stopFunc: s.changeNotifiers.Delete}
	s.changeNotifiers.Store(handle, n)
	return handle
}

func (s *Setting) notifyChange(oldValue string) {
	revision := uint64(0)
	if s.owner != nil {
		revision = s.owner.Revision()
	}
	change := Change{Path: s.Path, Old: oldValue, New: s.String(), Revision: revision}
	s.changeNotifiers.Range(func(key, value any) bool {
		notifier, ok := value.(ChangeNotifier)
		if !ok || notifier == nil {
			s.changeNotifiers.Delete(key)
			return true
		}
		notifier.NotifyChange(change)
		return true
	})
}

// Set parses and commits a new value from text.
func (s *Setting) Set(text string) error {
	var owner *Set
	if s.owner != nil {
		s.owner.mu.Lock()
		owner = s.owner
		if s.Lockable && s.owner.locked {
			s.owner.mu.Unlock()
			return &LockedError{Path: s.Path}
		}
	}
	unlock := func() {
		if owner != nil {
			owner.mu.Unlock()
			owner = nil
		}
	}
	defer unlock()

	parsed, err := s.settingCodec().Parse(text)
	if err != nil {
		return &SettingError{Path: s.Path, Err: err}
	}
	if s.settingCodec().Equal(s.value, parsed) {
		return nil
	}

	oldValue := s.stringValue()
	updated, err := setParsedValue(s.value, parsed)
	if err != nil {
		return &SettingError{Path: s.Path, Err: err}
	}
	s.value = updated
	if s.owner != nil {
		s.owner.revision++
	}
	unlock()
	s.notifyChange(oldValue)
	s.notify()
	return nil
}

// String formats the current value, masking it when Mask is true.
func (s *Setting) String() string {
	if s.owner != nil {
		s.owner.mu.RLock()
		defer s.owner.mu.RUnlock()
	}
	return s.stringValue()
}

func (s *Setting) stringValue() string {
	if s.Mask {
		return "*****"
	}
	formatted, err := s.settingCodec().Format(s.value)
	if err != nil {
		return fmt.Sprintf("%v", s.value)
	}
	return formatted
}

// Equals reports whether text represents the current value.
func (s *Setting) Equals(text string) bool {
	if s.owner != nil {
		s.owner.mu.RLock()
		defer s.owner.mu.RUnlock()
	}
	return s.equals(text)
}

func (s *Setting) equals(text string) bool {
	parsed, err := s.settingCodec().Parse(text)
	if err != nil {
		return false
	}
	return s.settingCodec().Equal(s.value, parsed)
}

// Type returns the setting type name without a pointer prefix.
// It satisfies the value contract used by github.com/spf13/pflag.
func (s *Setting) Type() string {
	if s.owner != nil {
		s.owner.mu.RLock()
		defer s.owner.mu.RUnlock()
	}
	return strings.TrimLeft(fmt.Sprintf("%T", s.value), "*")
}

// IsBoolFlag reports whether the setting can be used as a boolean flag without a value.
func (s *Setting) IsBoolFlag() bool {
	if s.owner != nil {
		s.owner.mu.RLock()
		defer s.owner.mu.RUnlock()
	}
	return s.value != nil && strings.TrimLeft(fmt.Sprintf("%T", s.value), "*") == "bool"
}

// SetFlag registers the setting in fs and sets the Flag field. If fs is nil, the flag is not registered.
func (s *Setting) SetFlag(name string, fs *flag.FlagSet) {
	s.Flag = name

	if fs == nil {
		return
	}
	fs.Var(s, name, s.Description)
}
