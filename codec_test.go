package config

import (
	"encoding"
	"testing"
)

type namedInt int

type textValue string

var _ encoding.TextMarshaler = (*textValue)(nil)
var _ encoding.TextUnmarshaler = (*textValue)(nil)

func (value *textValue) MarshalText() ([]byte, error) { return []byte(*value), nil }
func (value *textValue) UnmarshalText(input []byte) error {
	*value = textValue(input)
	return nil
}

func TestCodecSupportsNamedPrimitives(t *testing.T) {
	value := namedInt(7)
	setting := &Setting{value: &value}

	if err := setting.Set("42"); err != nil {
		t.Fatal(err)
	}
	if value != 42 || setting.String() != "42" {
		t.Fatalf("named primitive = %d, string = %q", value, setting.String())
	}
}

func TestCodecSupportsTextInterfaces(t *testing.T) {
	value := textValue("before")
	setting := &Setting{value: &value}

	if setting.String() != "before" {
		t.Fatalf("String() = %q, want before", setting.String())
	}
	if err := setting.Set("after"); err != nil {
		t.Fatal(err)
	}
	if value != "after" {
		t.Fatalf("text value = %q, want after", value)
	}
}

func TestCodecParseFailurePreservesPointerValue(t *testing.T) {
	value := 7
	setting := &Setting{value: &value}

	err := setting.Set("not-an-int")
	if err == nil {
		t.Fatal("Set succeeded for invalid integer")
	}
	if value != 7 {
		t.Fatalf("value changed after parse failure: %d", value)
	}
}

type testCodec struct{}

func (testCodec) Parse(value string) (any, error)  { return value + "!", nil }
func (testCodec) Format(value any) (string, error) { return value.(string), nil }
func (testCodec) Equal(left, right any) bool       { return left == right }

func TestCustomCodecCanExtendSettingConversion(t *testing.T) {
	set := NewSet()
	setting := set.Setting("Name", "initial", "name", WithCodec(testCodec{}))
	if err := setting.Set("updated"); err != nil {
		t.Fatal(err)
	}
	if got := setting.String(); got != "updated!" {
		t.Fatalf("custom codec string = %q, want updated!", got)
	}
}
