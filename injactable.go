package gioc

// Injection stores a resolved instance with its token.
type Injection struct {
	Token    Token
	Instance any
}

// Injections is the dependency set passed to a factory constructor.
type Injections []*Injection

// Inject returns dependency tokens for a factory.
func Inject(injections ...Token) []Token {
	return injections
}

// Resolve returns token from injections as T.
func Resolve[T any](token Token, injections Injections) (T, error) {
	return resolveFrom[T](token, injections)
}

// MustResolve returns token from injections as T or panics.
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

// Resolve returns token from injections as any.
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

// MustResolve returns token from injections as any or panics.
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

// Require panics if any token is missing from injections.
func Require(injections Injections, tokens ...Token) {
	injections.Require(tokens...)
}

// Require panics if any token is missing from injections.
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
