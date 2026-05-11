package gioc

import (
	"fmt"
	"strings"
)

const (
	circularModuleErrorRoot     = "module"
	circularDependencyErrorRoot = "dependency"
)

// CircularInjectionError is returned by Container.Run when the container
// detects a cycle in either the module import graph or the provider dependency
// graph. The Tokens field contains the full cycle path in traversal order.
type CircularInjectionError struct {
	root string

	// Tokens contains the full cycle path in traversal order.
	Tokens []Token
}

func circularModuleInjection(sequence ...Token) *CircularInjectionError {
	return &CircularInjectionError{
		root:   circularModuleErrorRoot,
		Tokens: sequence,
	}
}

func circularDependencyInjection(sequence ...Token) *CircularInjectionError {
	return &CircularInjectionError{
		root:   circularDependencyErrorRoot,
		Tokens: sequence,
	}
}

// Error implements the error interface.
// The message includes the cycle kind ("module" or "dependency") and the full
// token path that forms the cycle, e.g.:
//
//	circular dependency injection error: A -> B -> A
func (err *CircularInjectionError) Error() string {
	return fmt.Sprintf("circular %s injection error: %s ", err.root, strings.Join(err.Tokens, " -> "))
}

// DependencyError is returned when the container cannot satisfy a declared
// dependency — either because the required token is not registered in the
// visible scope, or because an Injectable cannot be cast to the expected type.
type DependencyError struct {
	reason string
}

func invalidTokenType(provided string) *DependencyError {
	return &DependencyError{
		reason: fmt.Sprintf("token can't be derived from type: %s", provided),
	}
}

func missingDependency(provider IProvider, dependency Token) *DependencyError {
	return &DependencyError{
		reason: fmt.Sprintf(
			"%s is not observed on module %s for %s. validate injections: %s",
			dependency, provider.AssignedTo().Token(), provider.Token(), strings.ReplaceAll(strings.Join(provider.Injections(), ","), dependency, "?"),
		),
	}
}

func invalidInjectionType[T any](token Token, value any) *DependencyError {
	return &DependencyError{
		reason: fmt.Sprintf("couldn't cast %s to type %s as it has type %s", token, name[T](), nameOf(value)),
	}
}

func nilInjection() *DependencyError {
	return &DependencyError{
		reason: "injection is nil",
	}
}

func missingInjection(token Token, injections []*Injectable) *DependencyError {
	return &DependencyError{
		reason: fmt.Sprintf("injection %s is missing in the scope: %v", token, injections),
	}
}

func missingFactoryConstructor(token Token) *DependencyError {
	return &DependencyError{
		reason: fmt.Sprintf("factory constructor is missing for %s", token),
	}
}

// Error implements the error interface.
func (err *DependencyError) Error() string {
	return fmt.Sprintf("dependency error: %s ", err.reason)
}
