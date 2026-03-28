package gioc

type metadata struct {
	loaded map[*Module]struct{}
}

// IOContainer is the top-level dependency injection container.
// It owns a set of modules, resolves their dependency graph, and instantiates
// all providers in the correct topological order when Run is called.
type IOContainer struct {
	modules []*Module
	globals []*Module

	meta *metadata
}

// NewContainer creates a new, empty IOContainer ready to accept modules.
func NewContainer() *IOContainer {
	return &IOContainer{
		meta: &metadata{
			loaded: make(map[*Module]struct{}),
		},
	}
}

// AddModules registers one or more modules with the container.
// Modules must be added before calling Run.
func (container *IOContainer) AddModules(modules ...*Module) {
	container.modules = append(container.modules, modules...)
}

// Run wires the entire container:
//  1. Performs a depth-first traversal of the module import graph to detect
//     circular module dependencies and collect global modules.
//  2. Initialises global modules first, so their singletons are ready before
//     regular modules are processed.
//  3. Initialises all remaining modules, verifying provider dependency graphs
//     for cycles and instantiating each provider in topological order.
//     Providers from global modules are visible to every module during this
//     phase without an explicit Import.
//
// Returns a [CircularInjectionError] if a cycle is found in the module or
// provider graph, or a [DependencyError] if a required dependency is missing
// or a constructor returns an error.
// After a successful run the internal metadata is cleared.
func (container *IOContainer) Run() (err error) {
	err = dfs(container.modules, nodeConfig[*Module]{
		Neighbors: func(module *Module) ([]*Module, error) {
			return module.imports, nil
		},
		ShouldVisit: func(module *Module) bool {
			return true
		},
		OnVisited: func(module *Module) error {
			if module.global {
				container.globals = append(container.globals, module)
			}
			return nil
		},
		OnCycle: func(chain []Token) error {
			return circularModuleInjection(chain...)
		},
	})

	if err != nil {
		return err
	}

	err = container.initModules(container.globals...)
	if err != nil {
		return err
	}

	err = container.initModules(container.modules...)
	if err != nil {
		return err
	}

	container.meta = nil

	return nil
}

// lookup resolves a token for the given module context. It first searches the
// module's own providers and direct imports; if nothing is found it falls
// through to each global module in registration order, stopping as soon as
// the current module is reached in the globals list.
func (container *IOContainer) lookup(module *Module, token Token) IProvider {
	observed := module.lookup(token)
	if observed != nil {
		return observed
	}

	for _, global := range container.globals {
		if global.token == module.token {
			break
		}

		observed = global.lookup(token)

		if observed != nil {
			return observed
		}
	}
	return nil
}

// initModules runs the provider dependency DFS for each module and instantiates
// every provider in topological order via createObject.
func (container *IOContainer) initModules(modules ...*Module) (err error) {
	for _, module := range modules {
		if err = container.initModules(module.imports...); err != nil {
			return
		}
		err = dfs(module.providerList(), nodeConfig[IProvider]{
			Neighbors: func(provider IProvider) ([]IProvider, error) {
				var providers []IProvider
				for _, injection := range provider.Injections() {
					observed := container.lookup(provider.AssignedTo(), injection)
					if observed == nil {
						return nil, missingDependency(provider, injection)
					}
					providers = append(providers, observed)
				}
				return providers, nil
			},
			ShouldVisit: func(provider IProvider) bool {
				if _, ok := container.meta.loaded[module]; ok {
					return false
				}
				return true
			},
			OnVisited: func(provider IProvider) error {
				_, createErr := container.createObject(provider)
				return createErr
			},
			OnCycle: func(provider []Token) error {
				return circularDependencyInjection(provider...)
			},
		})

		if err != nil {
			return err
		}
	}
	return
}

// createObject recursively resolves all declared dependencies of the provider
// and calls its Create method with the fully-built injection list.
func (container *IOContainer) createObject(provider IProvider) (*Injectable, error) {

	var err error
	var injections []*Injectable

	for _, injection := range provider.Injections() {
		var built *Injectable
		observed := container.lookup(provider.AssignedTo(), injection)
		if observed == nil {
			return nil, missingDependency(provider, injection)
		}

		built, err = container.createObject(observed)
		if err != nil {
			return nil, err
		}

		injections = append(injections, built)
	}

	return provider.Create(injections...)
}
