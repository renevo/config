# Configuration Package

[![GoDoc](https://godoc.org/github.com/renevo/config?status.svg)](https://godoc.org/github.com/renevo/config)
[![Go Report Card](https://goreportcard.com/badge/github.com/renevo/config)](https://goreportcard.com/report/github.com/renevo/config)
[![Test](https://github.com/renevo/config/actions/workflows/test.yml/badge.svg)](https://github.com/renevo/config/actions/workflows/test.yml)

`github.com/renevo/config` provides a typed configuration tree with explicit
lifecycle, source precedence, validation, snapshots, and runtime locking.

## Lifecycle

Register the schema before loading values:

```go
settings := config.NewSet()
port := settings.Setting("HTTP.Port", 8080, "HTTP listener port", config.Lockable())
settings.Setting("Debug.Enabled", false, "Enable debug behavior")

if err := settings.Load(ctx, config.ValuesSource("file", values)); err != nil {
	panic(err)
}

// Freeze infrastructure-sensitive settings after startup.
settings.Lock()
```

`Load` succeeds once. `Reload` rebuilds effective values from defaults and the
provided sources. `Apply` updates several settings from explicit strings as one
transaction. `Update` and `Setting.Set` update one setting.

Failed loads, reloads, and applications do not partially update settings,
bound fields, revisions, or notifications. A reload with no effective changes
does not increment the revision or emit an event.

## Sources and precedence

Sources are applied in argument order after registered defaults; later sources
override earlier sources. An omitted source value does not clear a default,
while an explicit empty string is still a value. Unknown source paths return an
error.

`EnvironmentSource` reads environment variables for the settings registered in
the receiving set. Setting paths are uppercased and non-alphanumeric separators
become underscores, so `HTTP.Server.Read-Timeout` maps to
`MYAPP_HTTP_SERVER_READ_TIMEOUT` with the `MYAPP` prefix:

```go
settings.Setting("HTTP.Server.Read-Timeout", 15*time.Second, "HTTP read timeout")

if err := settings.Load(ctx, fileSource, settings.EnvironmentSource("MYAPP")); err != nil {
	panic(err)
}
```

Prefixes are uppercased but otherwise preserved. They may contain ASCII letters,
digits, and underscores, and a non-empty prefix must start with a letter or
underscore. For example, `__M__` produces `__M___HTTP_SERVER_READ_TIMEOUT`. Use
an empty prefix to read names such as `HTTP_SERVER_READ_TIMEOUT`. Ambiguous path
mappings fail loading instead of choosing one setting.

Use a named `RuntimeSource` when overrides should survive later reloads:

```go
runtime := config.NewRuntimeSource("runtime")
_ = runtime.Set("HTTP.Port", "8080")
_ = settings.Load(ctx, fileSource, runtime)
_ = settings.Reload(ctx, fileSource, runtime)
```

## Paths

Paths are case-insensitive at input boundaries and have one canonical spelling,
similar to HTTP header canonicalization. Each dotted segment starts with an
uppercase character and otherwise uses lowercase characters, with hyphenated
segments capitalized after hyphens. For example, `http.port`, `HTTP.PORT`, and
`Http.Port` all resolve to `Http.Port`.

## Locking

`Lock` is irreversible and applies to the root configuration tree. It freezes
schema registration and prevents changes to settings registered with
`config.Lockable()`. Non-lockable settings may still be reloaded or updated.
There is no `Unlock`. Direct `Setting.Set` calls enforce the same lock rule as
`Apply` and `Reload`.

## Binding

Binding is an error-returning adapter for startup configuration structs:

```go
var application struct {
	HTTP struct {
		Port int `setting:"Port" description:"HTTP listener port"`
	}
}

if err := settings.Bind(&application); err != nil {
	panic(err)
}
```

The package owns bound fields after binding and may update them through setting
operations. Application code must not write bound fields directly. Direct
struct reads are suitable for startup/read-only use; use `Snapshot` or typed
setting retrieval for runtime consumers that need synchronized reads.

## Validation and snapshots

Use `WithValidator` for per-setting checks and `AddValidator` for rules over a
complete candidate configuration. Validation occurs before a transaction
commits and multiple failures are returned together.

`Snapshot` captures all settings at one revision. `Value[T]` performs strict
typed retrieval and returns an error for a type mismatch:

```go
snapshot, err := settings.Snapshot()
portValue, err := config.Value[int](port)
```

## Encoding and notifications

Built-in codecs support scalar values, named primitive types,
`time.Duration`, and standard `encoding.TextMarshaler`/
`encoding.TextUnmarshaler` implementations. Explicit codecs are preferred for
composite values. Change subscribers receive stable old/new string values and
the committed revision. Callbacks run after internal locks are released;
callback panics are not recovered.

## Stability

The package is undergoing a deliberate breaking redesign. The API is expected
to stabilize at 1.0.0.

## Examples

Examples are provided in the documentation, more will be added in the future.
