package config

import (
	"errors"
	"testing"
)

func TestSnapshotIsSortedAndCoherent(t *testing.T) {
	set := NewSet()
	set.Setting("Zed", 2, "zed")
	set.Setting("Alpha", "one", "alpha")
	if err := set.Apply(map[string]string{"zed": "3", "alpha": "two"}); err != nil {
		t.Fatal(err)
	}
	snapshot, err := set.Snapshot()
	if err != nil {
		t.Fatal(err)
	}
	if snapshot.Revision != 1 || len(snapshot.Values) != 2 {
		t.Fatalf("snapshot metadata = revision %d, values %d", snapshot.Revision, len(snapshot.Values))
	}
	if snapshot.Values[0].Path != "Alpha" || snapshot.Values[1].Path != "Zed" {
		t.Fatalf("snapshot paths are not sorted: %+v", snapshot.Values)
	}
	if snapshot.Values[0].Value != "two" || snapshot.Values[1].Value != "3" {
		t.Fatalf("snapshot values = %+v", snapshot.Values)
	}
}

func TestTypedValueRetrievalIsStrict(t *testing.T) {
	set := NewSet()
	setting := set.Setting("Port", 8080, "port")
	value, err := Value[int](setting)
	if err != nil || value != 8080 {
		t.Fatalf("Value[int] = %d, %v", value, err)
	}
	if _, err := Value[string](setting); !errors.Is(err, ErrUnsupported) {
		t.Fatalf("Value[string] error = %v, want ErrUnsupported", err)
	}
}

func TestRangeDoesNotMatchSharedPrefixes(t *testing.T) {
	set := NewSet()
	set.Setting("Http.Port", 1, "http")
	set.Setting("HttpServer.Port", 2, "server")
	child := set.Subset("Http")
	var paths []string
	child.Range(func(path string, _ *Setting) bool {
		paths = append(paths, path)
		return true
	})
	if len(paths) != 1 || paths[0] != "Http.Port" {
		t.Fatalf("child range = %v, want [Http.Port]", paths)
	}
}
