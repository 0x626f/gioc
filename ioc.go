package gioc

import "sync"

type metadata struct {
	loaded    map[*Module]struct{}
	instances map[IProvider]*Injection
}

// Container owns modules and resolves provider dependencies.
type Container struct {
	modules []*Module
	globals []*Module

	meta   *metadata
	mu     sync.Mutex
	ran    bool
	runErr error
}

// NewContainer returns an empty Container.
func NewContainer() *Container {
	return &Container{
		meta: &metadata{
			loaded:    make(map[*Module]struct{}),
			instances: make(map[IProvider]*Injection),
		},
	}
}

// AddModules registers modules before Run.
func (container *Container) AddModules(modules ...*Module) error {
	container.mu.Lock()
	defer container.mu.Unlock()

	if container.ran {
		return moduleRegistrationClosed()
	}

	container.modules = append(container.modules, modules...)
	return nil
}

// Run validates modules and creates provider instances.
func (container *Container) Run() (err error) {
	container.mu.Lock()
	defer container.mu.Unlock()

	if container.ran {
		return container.runErr
	}
	container.ran = true

	defer func() {
		container.runErr = err
	}()

	err = container.validateModules()
	if err != nil {
		return
	}

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
		return
	}

	err = container.initModules(container.globals...)
	if err != nil {
		return
	}

	err = container.initModules(container.modules...)
	if err != nil {
		return
	}

	container.meta.loaded = nil
	return
}

func (container *Container) cached(provider IProvider) (*Injection, bool) {
	injection, ok := container.meta.instances[provider]
	return injection, ok
}

func (container *Container) cache(provider IProvider, injection *Injection) {
	container.meta.instances[provider] = injection
}

// Resolve returns token from the given modules after Run.
func (container *Container) Resolve(token Token, modules ...*Module) (*Injection, error) {
	container.mu.Lock()
	defer container.mu.Unlock()

	if !(container.ran && container.runErr == nil) {
		return nil, containerNotReady()
	}

	if len(modules) == 0 {
		modules = container.modules
	}

	for _, module := range modules {
		if module == nil {
			return nil, nilModule()
		}

		provider := container.lookup(module, token)
		if provider == nil {
			continue
		}

		return container.createObject(provider)
	}

	return nil, missingInjection(token, nil)
}

// Get resolves token from the container as T.
func Get[T any](container *Container, token Token, modules ...*Module) (T, error) {
	injection, err := container.Resolve(token, modules...)
	if err != nil {
		return zero[T](), err
	}
	return resolveInjection[T](injection)
}

// lookup resolves a token for the given module context. It first searches the
// module's own providers and direct imports; if nothing is found it falls
// through to each global module in registration order, skipping the current
// module if it is itself global.
func (container *Container) lookup(module *Module, token Token) IProvider {
	if module == nil {
		return nil
	}

	observed := module.lookup(token)
	if observed != nil {
		return observed
	}

	for _, global := range container.globals {
		if global.token == module.token {
			continue
		}

		observed = global.lookup(token)

		if observed != nil {
			return observed
		}
	}
	return nil
}

func (container *Container) validateModules() error {
	byToken := make(map[Token]*Module)

	for _, module := range container.modules {
		if err := observeModuleToken(byToken, module); err != nil {
			return err
		}
	}

	return dfs(container.modules, nodeConfig[*Module]{
		Neighbors: func(module *Module) ([]*Module, error) {
			if module == nil {
				return nil, nilModule()
			}
			if module.err != nil {
				return nil, module.err
			}
			for _, imp := range module.imports {
				if err := observeModuleToken(byToken, imp); err != nil {
					return nil, err
				}
			}
			return module.imports, nil
		},
		ShouldVisit: func(module *Module) bool {
			return true
		},
		OnVisited: func(module *Module) error {
			return observeModuleToken(byToken, module)
		},
		OnCycle: func(chain []Token) error {
			return nil
		},
	})
}

func observeModuleToken(byToken map[Token]*Module, module *Module) error {
	if module == nil {
		return nilModule()
	}
	if module.err != nil {
		return module.err
	}

	if existing, ok := byToken[module.Token()]; ok && existing != module {
		return duplicateModuleToken(module.Token())
	}
	byToken[module.Token()] = module
	return nil
}

// initModules runs the provider dependency DFS for each module and instantiates
// every provider in topological order via createObject.
func (container *Container) initModules(modules ...*Module) (err error) {
	for _, module := range modules {
		if _, ok := container.meta.loaded[module]; ok {
			continue
		}

		if err = container.initModules(module.imports...); err != nil {
			return
		}
		if _, ok := container.meta.loaded[module]; ok {
			continue
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

		container.meta.loaded[module] = struct{}{}
	}
	return
}

// createObject recursively resolves all declared dependencies of the provider
// and calls its Create method with the fully-built injection list.
func (container *Container) createObject(provider IProvider) (*Injection, error) {
	return container.createObjectWithPath(provider, nil, nil)
}

func (container *Container) createObjectWithPath(provider IProvider, path map[IProvider]struct{}, tokens []Token) (*Injection, error) {
	if provider == nil {
		return nil, nilProvider()
	}

	if _, ok := path[provider]; ok {
		return nil, circularDependencyInjection(append(tokens, provider.Token())...)
	}

	if provider.Scope() == Singleton {
		if built, ok := container.cached(provider); ok {
			return built, nil
		}
	}

	if path == nil {
		path = make(map[IProvider]struct{})
	}
	path[provider] = struct{}{}
	tokens = append(tokens, provider.Token())
	defer delete(path, provider)

	var err error
	var injections Injections

	for _, injection := range provider.Injections() {
		var built *Injection
		observed := container.lookup(provider.AssignedTo(), injection)
		if observed == nil {
			return nil, missingDependency(provider, injection)
		}

		built, err = container.createObjectWithPath(observed, path, tokens)
		if err != nil {
			return nil, err
		}

		injections = append(injections, built)
	}

	built, err := provider.Create(injections)
	if err != nil {
		return nil, err
	}

	if provider.Scope() == Singleton {
		container.cache(provider, built)
	}

	return built, nil
}
