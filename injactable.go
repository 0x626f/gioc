package gioc

// Injection is the runtime wrapper produced by a provider after construction.
// It pairs the resolved instance with its Token so the container can match it
// against injection requests by name.
type Injection struct {
	Token    Token
	Instance any
}

// Injections is the resolved dependency set passed to a factory constructor.
// It replaces the older variadic []*Injection constructor argument while
// preserving slice semantics for iteration and helper use.
type Injections []*Injection

// Inject declares the ordered list of dependency tokens that a factory
// constructor expects to receive as its Injections argument. The tokens
// must correspond to providers registered within the same module or any of its
// directly imported modules.
//
//	FactoryProvider[*Service]("", Factory[*Service]{
//	    Injects: Inject("Logger", "Database"),
//	    Constructor: func(deps Injections) (*Service, error) { ... },
//	}, false)
func Inject(injections ...Token) []Token {
	return injections
}

// Resolve finds the dependency with the given token in the injection set and
// type-asserts its instance to T. It returns a [DependencyError] if the
// dependency is missing or cannot be cast to T.
//
//	db, err := Resolve[*Database]("Database", deps)
func Resolve[T any](token Token, injections Injections) (T, error) {
	return resolveFrom[T](token, injections)
}

// MustResolve finds and type-asserts the dependency with the given token in the
// injection set. It panics with a [DependencyError] if the dependency is missing
// or cannot be cast to T.
func MustResolve[T any](token Token, injections Injections) T {
	instance, err := Resolve[T](token, injections)
	if err != nil {
		panic(err)
	}
	return instance
}

func resolveInjection[T any](injection *Injection) (T, error) {
	if injection == nil {
		return zero[T](), nilInjection()
	}

	return resolveInstance[T](injection.Token, injection.Instance)
}

func resolveInstance[T any](token Token, instance any) (T, error) {
	val, ok := instance.(T)
	if !ok {
		return zero[T](), invalidInjectionType[T](token, instance)
	}
	return val, nil
}

// Resolve finds the dependency with the given token in the injection set and
// returns its ready-to-use instance.
func (injections Injections) Resolve(token Token) (any, error) {
	for _, injection := range injections {
		if injection == nil {
			return nil, nilInjection()
		}

		if token == injection.Token {
			return injection.Instance, nil
		}
	}
	return nil, missingInjection(token, injections)
}

// MustResolve finds the dependency with the given token in the injection set
// and returns its ready-to-use instance. It panics with a [DependencyError] if
// the dependency is missing.
func (injections Injections) MustResolve(token Token) any {
	instance, err := injections.Resolve(token)
	if err != nil {
		panic(err)
	}
	return instance
}

func resolveFrom[T any](token Token, injections Injections) (T, error) {
	instance, err := injections.Resolve(token)
	if err != nil {
		return zero[T](), err
	}
	return resolveInstance[T](token, instance)
}

// Require panics with a [DependencyError] if any of the required tokens is
// absent from the injection set. Use it at the top of a factory constructor
// to assert that all expected dependencies were provided before accessing them.
//
//	Require(deps, "Logger", "Database")
func Require(injections Injections, tokens ...Token) {
	injections.Require(tokens...)
}

// Require panics with a [DependencyError] if any of the required tokens is
// absent from the injection set.
func (injections Injections) Require(tokens ...Token) {
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
