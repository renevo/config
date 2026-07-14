package config

import "testing"

func TestCanonicalPathLookup(t *testing.T) {
	set := NewSet()
	setting := set.Setting("HTTP.Port", 8080, "port")

	if setting.Path != "Http.Port" {
		t.Fatalf("canonical path = %q, want %q", setting.Path, "Http.Port")
	}
	for _, path := range []string{"http.port", "HTTP.PORT", "Http.Port"} {
		if got := set.Get(path); got != setting {
			t.Fatalf("Get(%q) returned %p, want %p", path, got, setting)
		}
	}
}
