package config

import "errors"

var (
	// ErrNotFound reports that a configuration setting or path does not exist.
	ErrNotFound = errors.New("configuration setting not found")
	// ErrAlreadyExists reports an attempt to register a duplicate setting.
	ErrAlreadyExists = errors.New("configuration setting already exists")
	// ErrInvalidPath reports a malformed configuration path.
	ErrInvalidPath = errors.New("invalid configuration path")
	// ErrNotLoaded reports an operation that requires an initial load.
	ErrNotLoaded = errors.New("configuration has not been loaded")
	// ErrAlreadyLoaded reports a second call to Load.
	ErrAlreadyLoaded = errors.New("configuration has already been loaded")
	// ErrLocked reports an attempted change to a locked setting.
	ErrLocked = errors.New("configuration setting is locked")
	// ErrSchemaLocked reports an attempt to change a locked schema.
	ErrSchemaLocked = errors.New("configuration schema is locked")
	// ErrUnsupported reports an unsupported setting type or operation.
	ErrUnsupported = errors.New("unsupported configuration type")
)

// SettingError adds path and source context to an underlying error.
type SettingError struct {
	// Path is the canonical setting path associated with the error.
	Path string
	// Source identifies the source that produced the error, when applicable.
	Source string
	// Err is the underlying cause.
	Err error
}

// Error returns the contextual error message.
func (e *SettingError) Error() string {
	if e.Source == "" {
		return e.Path + ": " + e.Err.Error()
	}
	return e.Source + " " + e.Path + ": " + e.Err.Error()
}

// Unwrap returns the underlying cause.
func (e *SettingError) Unwrap() error { return e.Err }

// LockedError identifies a lockable setting that could not be changed.
type LockedError struct {
	// Path is the canonical path of the locked setting.
	Path string
}

// Error returns the locked setting error message.
func (e *LockedError) Error() string { return e.Path + ": " + ErrLocked.Error() }

// Unwrap returns ErrLocked.
func (e *LockedError) Unwrap() error { return ErrLocked }
