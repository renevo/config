package config

import (
	"context"
	"errors"
	"testing"
)

func TestLoadReloadLifecycle(t *testing.T) {
	set := NewSet()
	set.Setting("Port", 8080, "port")

	if set.Loaded() || set.Revision() != 0 {
		t.Fatal("new set has unexpected lifecycle state")
	}
	if err := set.Load(context.Background(), ValuesSource("file", map[string]string{"port": "9090"})); err != nil {
		t.Fatal(err)
	}
	if !set.Loaded() || set.Revision() != 1 || set.Get("PORT").String() != "9090" {
		t.Fatalf("load state/value incorrect: loaded=%v revision=%d value=%s", set.Loaded(), set.Revision(), set.Get("port").String())
	}
	if err := set.Load(context.Background()); !errors.Is(err, ErrAlreadyLoaded) {
		t.Fatalf("second Load error = %v, want ErrAlreadyLoaded", err)
	}
	if err := set.Reload(context.Background(), ValuesSource("file", map[string]string{"port": "9090"})); err != nil {
		t.Fatal(err)
	}
	if set.Revision() != 1 {
		t.Fatalf("no-op reload revision = %d, want 1", set.Revision())
	}
	if err := set.Reload(context.Background(), ValuesSource("file", map[string]string{"port": "7070"})); err != nil {
		t.Fatal(err)
	}
	if set.Revision() != 2 || set.Get("port").String() != "7070" {
		t.Fatal("reload did not commit source value")
	}
}

func TestRuntimeSourceSurvivesReload(t *testing.T) {
	set := NewSet()
	set.Setting("Port", 8080, "port")
	runtime := NewRuntimeSource("runtime")
	if err := runtime.Set("PORT", "7777"); err != nil {
		t.Fatal(err)
	}
	file := ValuesSource("file", map[string]string{"port": "9090"})
	if err := set.Load(context.Background(), file, runtime); err != nil {
		t.Fatal(err)
	}
	if got := set.Get("port").String(); got != "7777" {
		t.Fatalf("loaded runtime override = %q, want 7777", got)
	}
	if err := set.Reload(context.Background(), file, runtime); err != nil {
		t.Fatal(err)
	}
	if got := set.Get("port").String(); got != "7777" {
		t.Fatalf("reloaded runtime override = %q, want 7777", got)
	}
}

func TestLoadFailureLeavesSetUnloaded(t *testing.T) {
	set := NewSet()
	set.Setting("Port", 8080, "port")
	if err := set.Load(context.Background(), ValuesSource("file", map[string]string{"missing": "1"})); err == nil {
		t.Fatal("Load succeeded with unknown source key")
	}
	if set.Loaded() || set.Revision() != 0 || set.Get("port").String() != "8080" {
		t.Fatal("failed Load changed lifecycle or values")
	}
}

func TestReloadRejectsChangedLockedValue(t *testing.T) {
	set := NewSet()
	setting := set.Setting("Port", 8080, "port", Lockable())
	if err := set.Load(context.Background()); err != nil {
		t.Fatal(err)
	}
	set.Lock()
	if err := set.Reload(context.Background(), ValuesSource("file", map[string]string{"port": "9090"})); !errors.Is(err, ErrLocked) {
		t.Fatalf("reload error = %v, want ErrLocked", err)
	}
	if setting.String() != "8080" || set.Revision() != 1 {
		t.Fatal("failed locked reload changed state")
	}
}
