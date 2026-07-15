package config_test

import (
	"context"
	"iter"
	"testing"

	"github.com/matryer/is"
	"github.com/renevo/config"
)

func TestNewSetCreatesIndependentRootSet(t *testing.T) {
	is := is.New(t)
	root := config.NewSet()
	is.True(root != nil)        // expected a new root set
	is.Equal(root.Path(), "")   // expected an empty root path
	is.Equal(root.Root(), root) // expected a new set to be its own root
}

func TestBindOnExplicitRootSetCreatesNestedSettings(t *testing.T) {
	is := is.New(t)
	root := config.NewSet()

	var cfg struct {
		Name string `setting:"name"`
		HTTP struct {
			Addr string `setting:"addr"`
		}
	}

	bindErr := root.Bind(&cfg)
	if bindErr != nil {
		t.Fatal(bindErr)
	}

	nameSetting := root.Get("name")
	is.True(nameSetting != nil) // expected a top-level setting to be created

	httpSetting := root.Get("HTTP.addr")
	is.True(httpSetting != nil) // expected a nested setting to be created
}

func TestSourceFuncAcceptsSettingMetadataSequence(t *testing.T) {
	settings := config.NewSet()
	settings.Setting("Port", 8080, "HTTP port")
	source := config.SourceFunc(func(_ context.Context, metadata iter.Seq[config.SettingMetadata]) ([]config.RawValue, error) {
		for setting := range metadata {
			if setting.Path != "Port" {
				t.Fatalf("metadata path = %q, want Port", setting.Path)
			}
		}
		return nil, nil
	})

	if err := settings.Load(context.Background(), source); err != nil {
		t.Fatal(err)
	}
}
