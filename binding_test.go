package config

import (
	"errors"
	"flag"
	"testing"
)

func TestBindReturnsErrorsForInvalidTargets(t *testing.T) {
	set := NewSet()
	if err := set.Bind(nil); err == nil {
		t.Fatal("Bind(nil) succeeded")
	}
	if err := set.Bind(42); err == nil {
		t.Fatal("Bind(non-pointer) succeeded")
	}
	var value int
	if err := set.Bind(&value); err == nil {
		t.Fatal("Bind(pointer-to-scalar) succeeded")
	}
}

func TestBindPointerScalarAndExplicitFlagSet(t *testing.T) {
	set := NewSet()
	flagSet := flag.NewFlagSet("test", flag.ContinueOnError)
	port := 8080
	config := struct {
		Port *int `setting:"HTTP.Port" flag:"port"`
	}{}
	config.Port = &port

	if err := set.Bind(&config, WithFlagSet(flagSet)); err != nil {
		t.Fatal(err)
	}
	if err := flagSet.Parse([]string{"-port", "9090"}); err != nil {
		t.Fatal(err)
	}
	if *config.Port != 9090 {
		t.Fatalf("bound pointer scalar = %d, want 9090", *config.Port)
	}
	if set.Get("http.port") == nil {
		t.Fatal("pointer scalar setting was not registered")
	}
}

func TestBindRejectsSchemaChangesAfterLock(t *testing.T) {
	set := NewSet()
	set.Lock()
	var config struct{ Port int }
	if err := set.Bind(&config); !errors.Is(err, ErrSchemaLocked) {
		t.Fatalf("Bind error = %v, want ErrSchemaLocked", err)
	}
}
