package config

import (
	"flag"
	"strings"
)

// Validator checks one setting's parsed value before a transaction commits.
type Validator func(any) error

// Candidate exposes the complete parsed configuration candidate to set validators.
type Candidate struct {
	values map[string]any
}

// Get returns a candidate value by canonical or case-insensitive path.
func (c Candidate) Get(path string) (any, bool) {
	canonical, err := canonicalPath(path)
	if err != nil {
		return nil, false
	}
	value, ok := c.values[strings.ToLower(canonical)]
	return value, ok
}

// SetValidator checks a complete candidate configuration before it commits.
type SetValidator func(Candidate) error

// SettingOption configures a setting at registration time.
type SettingOption func(*Setting)

// BindOptions configures how a struct is bound to a Set.
type BindOptions struct {
	// FlagSet receives flags declared by struct tags. The default is flag.CommandLine.
	FlagSet *flag.FlagSet
	// FlattenAnonymous binds fields of anonymous nested structs into the parent set.
	FlattenAnonymous bool
}

// BindOption customizes struct binding.
type BindOption func(*BindOptions)

// WithFlagSet registers bound flags in flagSet.
func WithFlagSet(flagSet *flag.FlagSet) BindOption {
	return func(options *BindOptions) { options.FlagSet = flagSet }
}

// FlattenAnonymous makes binding flatten anonymous nested structs.
func FlattenAnonymous() BindOption {
	return func(options *BindOptions) { options.FlattenAnonymous = true }
}

// WithValidator validates a setting value before a transaction commits.
func WithValidator(validator Validator) SettingOption {
	return func(setting *Setting) { setting.validator = validator }
}

// WithCodec uses codec to parse, format, and compare a setting value.
func WithCodec(codec Codec) SettingOption {
	return func(setting *Setting) { setting.codec = codec }
}

// WithLockable sets whether a setting is protected by Set.Lock.
func WithLockable(lockable bool) SettingOption {
	return func(setting *Setting) { setting.Lockable = lockable }
}

// Lockable marks a setting as protected by Set.Lock.
func Lockable() SettingOption { return WithLockable(true) }
