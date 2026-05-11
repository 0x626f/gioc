package gioc

import "reflect"

// CreateToken derives a Token string from the type parameter T.
// For a pointer type (*Foo) the name of the pointed-to struct is used ("Foo").
// For a bare struct type (Foo) the struct name is used directly.
// Panics if T is not a struct or pointer to a struct.
//
//	CreateToken[*MyService]() // returns "MyService"
//	CreateToken[MyService]()  // returns "MyService"
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
