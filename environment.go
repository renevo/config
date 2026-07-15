package config

import (
	"context"
	"fmt"
	"iter"
	"os"
	"sort"
	"strings"
)

type environmentSettingsSource struct{ prefix string }

// EnvironmentSource reads registered settings from environment variables.
// Prefix is normalized when the source loads; an invalid prefix fails loading.
func EnvironmentSource(prefix string) Source {
	return environmentSettingsSource{prefix: prefix}
}

type environmentSetting struct {
	name string
	path string
}

func (source environmentSettingsSource) Load(ctx context.Context, metadata iter.Seq[SettingMetadata]) ([]RawValue, error) {
	prefix, err := normalizeEnvironmentPrefix(source.prefix)
	if err != nil {
		return nil, err
	}
	settings := make([]environmentSetting, 0)
	for setting := range metadata {
		settings = append(settings, environmentSetting{
			name: environmentName(prefix, setting.Path),
			path: setting.Path,
		})
	}
	sort.Slice(settings, func(i, j int) bool {
		if settings[i].name == settings[j].name {
			return settings[i].path < settings[j].path
		}
		return settings[i].name < settings[j].name
	})

	values := make([]RawValue, 0, len(settings))
	for index, setting := range settings {
		if index > 0 && settings[index-1].name == setting.name {
			return nil, fmt.Errorf(
				"environment variable %q maps to both settings %q and %q",
				setting.name,
				settings[index-1].path,
				setting.path,
			)
		}
		value, ok := os.LookupEnv(setting.name)
		if !ok {
			continue
		}
		values = append(values, RawValue{
			Path:   setting.path,
			Value:  value,
			Source: setting.name,
		})
	}
	return values, nil
}

func environmentName(prefix, path string) string {
	name := strings.Map(func(character rune) rune {
		switch {
		case character >= 'a' && character <= 'z':
			return character - ('a' - 'A')
		case character >= 'A' && character <= 'Z', character >= '0' && character <= '9':
			return character
		default:
			return '_'
		}
	}, path)
	if prefix == "" {
		return name
	}
	return prefix + "_" + name
}

func normalizeEnvironmentPrefix(prefix string) (string, error) {
	prefix = strings.ToUpper(prefix)
	for index, character := range prefix {
		if (character >= 'A' && character <= 'Z') || character == '_' || (index > 0 && character >= '0' && character <= '9') {
			continue
		}
		return "", fmt.Errorf("environment prefix %q must contain only ASCII letters, digits, and underscores and start with a letter or underscore", prefix)
	}
	return prefix, nil
}
