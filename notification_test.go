package config

import (
	"sync/atomic"
	"testing"
)

func TestSettingNotificationCanRecurse(t *testing.T) {
	set := NewSet()
	setting := set.Setting("Value", 1, "value")
	var calls atomic.Int32
	setting.Notify(NotifyFunc(func(value *Setting) {
		if calls.Add(1) == 1 {
			if err := value.Set("2"); err != nil {
				t.Error(err)
			}
		}
	}))

	if err := setting.Set("3"); err != nil {
		t.Fatal(err)
	}
	if setting.String() != "2" {
		t.Fatalf("recursive notification value = %q, want 2", setting.String())
	}
	if calls.Load() != 2 {
		t.Fatalf("notification calls = %d, want 2", calls.Load())
	}
}

func TestChangeNotificationPublishesStableRecordAndCloseIsIdempotent(t *testing.T) {
	set := NewSet()
	setting := set.Setting("Value", 1, "value")
	var changes []Change
	handle := setting.NotifyChange(ChangeFunc(func(change Change) {
		changes = append(changes, change)
	}))

	if err := setting.Set("2"); err != nil {
		t.Fatal(err)
	}
	if len(changes) != 1 {
		t.Fatalf("change count = %d, want 1", len(changes))
	}
	change := changes[0]
	if change.Path != "Value" || change.Old != "1" || change.New != "2" || change.Revision != 1 {
		t.Fatalf("unexpected change: %+v", change)
	}
	if err := handle.Close(); err != nil {
		t.Fatal(err)
	}
	if err := handle.Close(); err != nil {
		t.Fatal(err)
	}
	if err := setting.Set("3"); err != nil {
		t.Fatal(err)
	}
	if len(changes) != 1 {
		t.Fatalf("change count after close = %d, want 1", len(changes))
	}
}
