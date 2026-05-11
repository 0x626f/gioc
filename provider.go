package gioc

import "sync"

// IProvider is the common interface implemented by all provider types.
// A provider encapsulates the construction logic for a single dependency and
// carries metadata (token, scope, exportability) used by the container during
// wiring.
type IProvider interface {
	// Token returns the unique identifier for this provider within its module.
	Token() Token

	// Injections returns the ordered list of dependency tokens this provider
	// requires to construct its instance.
	Injections() []Token

	// Create instantiates the managed object using the supplied resolved
	// dependencies and wraps it in an Injectable.
	Create(...*Injectable) (*Injectable, error)

	// Exportable reports whether this provider's token is visible to modules
	// that import the owning module.
	Exportable() bool

	// Scope returns the lifecycle scope (Singleton or Prototype) of the
	// instances produced by this provider.
	Scope() Scope

	// AssignOn binds this provider to the given module. Called automatically
	// by Module.Provide.
	AssignOn(module *Module)

	// AssignedTo returns the module this provider was registered with.
	AssignedTo() *Module
}

// ValueProviderInjection wraps a pre-existing value as a provider.
// It always has Singleton scope — every call to Create returns the same
// underlying value.
type ValueProviderInjection[T any] struct {
	Key       Token
	Export    bool
	Value     T
	module    *Module
	tokenOnce sync.Once
}

// ValueProvider creates a provider that serves the given value as-is, without
// any constructor logic. The token is derived from T when left empty.
// Set exportable to true to make the provider visible to importing modules.
//
//	mod.Provide(ValueProvider[*Config]("", &Config{DSN: "..."}, true))
func ValueProvider[T any](token Token, value T, exportable bool) *ValueProviderInjection[T] {
	return &ValueProviderInjection[T]{
		Key:    token,
		Value:  value,
		Export: exportable,
	}
}

// Token returns the provider's token, deriving it from the type parameter when
// the Key field is empty.
func (provider *ValueProviderInjection[T]) Token() Token {
	provider.tokenOnce.Do(func() {
		if provider.Key == "" {
			provider.Key = CreateToken[T]()
		}
	})
	return provider.Key
}

// Injections always returns an empty slice — a value provider has no
// constructor dependencies.
func (provider *ValueProviderInjection[T]) Injections() []Token {
	return []Token{}
}

// Create wraps the held value in an Injectable. The same value pointer is
// returned on every call (Singleton behaviour).
func (provider *ValueProviderInjection[T]) Create(...*Injectable) (*Injectable, error) {
	return &Injectable{
		Token:    provider.Token(),
		Instance: provider.Value,
	}, nil
}

// Exportable reports whether this provider is visible to importing modules.
func (provider *ValueProviderInjection[T]) Exportable() bool {
	return provider.Export
}

// Scope always returns Singleton for value providers.
func (provider *ValueProviderInjection[T]) Scope() Scope {
	return Singleton
}

// AssignOn binds the provider to its owning module.
func (provider *ValueProviderInjection[T]) AssignOn(module *Module) {
	provider.module = module
}

// AssignedTo returns the module this provider belongs to.
func (provider *ValueProviderInjection[T]) AssignedTo() *Module {
	return provider.module
}

// Factory describes the construction contract for a FactoryProvider: the
// tokens of its dependencies, the desired scope, and the constructor function
// that receives the resolved dependencies and returns a new instance.
type Factory[T any] struct {
	// Injects lists the dependency tokens passed to Constructor, in order.
	Injects []Token

	// ValueScope controls whether a single instance is reused (Singleton) or
	// a new instance is created on every request (Prototype).
	ValueScope Scope

	// Constructor is called with the resolved dependencies each time a new
	// instance is needed (always for Prototype; once for Singleton).
	Constructor func(...*Injectable) (T, error)
}

// FactoryProviderInjection is a provider backed by a constructor function.
// For Singleton scope the instance is created once and cached; for Prototype
// scope the constructor is invoked on every Create call.
type FactoryProviderInjection[T any] struct {
	Key       Token
	Export    bool
	Factory   Factory[T]
	instance  *Injectable
	module    *Module
	tokenOnce sync.Once
}

// FactoryProvider creates a provider that constructs its instance via the
// supplied Factory. The token is derived from T when left empty.
// Set exportable to true to make the provider visible to importing modules.
//
//	mod.Provide(FactoryProvider[*Service]("", Factory[*Service]{
//	    Injects:     Inject("Logger", "Database"),
//	    ValueScope:  Singleton,
//	    Constructor: func(deps ...*Injectable) (*Service, error) {
//	        log, _ := ResolveFrom[*Logger]("Logger", deps)
//	        db,  _ := ResolveFrom[*Database]("Database", deps)
//	        return &Service{Log: log, DB: db}, nil
//	    },
//	}, true))
func FactoryProvider[T any](token Token, factory Factory[T], exportable bool) *FactoryProviderInjection[T] {
	return &FactoryProviderInjection[T]{
		Key:     token,
		Export:  exportable,
		Factory: factory,
	}
}

// Token returns the provider's token, deriving it from the type parameter when
// the Key field is empty.
func (provider *FactoryProviderInjection[T]) Token() Token {
	provider.tokenOnce.Do(func() {
		if provider.Key == "" {
			provider.Key = CreateToken[T]()
		}
	})
	return provider.Key
}

// Injections returns the dependency tokens declared in Factory.Injects.
func (provider *FactoryProviderInjection[T]) Injections() []Token {
	return provider.Factory.Injects
}

// Create builds and returns the managed instance. For Singleton scope the
// constructor is called only on the first invocation; subsequent calls return
// the cached Injectable. For Prototype scope the constructor is called every
// time. Any error from the constructor is propagated directly.
func (provider *FactoryProviderInjection[T]) Create(injections ...*Injectable) (*Injectable, error) {
	token := provider.Token()

	if provider.Factory.Constructor == nil {
		return nil, missingFactoryConstructor(token)
	}

	if provider.Factory.ValueScope == Singleton {
		if provider.instance == nil {
			instance, err := provider.Factory.Constructor(injections...)
			if err != nil {
				return nil, err
			}
			provider.instance = &Injectable{
				Token:    token,
				Instance: instance,
			}
		}
		return provider.instance, nil
	}

	instance, err := provider.Factory.Constructor(injections...)
	if err != nil {
		return nil, err
	}

	return &Injectable{
		Token:    token,
		Instance: instance,
	}, nil
}

// Exportable reports whether this provider is visible to importing modules.
func (provider *FactoryProviderInjection[T]) Exportable() bool {
	return provider.Export
}

// Scope returns the lifecycle scope configured in the Factory.
func (provider *FactoryProviderInjection[T]) Scope() Scope {
	return provider.Factory.ValueScope
}

// AssignOn binds the provider to its owning module.
func (provider *FactoryProviderInjection[T]) AssignOn(module *Module) {
	provider.module = module
}

// AssignedTo returns the module this provider belongs to.
func (provider *FactoryProviderInjection[T]) AssignedTo() *Module {
	return provider.module
}
