package config

import (
	"errors"
	"testing"
)

func TestLockIsRootWideAndEnforcedBySettingSet(t *testing.T) {
	set := NewSet()
	child := set.Subset("HTTP")
	setting := child.Setting("Port", 8080, "port")
	setting.Lockable = true

	child.Lock()
	if !set.Locked() || !child.Locked() {
		t.Fatal("locking a child did not lock the root tree")
	}
	if err := setting.Set("9090"); !errors.Is(err, ErrLocked) {
		t.Fatalf("setting.Set error = %v, want ErrLocked", err)
	}
	if got := setting.String(); got != "8080" {
		t.Fatalf("locked setting changed to %q", got)
	}
}

func TestLockFreezesSchema(t *testing.T) {
	set := NewSet()
	set.Lock()

	if child := set.Subset("HTTP"); child != nil {
		t.Fatal("Subset succeeded after schema lock")
	}

	defer func() {
		value := recover()
		err, ok := value.(error)
		if !ok || !errors.Is(err, ErrSchemaLocked) {
			t.Fatal("Setting did not panic with ErrSchemaLocked")
		}
	}()
	set.Setting("Port", 8080, "port")
}
