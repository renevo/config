package config

import (
	"errors"
	"strings"
	"testing"
)

func TestApplyIsAtomicAndAggregatesErrors(t *testing.T) {
	set := NewSet()
	port := set.Setting("HTTP.Port", 8080, "port")
	port.Lockable = true
	set.Setting("Workers", 2, "workers", WithValidator(func(value any) error {
		if value.(int) < 1 {
			return errors.New("workers must be positive")
		}
		return nil
	}))
	set.Lock()

	err := set.Apply(map[string]string{
		"HTTP.Port": "9090",
		"Workers":   "0",
		"Missing":   "true",
	})
	if err == nil {
		t.Fatal("Apply succeeded with invalid, unknown, and lockable updates")
	}
	for _, text := range []string{"Http.Port", "Workers", "Missing", "workers must be positive"} {
		if !strings.Contains(err.Error(), text) {
			t.Fatalf("Apply error %q does not contain %q", err, text)
		}
	}
	if port.String() != "8080" || set.Get("Workers").String() != "2" {
		t.Fatal("failed Apply partially changed configuration")
	}
	if set.Revision() != 0 {
		t.Fatalf("revision = %d after failed Apply, want 0", set.Revision())
	}
}

func TestApplyCommitsOneRevision(t *testing.T) {
	set := NewSet()
	set.Setting("First", 1, "first")
	set.Setting("Second", 2, "second")

	if err := set.Apply(map[string]string{"first": "3", "SECOND": "4"}); err != nil {
		t.Fatal(err)
	}
	if set.Revision() != 1 {
		t.Fatalf("revision = %d, want 1", set.Revision())
	}
	if set.Get("first").String() != "3" || set.Get("second").String() != "4" {
		t.Fatal("Apply did not commit all values")
	}
}

func TestSetValidatorSeesCandidate(t *testing.T) {
	set := NewSet()
	set.Setting("Min", 1, "minimum")
	set.Setting("Max", 10, "maximum")
	if err := set.AddValidator(func(candidate Candidate) error {
		minimum, _ := candidate.Get("min")
		maximum, _ := candidate.Get("MAX")
		if minimum.(int) > maximum.(int) {
			return errors.New("minimum exceeds maximum")
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}

	if err := set.Apply(map[string]string{"min": "20"}); err == nil {
		t.Fatal("Apply succeeded despite cross-setting validation failure")
	}
	if set.Get("min").String() != "1" {
		t.Fatal("cross-setting validation partially changed configuration")
	}
}
