package gioc

import "sync"

// IProvider describes a value the container can create.
type IProvider interface {
	// Token returns the provider token.
	Token() Token

	// Injections returns dependency tokens required by Create.
	Injections() []Token

	// Create builds the provider value from resolved dependencies.
	Create(Injections) (*Injection, error)

	// Exportable reports whether importers can use this provider.
	Exportable() bool

	// Scope returns the provider lifecycle.
	Scope() Scope

	// AssignOn binds the provider to a module.
	AssignOn(module *Module)

	// AssignedTo returns the owning module.
	AssignedTo() *Module
}

// ValueProviderInjection provides an existing singleton value.
type ValueProviderInjection[T any] struct {
	Key       Token
	Export    bool
	Value     T
	module    *Module
	tokenOnce sync.Once
}

// ValueProvider returns a singleton provider for value.
func ValueProvider[T any](token Token, value T, exportable bool) *ValueProviderInjection[T] {
	return &ValueProviderInjection[T]{
		Key:    token,
		Value:  value,
		Export: exportable,
	}
}

// Token returns the configured or derived provider token.
func (provider *ValueProviderInjection[T]) Token() Token {
	provider.tokenOnce.Do(func() {
		if provider.Key == "" {
			provider.Key = CreateToken[T]()
		}
	})
	return provider.Key
}

// Injections returns nil for value providers.
func (provider *ValueProviderInjection[T]) Injections() []Token {
	return []Token{}
}

// Create returns the stored value as an Injection.
func (provider *ValueProviderInjection[T]) Create(Injections) (*Injection, error) {
	return &Injection{
		Token:    provider.Token(),
		Instance: provider.Value,
	}, nil
}

// Exportable reports whether importers can use this provider.
func (provider *ValueProviderInjection[T]) Exportable() bool {
	return provider.Export
}

// Scope returns Singleton for value providers.
func (provider *ValueProviderInjection[T]) Scope() Scope {
	return Singleton
}

// AssignOn binds the provider to module.
func (provider *ValueProviderInjection[T]) AssignOn(module *Module) {
	provider.module = module
}

// AssignedTo returns the owning module.
func (provider *ValueProviderInjection[T]) AssignedTo() *Module {
	return provider.module
}

// Factory defines how to build a provider value.
type Factory[T any] struct {
	// Injects lists constructor dependency tokens.
	Injects []Token

	// ValueScope selects Singleton or Prototype behavior.
	ValueScope Scope

	// Constructor builds the value from resolved dependencies.
	Constructor func(Injections) (T, error)
}

// NewFactory returns a Factory.
func NewFactory[T any](injects []Token, valueScope Scope, constructor func(Injections) (T, error)) Factory[T] {
	return Factory[T]{
		Injects:     injects,
		ValueScope:  valueScope,
		Constructor: constructor,
	}
}

// FactoryProviderInjection provides values from a Factory.
type FactoryProviderInjection[T any] struct {
	Key       Token
	Export    bool
	Factory   Factory[T]
	instance  *Injection
	module    *Module
	tokenOnce sync.Once
}

// FactoryProvider returns a provider backed by factory.
func FactoryProvider[T any](token Token, factory Factory[T], exportable bool) *FactoryProviderInjection[T] {
	return &FactoryProviderInjection[T]{
		Key:     token,
		Export:  exportable,
		Factory: factory,
	}
}

// Token returns the configured or derived provider token.
func (provider *FactoryProviderInjection[T]) Token() Token {
	provider.tokenOnce.Do(func() {
		if provider.Key == "" {
			provider.Key = CreateToken[T]()
		}
	})
	return provider.Key
}

// Injections returns the factory dependency tokens.
func (provider *FactoryProviderInjection[T]) Injections() []Token {
	return provider.Factory.Injects
}

// Create builds or returns the provider value.
func (provider *FactoryProviderInjection[T]) Create(injections Injections) (*Injection, error) {
	token := provider.Token()

	if provider.Factory.Constructor == nil {
		return nil, missingFactoryConstructor(token)
	}

	if provider.Factory.ValueScope == Singleton {
		if provider.instance == nil {
			instance, err := provider.Factory.Constructor(injections)
			if err != nil {
				return nil, err
			}
			provider.instance = &Injection{
				Token:    token,
				Instance: instance,
			}
		}
		return provider.instance, nil
	}

	instance, err := provider.Factory.Constructor(injections)
	if err != nil {
		return nil, err
	}

	return &Injection{
		Token:    token,
		Instance: instance,
	}, nil
}

// Exportable reports whether importers can use this provider.
func (provider *FactoryProviderInjection[T]) Exportable() bool {
	return provider.Export
}

// Scope returns the configured lifecycle.
func (provider *FactoryProviderInjection[T]) Scope() Scope {
	return provider.Factory.ValueScope
}

// AssignOn binds the provider to module.
func (provider *FactoryProviderInjection[T]) AssignOn(module *Module) {
	provider.module = module
}

// AssignedTo returns the owning module.
func (provider *FactoryProviderInjection[T]) AssignedTo() *Module {
	return provider.module
}
