package config

import (
	"context"
	"iter"
	"reflect"
	"testing"
)

func TestSourceReceivesSettingMetadata(t *testing.T) {
	set := NewSet()
	setting := set.Subset("HTTP").Setting("Port", 8080, "HTTP listener port", Lockable())
	setting.Mask = true

	var firstPass, secondPass, nextSource []SettingMetadata
	first := SourceFunc(func(_ context.Context, metadata iter.Seq[SettingMetadata]) ([]RawValue, error) {
		for setting := range metadata {
			firstPass = append(firstPass, setting)
			setting.Path = "Changed"
		}
		for setting := range metadata {
			secondPass = append(secondPass, setting)
		}
		return nil, nil
	})
	second := SourceFunc(func(_ context.Context, metadata iter.Seq[SettingMetadata]) ([]RawValue, error) {
		for setting := range metadata {
			nextSource = append(nextSource, setting)
		}
		return nil, nil
	})

	if err := set.Load(context.Background(), first, second); err != nil {
		t.Fatal(err)
	}
	if len(firstPass) != 1 || len(secondPass) != 1 || len(nextSource) != 1 {
		t.Fatalf("metadata lengths = %d, %d, %d; want 1, 1, 1", len(firstPass), len(secondPass), len(nextSource))
	}
	want := SettingMetadata{
		Path:        "Http.Port",
		Name:        "Port",
		Description: "HTTP listener port",
		Type:        reflect.TypeOf(8080),
		Masked:      true,
		Lockable:    true,
	}
	if !reflect.DeepEqual(firstPass[0], want) {
		t.Fatalf("metadata = %#v, want %#v", firstPass[0], want)
	}
	if !reflect.DeepEqual(secondPass[0], want) || !reflect.DeepEqual(nextSource[0], want) {
		t.Fatal("mutating yielded metadata affected a later iteration or source")
	}
	if setting.Path != want.Path {
		t.Fatalf("registered setting path = %q, want %q", setting.Path, want.Path)
	}
}
