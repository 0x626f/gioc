package gioc

import (
	"fmt"
	"strings"
)

const (
	circularModuleErrorRoot     = "module"
	circularDependencyErrorRoot = "dependency"
)

// CircularInjectionError reports a module or provider cycle.
type CircularInjectionError struct {
	root string

	// Tokens is the cycle path.
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

// Error returns the formatted cycle error.
func (err *CircularInjectionError) Error() string {
	return fmt.Sprintf("circular %s injection error: %s ", err.root, strings.Join(err.Tokens, " -> "))
}

// DependencyError reports an invalid or missing dependency.
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

func nilModule() *DependencyError {
	return &DependencyError{
		reason: "module is nil",
	}
}

func nilProvider() *DependencyError {
	return &DependencyError{
		reason: "provider is nil",
	}
}

func providerAlreadyAssigned(token Token, current Token, next Token) *DependencyError {
	return &DependencyError{
		reason: fmt.Sprintf("provider %s is already assigned to module %s and cannot be assigned to module %s", token, current, next),
	}
}

func moduleRegistrationClosed() *DependencyError {
	return &DependencyError{
		reason: "modules cannot be added after Run has been called",
	}
}

func containerNotReady() *DependencyError {
	return &DependencyError{
		reason: "container must Run successfully before Resolve can be called",
	}
}

func duplicateModuleToken(token Token) *DependencyError {
	return &DependencyError{
		reason: fmt.Sprintf("module token %s is registered more than once", token),
	}
}

func injectionScope(injections []*Injection) string {
	tokens := make([]string, 0, len(injections))
	for _, injection := range injections {
		if injection == nil {
			tokens = append(tokens, "<nil>")
			continue
		}
		tokens = append(tokens, injection.Token)
	}
	return fmt.Sprintf("[%s]", strings.Join(tokens, " "))
}

func missingInjection(token Token, injections []*Injection) *DependencyError {
	return &DependencyError{
		reason: fmt.Sprintf("injection %s is missing in the scope: %s", token, injectionScope(injections)),
	}
}

func missingFactoryConstructor(token Token) *DependencyError {
	return &DependencyError{
		reason: fmt.Sprintf("factory constructor is missing for %s", token),
	}
}

// Error returns the formatted dependency error.
func (err *DependencyError) Error() string {
	return fmt.Sprintf("dependency error: %s ", err.reason)
}
