package config

import (
	"errors"
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

func TestBindRejectsSchemaChangesAfterLock(t *testing.T) {
	set := NewSet()
	set.Lock()
	var config struct{ Port int }
	if err := set.Bind(&config); !errors.Is(err, ErrSchemaLocked) {
		t.Fatalf("Bind error = %v, want ErrSchemaLocked", err)
	}
}
