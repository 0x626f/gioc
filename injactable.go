package gioc

// Injectable is the runtime wrapper produced by a provider after construction.
// It pairs the resolved instance with its Token so the container can match it
// against injection requests by name.
type Injectable struct {
	Token    Token
	Instance any
}

// Inject declares the ordered list of dependency tokens that a factory
// constructor expects to receive as its []*Injectable arguments. The tokens
// must correspond to providers registered within the same module or any of its
// directly imported modules.
//
//	FactoryProvider[*Service]("", Factory[*Service]{
//	    Injects: Inject("Logger", "Database"),
//	    Constructor: func(deps ...*Injectable) (*Service, error) { ... },
//	}, false)
func Inject(injections ...Token) []Token {
	return injections
}

// Resolve extracts and type-asserts the instance stored in a single Injectable.
// It returns a [DependencyError] if the instance cannot be cast to T.
//
//	svc, err := Resolve[*MyService](inj)
func Resolve[T any](injection *Injectable) (T, error) {
	if injection == nil {
		return zero[T](), nilInjection()
	}

	val, ok := injection.Instance.(T)
	if !ok {
		return zero[T](), invalidInjectionType[T](injection.Token, injection.Instance)
	}
	return val, nil
}

// ResolveFrom finds the Injectable with the given token in a slice and
// type-asserts its instance to T. It returns a [DependencyError] when either
// no Injectable with that token exists or the instance cannot be cast to T.
//
// This is the standard helper used inside factory constructors:
//
//	db, err := ResolveFrom[*Database]("Database", deps)
func ResolveFrom[T any](token Token, injections []*Injectable) (T, error) {
	for _, injection := range injections {
		if injection == nil {
			return zero[T](), nilInjection()
		}

		if token == injection.Token {
			val, ok := injection.Instance.(T)
			if !ok {
				return zero[T](), invalidInjectionType[T](injection.Token, injection.Instance)
			}
			return val, nil
		}
	}
	return zero[T](), missingInjection(token, injections)
}

// Require panics with a [DependencyError] if any of the required tokens is
// absent from the injections slice. Use it at the top of a factory constructor
// to assert that all expected dependencies were provided before accessing them.
//
//	Require(deps, "Logger", "Database")
func Require(injections []*Injectable, tokens ...Token) {
	summary := make(map[string]struct{}, len(injections))
	for _, injection := range injections {
		if injection == nil {
			panic(nilInjection())
		}
		summary[injection.Token] = struct{}{}
	}

	for _, token := range tokens {
		if _, ok := summary[token]; !ok {
			panic(missingInjection(token, injections))
		}
	}
}
