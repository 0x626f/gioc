package gioc

// Module is the unit of organisation in the container. It groups a set of
// providers, declares which other modules it depends on (via Import), and
// optionally marks itself as global so its providers are visible across the
// entire container without an explicit import.
type Module struct {
	token     Token
	global    bool
	imports   []*Module
	exports   map[Token]struct{}
	providers map[Token]IProvider
}

// NewModule creates a new, empty module identified by the given token.
// The token must be unique within the container.
func NewModule(token Token) *Module {
	return &Module{
		token:     token,
		exports:   make(map[Token]struct{}),
		providers: make(map[Token]IProvider),
	}
}

// Token returns the module's unique identifier.
func (module *Module) Token() Token {
	return module.token
}

// Global marks this module as global. Every other module in the container can
// inject providers from a global module without an explicit Import call.
// Global modules are initialised before regular modules.
// Returns the module itself for chaining.
func (module *Module) Global() *Module {
	module.global = true
	return module
}

// Import declares that this module depends on the given modules. Providers
// registered in the imported modules become directly visible to this module's
// own providers during dependency resolution.
// Returns the module itself for chaining.
func (module *Module) Import(modules ...*Module) *Module {
	module.imports = append(module.imports, modules...)
	return module
}

// Provide registers one or more providers with this module. If a provider is
// marked as exportable it is also recorded in the module's export set.
// If two providers share the same token the last one registered wins.
// Returns the module itself for chaining.
func (module *Module) Provide(providers ...IProvider) *Module {
	for _, provider := range providers {
		token := provider.Token()
		if provider.Exportable() {
			module.exports[token] = struct{}{}
		}
		module.providers[token] = provider
		provider.AssignOn(module)
	}
	return module
}

// lookup searches for a provider by token, checking the module's own
// providers first and then the providers of each directly imported module.
// It does not perform transitive (grandparent) import resolution.
func (module *Module) lookup(token Token) IProvider {
	if provider, ok := module.providers[token]; ok {
		return provider
	}

	for _, imp := range module.imports {
		if provider, ok := imp.providers[token]; ok {
			if _, exp := imp.exports[token]; exp {
				return provider
			}
		}
	}

	return nil
}

// providerList returns all providers registered in this module as a slice.
func (module *Module) providerList() []IProvider {
	providers := make([]IProvider, len(module.providers))
	index := 0
	for _, provider := range module.providers {
		providers[index] = provider
		index++
	}
	return providers
}
