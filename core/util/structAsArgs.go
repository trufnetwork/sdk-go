package util

import (
	"github.com/pkg/errors"
	"reflect"
)

// addArgOrNull adds a new argument to the list of arguments
// this helps us making it NULL if it's equal to its zero value
// The caveat is that we won't be able to pass the zero value of the type. Issues with this?
func addArgOrNull(oldArgs []any, newArg any, nullIfZero bool) []any {
	if nullIfZero && reflect.ValueOf(newArg).IsZero() {
		return append(oldArgs, nil)
	}

	return append(oldArgs, newArg)
}

// StructAsArgs converts a struct to a list of arguments from struct fields, in the same order
// as they are defined in the struct.
// it also checks if the fields was required by tag
func StructAsArgs(s interface{}) ([]any, error) {
	v := reflect.ValueOf(s)
	t := v.Type()

	var args []any
	for i := 0; i < t.NumField(); i++ {
		field := t.Field(i)
		value := v.Field(i).Interface()

		// check if the field is required
		isRequired := field.Tag.Get("validate") == "required"
		if isRequired && reflect.ValueOf(value).IsZero() {
			return nil, errors.Errorf("required field '%s' is empty", field.Name)
		}

		// check if the field is an accepted type
		if !isAcceptedType(value) {
			return nil, errors.Errorf("unsupported field type '%s' for field '%s'", reflect.TypeOf(value).Name(), field.Name)
		}

		// if it's not required, then we can add it as NULL if it's zero-like
		args = addArgOrNull(args, value, !isRequired)
	}

	return args, nil
}

// isAcceptedType checks if the given value is of an accepted type
func isAcceptedType(v interface{}) bool {
	t := reflect.TypeOf(v)

	switch t.Kind() {
	case reflect.String, reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64,
		reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64,
		reflect.Bool, reflect.Float32, reflect.Float64:
		return true
	case reflect.Slice, reflect.Array:
		// Check if the slice/array element type is an accepted type
		return isAcceptedType(reflect.Zero(t.Elem()).Interface())
	default:
		return false
	}
}
