package config

import (
	"context"
	"strings"
	"testing"

	"github.com/matryer/is"
)

func TestEnvironmentSettingsSource(t *testing.T) {
	tests := []struct {
		name   string
		prefix string
		path   string
		env    string
		value  string
	}{
		{name: "unprefixed", path: "http.address", env: "HTTP_ADDRESS", value: "127.0.0.1:8080"},
		{name: "prefixed", prefix: "MYAPP", path: "http.address", env: "MYAPP_HTTP_ADDRESS", value: "127.0.0.1:9090"},
		{name: "underscore prefix", prefix: "__M__", path: "http.address", env: "__M___HTTP_ADDRESS", value: "127.0.0.1:9090"},
		{name: "nested separators", prefix: "MYAPP", path: "http.server.read-timeout", env: "MYAPP_HTTP_SERVER_READ_TIMEOUT", value: "15s"},
		{name: "explicit empty", path: "http.address", env: "HTTP_ADDRESS", value: ""},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			is := is.New(t)
			var target string
			settings := NewSet()
			settings.Setting(test.path, &target, "test setting")
			t.Setenv(test.env, test.value)

			source := settings.EnvironmentSource(test.prefix)
			values, err := source.Load(context.Background())
			is.NoErr(err)            // registered settings should map to valid environment names
			is.Equal(len(values), 1) // the matching environment variable should produce one raw value
			is.Equal(values[0].Path, settingsPath(settings))
			is.Equal(values[0].Value, test.value) // source should preserve text, including an explicit empty string
			is.Equal(values[0].Source, test.env)  // diagnostics should identify the concrete environment variable
			is.NoErr(settings.Load(context.Background(), source))
			is.Equal(target, test.value) // the existing setting codec should commit the environment text unchanged
		})
	}
}

func TestEnvironmentSettingsSourceCollision(t *testing.T) {
	is := is.New(t)
	settings := NewSet()
	var first, second string
	settings.Setting("http.read_timeout", &first, "first setting")
	settings.Setting("http.read.timeout", &second, "second setting")

	values, err := (environmentSettingsSource{settings: settings}).Load(context.Background())
	is.Equal(values, []RawValue(nil)) // ambiguous names should not produce partial source values
	is.True(err != nil)               // ambiguous normalized names should fail source loading
	is.True(strings.Contains(err.Error(), "HTTP_READ_TIMEOUT"))
	is.True(strings.Contains(err.Error(), "Http.Read_timeout"))
	is.True(strings.Contains(err.Error(), "Http.Read.Timeout"))
}

func settingsPath(settings *Set) string {
	var path string
	settings.Range(func(settingPath string, _ *Setting) bool {
		path = settingPath
		return false
	})
	return path
}
