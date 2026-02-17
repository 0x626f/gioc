package gioc

import "reflect"

// CreateToken derives a Token string from the type parameter T.
// For a pointer type (*Foo) the name of the pointed-to struct is used ("Foo").
// For a bare struct type (Foo) the struct name is used directly.
// Panics if T is a pointer to a non-struct type (e.g. *string).
//
//	CreateToken[*MyService]() // returns "MyService"
//	CreateToken[MyService]()  // returns "MyService"
func CreateToken[T any]() Token {
	var sample T
	t := reflect.TypeOf(sample)

	if t.Kind() == reflect.Ptr {
		if t.Elem().Kind() != reflect.Struct {
			panic(invalidTokenType(t.Name()))
		}
		t = t.Elem()
	}

	return t.Name()
}

func name[T any]() string {
	var sample T
	t := reflect.TypeOf(sample)
	return t.String()
}

func nameOf(value any) string {
	t := reflect.TypeOf(value)
	return t.String()
}

func zero[T any]() T {
	var z T
	return z
}
