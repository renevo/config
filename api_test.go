package config_test

import (
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
