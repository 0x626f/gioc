package gioc

// Module groups providers and imports.
type Module struct {
	token     Token
	global    bool
	imports   []*Module
	exports   map[Token]struct{}
	providers map[Token]IProvider
	err       error
}

// NewModule returns an empty module with token.
func NewModule(token Token) *Module {
	return &Module{
		token:     token,
		exports:   make(map[Token]struct{}),
		providers: make(map[Token]IProvider),
	}
}

// Token returns the module token.
func (module *Module) Token() Token {
	return module.token
}

// Global makes this module visible to all modules.
func (module *Module) Global() *Module {
	module.global = true
	return module
}

// Import makes exported providers from modules visible here.
func (module *Module) Import(modules ...*Module) *Module {
	module.imports = append(module.imports, modules...)
	return module
}

// Provide registers providers on this module.
func (module *Module) Provide(providers ...IProvider) *Module {
	for _, provider := range providers {
		if provider == nil {
			module.err = nilProvider()
			continue
		}

		token := provider.Token()
		if assigned := provider.AssignedTo(); assigned != nil && assigned != module {
			module.err = providerAlreadyAssigned(token, assigned.Token(), module.Token())
			continue
		}

		if provider.Exportable() {
			module.exports[token] = struct{}{}
		} else {
			delete(module.exports, token)
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
