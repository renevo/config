package config

import (
	"encoding"
	"fmt"
	"reflect"
	"strconv"
	"time"
)

// Codec converts a setting value to and from its text representation.
//
// Implementations can add support for application-specific setting types.
type Codec interface {
	// Parse converts text into the setting's value type.
	Parse(string) (any, error)
	// Format converts a setting value to text.
	Format(any) (string, error)
	// Equal reports whether two values represent the same setting value.
	Equal(any, any) bool
}

type valueCodec struct {
	typeOf reflect.Type
}

func codecFor(value any) Codec {
	return valueCodec{typeOf: reflect.TypeOf(value)}
}

func (c valueCodec) Parse(text string) (any, error) {
	if c.typeOf == nil {
		return nil, ErrUnsupported
	}
	return parseReflectValue(c.typeOf, text)
}

func (c valueCodec) Format(value any) (string, error) {
	if value == nil {
		return "", ErrUnsupported
	}
	return formatReflectValue(reflect.ValueOf(value))
}

func (c valueCodec) Equal(left, right any) bool {
	return reflect.DeepEqual(left, right)
}

func parseReflectValue(typeOf reflect.Type, text string) (any, error) {
	if typeOf.Kind() == reflect.Pointer {
		value, err := parseReflectValue(typeOf.Elem(), text)
		if err != nil {
			return nil, err
		}
		result := reflect.New(typeOf.Elem())
		result.Elem().Set(reflect.ValueOf(value))
		return result.Interface(), nil
	}

	if typeOf == reflect.TypeFor[time.Duration]() {
		value, err := time.ParseDuration(text)
		if err != nil {
			return nil, fmt.Errorf("unable to parse %s: %w", typeOf, err)
		}
		return value, nil
	}

	value := reflect.New(typeOf)
	if unmarshaler, ok := value.Interface().(encoding.TextUnmarshaler); ok {
		if err := unmarshaler.UnmarshalText([]byte(text)); err != nil {
			return nil, fmt.Errorf("unable to parse %s: %w", typeOf, err)
		}
		return value.Elem().Interface(), nil
	}

	var parsed any
	var err error
	switch typeOf.Kind() {
	case reflect.String:
		parsed = text
	case reflect.Bool:
		parsed, err = strconv.ParseBool(text)
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		var number int64
		number, err = strconv.ParseInt(text, 0, typeOf.Bits())
		parsed = number
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr:
		var number uint64
		number, err = strconv.ParseUint(text, 0, typeOf.Bits())
		parsed = number
	case reflect.Float32, reflect.Float64:
		var number float64
		number, err = strconv.ParseFloat(text, typeOf.Bits())
		parsed = number
	default:
		return nil, fmt.Errorf("%w: %s", ErrUnsupported, typeOf)
	}
	if err != nil {
		return nil, fmt.Errorf("unable to parse %s: %w", typeOf, err)
	}

	result := reflect.New(typeOf).Elem()
	parsedValue := reflect.ValueOf(parsed)
	if !parsedValue.Type().ConvertibleTo(typeOf) {
		return nil, fmt.Errorf("%w: %s", ErrUnsupported, typeOf)
	}
	result.Set(parsedValue.Convert(typeOf))
	return result.Interface(), nil
}

func formatReflectValue(value reflect.Value) (string, error) {
	if !value.IsValid() {
		return "", ErrUnsupported
	}
	if value.Kind() == reflect.Pointer {
		if value.IsNil() {
			return "", nil
		}
		return formatReflectValue(value.Elem())
	}
	if value.CanAddr() {
		if marshaler, ok := value.Addr().Interface().(encoding.TextMarshaler); ok {
			encoded, err := marshaler.MarshalText()
			return string(encoded), err
		}
	} else {
		addressable := reflect.New(value.Type())
		addressable.Elem().Set(value)
		if marshaler, ok := addressable.Interface().(encoding.TextMarshaler); ok {
			encoded, err := marshaler.MarshalText()
			return string(encoded), err
		}
	}
	if value.CanInterface() {
		if marshaler, ok := value.Interface().(encoding.TextMarshaler); ok {
			encoded, err := marshaler.MarshalText()
			return string(encoded), err
		}
	}
	if value.Type() == reflect.TypeFor[time.Duration]() {
		return value.Interface().(time.Duration).String(), nil
	}

	switch value.Kind() {
	case reflect.String:
		return value.String(), nil
	case reflect.Bool:
		return strconv.FormatBool(value.Bool()), nil
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return strconv.FormatInt(value.Int(), 10), nil
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr:
		return strconv.FormatUint(value.Uint(), 10), nil
	case reflect.Float32, reflect.Float64:
		return strconv.FormatFloat(value.Float(), 'g', -1, value.Type().Bits()), nil
	default:
		return "", fmt.Errorf("%w: %s", ErrUnsupported, value.Type())
	}
}

func setParsedValue(current, parsed any) (any, error) {
	currentValue := reflect.ValueOf(current)
	parsedValue := reflect.ValueOf(parsed)
	if !currentValue.IsValid() || !parsedValue.IsValid() {
		return nil, ErrUnsupported
	}
	if currentValue.Kind() == reflect.Pointer {
		if currentValue.IsNil() || parsedValue.Kind() != reflect.Pointer {
			return nil, ErrUnsupported
		}
		if !parsedValue.Type().AssignableTo(currentValue.Type()) {
			return nil, ErrUnsupported
		}
		currentValue.Elem().Set(parsedValue.Elem())
		return current, nil
	}
	if !parsedValue.Type().AssignableTo(currentValue.Type()) {
		return nil, ErrUnsupported
	}
	return parsed, nil
}
