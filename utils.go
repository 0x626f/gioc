package gioc

import "reflect"

// CreateToken returns the struct name of T as a Token.
func CreateToken[T any]() Token {
	t := reflect.TypeFor[T]()

	if t.Kind() == reflect.Ptr {
		if t.Elem().Kind() != reflect.Struct {
			panic(invalidTokenType(t.String()))
		}
		t = t.Elem()
	}

	if t.Kind() != reflect.Struct {
		panic(invalidTokenType(t.String()))
	}

	return t.Name()
}

func name[T any]() string {
	t := reflect.TypeFor[T]()
	return t.String()
}

func nameOf(value any) string {
	t := reflect.TypeOf(value)
	if t == nil {
		return "<nil>"
	}
	return t.String()
}

func zero[T any]() T {
	var z T
	return z
}
