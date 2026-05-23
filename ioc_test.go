package gioc

import (
	"errors"
	"strings"
	"sync"
	"testing"
)

type Logger struct{ Prefix string }
type Database struct{ DSN string }
type Cache struct{ Addr string }
type Config struct{ Env string }

type nonCachingSingletonProvider struct {
	token  Token
	calls  int
	mu     sync.Mutex
	module *Module
}

func (provider *nonCachingSingletonProvider) Token() Token {
	return provider.token
}

func (provider *nonCachingSingletonProvider) Injections() []Token {
	return nil
}

func (provider *nonCachingSingletonProvider) Create(Injections) (*Injection, error) {
	provider.mu.Lock()
	defer provider.mu.Unlock()

	provider.calls++
	return &Injection{Token: provider.token, Instance: &Logger{Prefix: "custom"}}, nil
}

func (provider *nonCachingSingletonProvider) Exportable() bool {
	return false
}

func (provider *nonCachingSingletonProvider) Scope() Scope {
	return Singleton
}

func (provider *nonCachingSingletonProvider) AssignOn(module *Module) {
	provider.module = module
}

func (provider *nonCachingSingletonProvider) AssignedTo() *Module {
	return provider.module
}

func (provider *nonCachingSingletonProvider) Calls() int {
	provider.mu.Lock()
	defer provider.mu.Unlock()

	return provider.calls
}

type UserService struct {
	Log *Logger
	DB  *Database
}

type OrderService struct {
	Users *UserService
	Cache *Cache
}

func mustRun(t *testing.T, c *Container) {
	t.Helper()
	if err := c.Run(); err != nil {
		t.Fatalf("container.Run: %v", err)
	}
}

func TestObjectProvider_CreateReturnsInjectable(t *testing.T) {
	db := &Database{DSN: "postgres://localhost"}
	p := ValueProvider[*Database]("", db, false)

	inj, err := p.Create(nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if inj.Instance != db {
		t.Fatal("instance should be the same pointer")
	}
	if inj.Token != "Database" {
		t.Fatalf("expected token Database, got %s", inj.Token)
	}
}

func TestObjectProvider_AutoToken(t *testing.T) {
	p := ValueProvider[*Logger]("", &Logger{}, false)
	if p.Token() != "Logger" {
		t.Fatalf("auto token should be Logger, got %s", p.Token())
	}
}

func TestObjectProvider_CustomToken(t *testing.T) {
	p := ValueProvider[*Logger]("my-logger", &Logger{}, false)
	if p.Token() != "my-logger" {
		t.Fatalf("expected my-logger, got %s", p.Token())
	}
}

func TestObjectProvider_AlwaysSingleton(t *testing.T) {
	p := ValueProvider[*Logger]("", &Logger{}, false)
	if p.Scope() != Singleton {
		t.Fatal("ValueProvider scope should always be Singleton")
	}
}

func TestObjectProvider_NoInjections(t *testing.T) {
	p := ValueProvider[*Logger]("", &Logger{}, false)
	if len(p.Injections()) != 0 {
		t.Fatal("ValueProvider should have no injections")
	}
}

func TestObjectProvider_CreateIdempotent(t *testing.T) {
	db := &Database{DSN: "pg"}
	p := ValueProvider[*Database]("", db, false)

	a, _ := p.Create(nil)
	b, _ := p.Create(nil)
	if a.Instance != b.Instance {
		t.Fatal("ValueProvider should return the same object every time")
	}
}

func TestObjectProvider_ExportFlag(t *testing.T) {
	p1 := ValueProvider[*Logger]("", &Logger{}, true)
	p2 := ValueProvider[*Logger]("", &Logger{}, false)
	if !p1.Exportable() {
		t.Fatal("expected exportable=true")
	}
	if p2.Exportable() {
		t.Fatal("expected exportable=false")
	}
}

func TestObjectProvider_AssignOn(t *testing.T) {
	mod := NewModule("test")
	p := ValueProvider[*Logger]("", &Logger{}, false)
	p.AssignOn(mod)
	if p.AssignedTo() != mod {
		t.Fatal("AssignedTo should return the assigned module")
	}
}

func TestFactoryProvider_Prototype(t *testing.T) {
	calls := 0
	p := FactoryProvider[*Logger]("", Factory[*Logger]{
		ValueScope: Prototype,
		Constructor: func(deps Injections) (*Logger, error) {
			calls++
			return &Logger{Prefix: "v"}, nil
		},
	}, false)

	a, err := p.Create(nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	b, err := p.Create(nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if calls != 2 {
		t.Fatalf("factory should be called twice for Prototype, got %d", calls)
	}
	if a == b {
		t.Fatal("Prototype should return different Injectable pointers")
	}
}

func TestFactoryProvider_Singleton(t *testing.T) {
	calls := 0
	p := FactoryProvider[*Logger]("", Factory[*Logger]{
		ValueScope: Singleton,
		Constructor: func(deps Injections) (*Logger, error) {
			calls++
			return &Logger{Prefix: "v"}, nil
		},
	}, false)

	a, err := p.Create(nil)
	if err != nil {
		t.Fatalf("first Create: %v", err)
	}
	b, err := p.Create(nil)
	if err != nil {
		t.Fatalf("second Create: %v", err)
	}
	if calls != 1 {
		t.Fatalf("factory should be called once for Singleton, got %d", calls)
	}
	if a != b {
		t.Fatal("Singleton should return the same Injectable pointer")
	}
}

func TestFactoryProvider_AutoToken(t *testing.T) {
	p := FactoryProvider[*Logger]("", Factory[*Logger]{
		Constructor: func(deps Injections) (*Logger, error) {
			return &Logger{}, nil
		},
	}, false)
	if p.Token() != "Logger" {
		t.Fatalf("auto token should be Logger, got %s", p.Token())
	}
}

func TestFactoryProvider_CustomToken(t *testing.T) {
	p := FactoryProvider[*Logger]("custom-logger", Factory[*Logger]{
		Constructor: func(deps Injections) (*Logger, error) {
			return &Logger{}, nil
		},
	}, false)
	if p.Token() != "custom-logger" {
		t.Fatalf("expected custom-logger, got %s", p.Token())
	}
}

func TestFactoryProvider_CreateAutoTokenWithoutPriorTokenCall(t *testing.T) {
	p := FactoryProvider[*Logger]("", Factory[*Logger]{
		Constructor: func(deps Injections) (*Logger, error) {
			return &Logger{}, nil
		},
	}, false)

	inj, err := p.Create(nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if inj.Token != "Logger" {
		t.Fatalf("expected token Logger, got %q", inj.Token)
	}
}

func TestFactoryProvider_PrototypeCreateAutoTokenWithoutPriorTokenCall(t *testing.T) {
	p := FactoryProvider[*Logger]("", Factory[*Logger]{
		ValueScope: Prototype,
		Constructor: func(deps Injections) (*Logger, error) {
			return &Logger{}, nil
		},
	}, false)

	inj, err := p.Create(nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if inj.Token != "Logger" {
		t.Fatalf("expected token Logger, got %q", inj.Token)
	}
}

func TestFactoryProvider_InjectionsReturnsInjects(t *testing.T) {
	p := FactoryProvider[*UserService]("", Factory[*UserService]{
		Injects: Inject("Logger", "Database"),
		Constructor: func(deps Injections) (*UserService, error) {
			return &UserService{}, nil
		},
	}, false)

	got := p.Injections()
	if len(got) != 2 {
		t.Fatalf("Injections() should return 2 tokens, got %d: %v", len(got), got)
	}
}

func TestFactoryProvider_ReceivesInjections(t *testing.T) {
	logToken := CreateToken[Logger]()
	dbToken := CreateToken[Database]()

	p := FactoryProvider[*UserService]("", Factory[*UserService]{
		Injects: Inject(logToken, dbToken),
		Constructor: func(deps Injections) (*UserService, error) {
			log, err := Resolve[*Logger](logToken, deps)
			if err != nil {
				return nil, err
			}
			db, err := Resolve[*Database](dbToken, deps)
			if err != nil {
				return nil, err
			}
			return &UserService{Log: log, DB: db}, nil
		},
	}, false)

	injections := []*Injection{
		{Token: logToken, Instance: &Logger{Prefix: "test"}},
		{Token: dbToken, Instance: &Database{DSN: "pg"}},
	}

	inj, err := p.Create(injections)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	svc, ok := inj.Instance.(*UserService)
	if !ok {
		t.Fatal("instance should be *UserService")
	}
	if svc.Log.Prefix != "test" {
		t.Fatalf("expected prefix test, got %s", svc.Log.Prefix)
	}
	if svc.DB.DSN != "pg" {
		t.Fatalf("expected DSN pg, got %s", svc.DB.DSN)
	}
}

func TestNewFactory(t *testing.T) {
	constructor := func(deps Injections) (*Logger, error) {
		return &Logger{Prefix: "new"}, nil
	}

	factory := NewFactory[*Logger](Inject("Logger"), constructor, Prototype)

	if len(factory.Injects) != 1 || factory.Injects[0] != "Logger" {
		t.Fatalf("unexpected injects: %v", factory.Injects)
	}
	if factory.ValueScope != Prototype {
		t.Fatalf("expected Prototype scope, got %v", factory.ValueScope)
	}
	log, err := factory.Constructor(nil)
	if err != nil {
		t.Fatalf("unexpected constructor error: %v", err)
	}
	if log.Prefix != "new" {
		t.Fatalf("expected prefix new, got %s", log.Prefix)
	}
}

func TestFactoryProvider_ExportFlag(t *testing.T) {
	p := FactoryProvider[*Logger]("", Factory[*Logger]{
		Constructor: func(deps Injections) (*Logger, error) { return &Logger{}, nil },
	}, true)
	if !p.Exportable() {
		t.Fatal("expected exportable")
	}
}

func TestFactoryProvider_ScopeField(t *testing.T) {
	p1 := FactoryProvider[*Logger]("", Factory[*Logger]{ValueScope: Singleton}, false)
	p2 := FactoryProvider[*Logger]("", Factory[*Logger]{ValueScope: Prototype}, false)
	if p1.Scope() != Singleton {
		t.Fatal("expected Singleton")
	}
	if p2.Scope() != Prototype {
		t.Fatal("expected Prototype")
	}
}

func TestFactoryProvider_NilConstructorReturnsError(t *testing.T) {
	p := FactoryProvider[*Logger]("", Factory[*Logger]{}, false)

	if _, err := p.Create(nil); err == nil {
		t.Fatal("expected error for nil constructor")
	}
}

func TestDerive_HappyPath(t *testing.T) {
	injections := []*Injection{
		{Token: "Database", Instance: &Database{DSN: "pg"}},
		{Token: "Logger", Instance: &Logger{Prefix: "x"}},
	}

	db, err := Resolve[*Database]("Database", injections)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if db.DSN != "pg" {
		t.Fatalf("expected DSN pg, got %s", db.DSN)
	}
}

func TestDerive_SecondElement(t *testing.T) {
	injections := []*Injection{
		{Token: "A", Instance: &Logger{Prefix: "a"}},
		{Token: "B", Instance: &Logger{Prefix: "b"}},
	}
	log, err := Resolve[*Logger]("B", injections)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if log.Prefix != "b" {
		t.Fatalf("expected prefix b, got %s", log.Prefix)
	}
}

func TestDerive_NotFound(t *testing.T) {
	_, err := Resolve[*Database]("Missing", nil)
	if err == nil {
		t.Fatal("expected error for missing injection")
	}
}

func TestDerive_WrongType(t *testing.T) {
	injections := []*Injection{
		{Token: "Database", Instance: "not a database"},
	}
	_, err := Resolve[*Database]("Database", injections)
	if err == nil {
		t.Fatal("expected error for wrong type")
	}
}

func TestDerive_EmptySlice(t *testing.T) {
	_, err := Resolve[*Database]("X", []*Injection{})
	if err == nil {
		t.Fatal("expected error for empty injections")
	}
}

func TestDerive_NilInjectionReturnsError(t *testing.T) {
	_, err := Resolve[*Database]("Database", []*Injection{nil})
	if err == nil {
		t.Fatal("expected error for nil injection")
	}
}

func TestModule_ProvideAndLookup(t *testing.T) {
	mod := NewModule("app")
	p := ValueProvider[*Logger]("", &Logger{Prefix: "app"}, false)
	mod.Provide(p)

	found := mod.lookup(p.Token())
	if found == nil {
		t.Fatal("should find provider in own module")
	}
}

func TestModule_LookupMissing(t *testing.T) {
	mod := NewModule("app")
	found := mod.lookup("NonExistent")
	if found != nil {
		t.Fatal("should return nil for unknown token")
	}
}

func TestModule_ImportLookup(t *testing.T) {
	infra := NewModule("infra")
	p := ValueProvider[*Database]("", &Database{DSN: "pg"}, true)
	infra.Provide(p)

	app := NewModule("app")
	app.Import(infra)

	found := app.lookup(p.Token())
	if found == nil {
		t.Fatal("should find provider from imported module")
	}
}

func TestModule_ImportChainDoesNotTransit(t *testing.T) {
	c := NewModule("C")
	c.Provide(ValueProvider[*Cache]("", &Cache{Addr: "redis"}, true))

	b := NewModule("B")
	b.Import(c)

	a := NewModule("A")
	a.Import(b)

	found := a.lookup(CreateToken[Cache]())
	if found != nil {
		t.Fatal("transitive import should NOT expose providers (only direct imports)")
	}
}

func TestModule_MultipleImports(t *testing.T) {
	m1 := NewModule("m1")
	m1.Provide(ValueProvider[*Logger]("", &Logger{Prefix: "m1"}, false))

	m2 := NewModule("m2")
	m2.Provide(ValueProvider[*Database]("", &Database{DSN: "pg"}, false))

	app := NewModule("app")
	app.Import(m1, m2)

	if app.lookup(CreateToken[Logger]()) != nil {
		t.Fatal("should not find Logger from m1")
	}
	if app.lookup(CreateToken[Database]()) != nil {
		t.Fatal("should not find Database from m2")
	}
}

func TestModule_OwnProviderShadowsImport(t *testing.T) {
	infra := NewModule("infra")
	infra.Provide(ValueProvider[*Logger]("", &Logger{Prefix: "infra"}, false))

	app := NewModule("app")
	app.Import(infra)
	app.Provide(ValueProvider[*Logger]("", &Logger{Prefix: "app-local"}, false))

	found := app.lookup(CreateToken[Logger]())
	if found == nil {
		t.Fatal("should find provider")
	}

	inj, _ := found.Create(nil)
	log := inj.Instance.(*Logger)
	if log.Prefix != "app-local" {
		t.Fatalf("own provider should shadow import, got prefix %s", log.Prefix)
	}
}

func TestModule_ProvidersCount(t *testing.T) {
	mod := NewModule("m")
	mod.Provide(
		ValueProvider[*Logger]("", &Logger{}, false),
		ValueProvider[*Database]("", &Database{}, false),
		ValueProvider[*Cache]("", &Cache{}, false),
	)
	if len(mod.providerList()) != 3 {
		t.Fatalf("expected 3 providers, got %d", len(mod.providerList()))
	}
}

func TestModule_ProvideExportAddsToExports(t *testing.T) {
	mod := NewModule("m")
	mod.Provide(ValueProvider[*Logger]("", &Logger{}, true))
	mod.Provide(ValueProvider[*Database]("", &Database{}, false))

	_, logExported := mod.exports[CreateToken[Logger]()]
	_, dbExported := mod.exports[CreateToken[Database]()]

	if !logExported {
		t.Fatal("Logger should be in exports map")
	}
	if dbExported {
		t.Fatal("Database should NOT be in exports map")
	}
}

func TestModule_DuplicateTokenOverwrites(t *testing.T) {
	mod := NewModule("m")
	mod.Provide(ValueProvider[*Logger]("", &Logger{Prefix: "first"}, false))
	mod.Provide(ValueProvider[*Logger]("", &Logger{Prefix: "second"}, false))

	found := mod.lookup(CreateToken[Logger]())
	inj, _ := found.Create(nil)
	log := inj.Instance.(*Logger)
	if log.Prefix != "second" {
		t.Fatalf("last provider should win, got prefix %s", log.Prefix)
	}
}

func TestModule_DuplicateTokenClearsExportWhenReplacementIsPrivate(t *testing.T) {
	infra := NewModule("infra")
	infra.Provide(ValueProvider[*Logger]("", &Logger{Prefix: "public"}, true))
	infra.Provide(ValueProvider[*Logger]("", &Logger{Prefix: "private"}, false))

	app := NewModule("app")
	app.Import(infra)

	if found := app.lookup(CreateToken[Logger]()); found != nil {
		t.Fatal("replacement provider should not inherit the previous export flag")
	}
}

func TestModule_ProvideNilReturnsRunError(t *testing.T) {
	mod := NewModule("app")
	mod.Provide(nil)

	c := NewContainer()
	if err := c.AddModules(mod); err != nil {
		t.Fatalf("unexpected AddModules error: %v", err)
	}

	if err := c.Run(); err == nil {
		t.Fatal("expected nil provider registration error")
	}
}

func TestModule_SameProviderCannotBeAssignedToMultipleModules(t *testing.T) {
	provider := ValueProvider[*Logger]("", &Logger{Prefix: "shared"}, true)

	first := NewModule("first")
	first.Provide(provider)

	second := NewModule("second")
	second.Provide(provider)

	c := NewContainer()
	if err := c.AddModules(first, second); err != nil {
		t.Fatalf("unexpected AddModules error: %v", err)
	}

	if err := c.Run(); err == nil {
		t.Fatal("expected provider assignment error")
	}
	if provider.AssignedTo() != first {
		t.Fatal("provider should remain assigned to the original module")
	}
}

func TestContainer_SimpleObjectProviders(t *testing.T) {
	c := NewContainer()
	mod := NewModule("app")
	mod.Provide(
		ValueProvider[*Logger]("", &Logger{Prefix: "app"}, false),
		ValueProvider[*Database]("", &Database{DSN: "pg"}, false),
	)
	c.AddModules(mod)
	mustRun(t, c)
}

func TestContainer_EmptyModule(t *testing.T) {
	c := NewContainer()
	c.AddModules(NewModule("empty"))
	mustRun(t, c)
}

func TestContainer_NoModules(t *testing.T) {
	c := NewContainer()
	mustRun(t, c)
}

func TestContainer_RunCanBeCalledTwice(t *testing.T) {
	c := NewContainer()
	mod := NewModule("app")
	mod.Provide(ValueProvider[*Logger]("", &Logger{Prefix: "app"}, false))
	c.AddModules(mod)

	mustRun(t, c)
	mustRun(t, c)
}

func TestContainer_AddModulesAfterRunReturnsErrorAndDoesNotRegister(t *testing.T) {
	c := NewContainer()
	first := NewModule("first")
	first.Provide(ValueProvider[*Logger]("", &Logger{Prefix: "first"}, false))
	c.AddModules(first)
	mustRun(t, c)

	second := NewModule("second")
	second.Provide(ValueProvider[*Database]("", &Database{DSN: "late"}, false))
	if err := c.AddModules(second); err == nil {
		t.Fatal("expected AddModules to reject registration after Run")
	}

	if _, err := c.Resolve(CreateToken[Database]()); err == nil {
		t.Fatal("late module should not be visible to Resolve")
	}
}

func TestContainer_RunFailureIsReturnedOnSubsequentRun(t *testing.T) {
	mod := NewModule("app")
	mod.Provide(FactoryProvider[*UserService]("", Factory[*UserService]{
		Injects: Inject("missing"),
		Constructor: func(deps Injections) (*UserService, error) {
			return &UserService{}, nil
		},
	}, false))

	c := NewContainer()
	c.AddModules(mod)

	firstErr := c.Run()
	if firstErr == nil {
		t.Fatal("expected first Run to fail")
	}
	secondErr := c.Run()
	if secondErr == nil {
		t.Fatal("expected second Run to return the stored failure")
	}
	if secondErr.Error() != firstErr.Error() {
		t.Fatalf("expected same failure on second Run, got %v then %v", firstErr, secondErr)
	}
}

func TestContainer_MultipleModulesNoImport(t *testing.T) {
	c := NewContainer()
	a := NewModule("a")
	a.Provide(ValueProvider[*Logger]("", &Logger{}, false))
	b := NewModule("b")
	b.Provide(ValueProvider[*Database]("", &Database{}, false))
	c.AddModules(a, b)
	mustRun(t, c)
}

func TestContainer_DuplicateModuleTokensReturnError(t *testing.T) {
	first := NewModule("app")
	first.Provide(ValueProvider[*Logger]("", &Logger{}, false))

	second := NewModule("app")
	second.Provide(ValueProvider[*Database]("", &Database{}, false))

	c := NewContainer()
	c.AddModules(first, second)

	err := c.Run()
	if err == nil {
		t.Fatal("expected duplicate module token error")
	}
	if !strings.Contains(err.Error(), "registered more than once") {
		t.Fatalf("expected duplicate module token message, got: %v", err)
	}
}

func TestContainer_DuplicateImportedModuleTokensReturnError(t *testing.T) {
	sharedA := NewModule("shared")
	sharedB := NewModule("shared")

	app := NewModule("app")
	app.Import(sharedA, sharedB)

	c := NewContainer()
	c.AddModules(app)

	if err := c.Run(); err == nil {
		t.Fatal("expected duplicate imported module token error")
	}
}

func TestContainer_ImportedModuleInitializedOnce(t *testing.T) {
	calls := 0

	infra := NewModule("infra")
	infra.Provide(FactoryProvider[*Logger]("", Factory[*Logger]{
		ValueScope: Prototype,
		Constructor: func(deps Injections) (*Logger, error) {
			calls++
			return &Logger{Prefix: "infra"}, nil
		},
	}, true))

	app := NewModule("app")
	app.Import(infra)

	c := NewContainer()
	c.AddModules(infra, app)
	mustRun(t, c)

	if calls != 1 {
		t.Fatalf("imported module should be initialized once, got %d calls", calls)
	}
}

func TestContainer_GlobalModuleLookup(t *testing.T) {
	c := NewContainer()

	global := NewModule("global")
	global.Global()
	global.Provide(ValueProvider[*Logger]("", &Logger{Prefix: "g"}, true))

	app := NewModule("app")
	app.Provide(ValueProvider[*Database]("", &Database{DSN: "pg"}, false))

	c.AddModules(global, app)
	mustRun(t, c)

	found := c.lookup(app, CreateToken[Logger]())
	if found == nil {
		t.Fatal("should find global provider from app module context")
	}
}

func TestContainer_GlobalNotVisibleToSelf(t *testing.T) {
	c := NewContainer()

	global := NewModule("shared")
	global.Global()
	global.Provide(ValueProvider[*Logger]("", &Logger{Prefix: "g"}, true))

	c.AddModules(global)
	mustRun(t, c)

	found := c.lookup(global, CreateToken[Logger]())
	if found == nil {
		t.Fatal("global module should still resolve its own providers")
	}
}

func TestContainer_LookupPrefersOwnOverGlobal(t *testing.T) {
	c := NewContainer()

	global := NewModule("global")
	global.Global()
	global.Provide(ValueProvider[*Logger]("", &Logger{Prefix: "global"}, true))

	app := NewModule("app")
	app.Provide(ValueProvider[*Logger]("", &Logger{Prefix: "local"}, false))

	c.AddModules(global, app)
	mustRun(t, c)

	found := c.lookup(app, CreateToken[Logger]())
	if found == nil {
		t.Fatal("should find provider")
	}
	inj, _ := found.Create(nil)
	log := inj.Instance.(*Logger)
	if log.Prefix != "local" {
		t.Fatalf("own module should take priority over global, got %s", log.Prefix)
	}
}

func TestContainer_LookupMissing(t *testing.T) {
	c := NewContainer()
	mod := NewModule("app")
	c.AddModules(mod)
	mustRun(t, c)

	found := c.lookup(mod, "does-not-exist")
	if found != nil {
		t.Fatal("should return nil for unknown token")
	}
}

func TestContainer_LookupNilModuleReturnsNil(t *testing.T) {
	c := NewContainer()

	found := c.lookup(nil, "Logger")
	if found != nil {
		t.Fatal("nil module lookup should return nil")
	}
}

func TestContainer_ResolveAfterRun(t *testing.T) {
	logToken := CreateToken[Logger]()

	c := NewContainer()
	mod := NewModule("app")
	mod.Provide(ValueProvider[*Logger]("", &Logger{Prefix: "resolved"}, false))
	c.AddModules(mod)
	mustRun(t, c)

	inj, err := c.Resolve(logToken, mod)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	log, err := resolveInjection[*Logger](inj)
	if err != nil {
		t.Fatalf("unexpected resolve error: %v", err)
	}
	if log.Prefix != "resolved" {
		t.Fatalf("expected resolved, got %s", log.Prefix)
	}
}

func TestContainer_ResolveWithoutModulesSearchesRootModules(t *testing.T) {
	logToken := CreateToken[Logger]()

	c := NewContainer()
	mod := NewModule("app")
	mod.Provide(ValueProvider[*Logger]("", &Logger{Prefix: "root"}, false))
	c.AddModules(mod)
	mustRun(t, c)

	inj, err := c.Resolve(logToken)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	log, err := resolveInjection[*Logger](inj)
	if err != nil {
		t.Fatalf("unexpected resolve error: %v", err)
	}
	if log.Prefix != "root" {
		t.Fatalf("expected root, got %s", log.Prefix)
	}
}

func TestContainer_ResolveUsesFirstMatchingModule(t *testing.T) {
	logToken := CreateToken[Logger]()

	first := NewModule("first")
	first.Provide(ValueProvider[*Logger]("", &Logger{Prefix: "first"}, false))

	second := NewModule("second")
	second.Provide(ValueProvider[*Logger]("", &Logger{Prefix: "second"}, false))

	c := NewContainer()
	c.AddModules(first, second)
	mustRun(t, c)

	inj, err := c.Resolve(logToken, second, first)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	log, err := resolveInjection[*Logger](inj)
	if err != nil {
		t.Fatalf("unexpected resolve error: %v", err)
	}
	if log.Prefix != "second" {
		t.Fatalf("expected second module to win, got %s", log.Prefix)
	}
}

func TestContainer_ResolveReturnsCreatedInstanceAfterRun(t *testing.T) {
	logToken := CreateToken[Logger]()
	calls := 0

	c := NewContainer()
	mod := NewModule("app")
	mod.Provide(FactoryProvider[*Logger]("", Factory[*Logger]{
		Constructor: func(deps Injections) (*Logger, error) {
			calls++
			return &Logger{Prefix: "created"}, nil
		},
	}, false))
	c.AddModules(mod)
	mustRun(t, c)

	if calls != 1 {
		t.Fatalf("expected factory to run once during Run, got %d", calls)
	}

	first, err := c.Resolve(logToken, mod)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	second, err := c.Resolve(logToken, mod)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if first != second {
		t.Fatal("Resolve should return the cached Injectable created during Run")
	}
	if calls != 1 {
		t.Fatalf("Resolve should not recreate the factory instance, got %d calls", calls)
	}
}

func TestContainer_ResolveValueProviderUsesContainerSingletonCache(t *testing.T) {
	logToken := CreateToken[Logger]()

	c := NewContainer()
	mod := NewModule("app")
	mod.Provide(ValueProvider[*Logger]("", &Logger{Prefix: "cached"}, false))
	c.AddModules(mod)
	mustRun(t, c)

	first, err := c.Resolve(logToken, mod)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	second, err := c.Resolve(logToken, mod)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if first != second {
		t.Fatal("singleton ValueProvider should return the cached Injectable wrapper")
	}
}

func TestContainer_ResolveCustomSingletonUsesContainerCache(t *testing.T) {
	provider := &nonCachingSingletonProvider{token: "CustomSingleton"}
	mod := NewModule("app")
	mod.Provide(provider)

	c := NewContainer()
	c.AddModules(mod)
	mustRun(t, c)

	first, err := c.Resolve(provider.Token(), mod)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	second, err := c.Resolve(provider.Token(), mod)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if first != second {
		t.Fatal("container should enforce singleton identity for custom providers")
	}
	if provider.Calls() != 1 {
		t.Fatalf("custom singleton Create should be called once, got %d", provider.Calls())
	}
}

func TestContainer_ResolveSingletonIsConcurrentSafe(t *testing.T) {
	logToken := CreateToken[Logger]()
	calls := 0
	var callsMu sync.Mutex

	c := NewContainer()
	mod := NewModule("app")
	mod.Provide(FactoryProvider[*Logger]("", Factory[*Logger]{
		ValueScope: Singleton,
		Constructor: func(deps Injections) (*Logger, error) {
			callsMu.Lock()
			defer callsMu.Unlock()

			calls++
			return &Logger{Prefix: "created"}, nil
		},
	}, false))
	c.AddModules(mod)
	mustRun(t, c)

	const workers = 32
	var wg sync.WaitGroup
	results := make([]*Injection, workers)
	errs := make([]error, workers)

	wg.Add(workers)
	for i := 0; i < workers; i++ {
		i := i
		go func() {
			defer wg.Done()
			results[i], errs[i] = c.Resolve(logToken, mod)
		}()
	}
	wg.Wait()

	for _, err := range errs {
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	}
	for i := 1; i < workers; i++ {
		if results[i] != results[0] {
			t.Fatal("all concurrent singleton Resolve calls should return the same Injectable")
		}
	}

	callsMu.Lock()
	defer callsMu.Unlock()
	if calls != 1 {
		t.Fatalf("factory should be called once, got %d", calls)
	}
}

func TestContainer_ResolvePrototypeCreatesNewInstance(t *testing.T) {
	logToken := CreateToken[Logger]()
	calls := 0

	c := NewContainer()
	mod := NewModule("app")
	mod.Provide(FactoryProvider[*Logger]("", Factory[*Logger]{
		ValueScope: Prototype,
		Constructor: func(deps Injections) (*Logger, error) {
			calls++
			return &Logger{Prefix: "created"}, nil
		},
	}, false))
	c.AddModules(mod)
	mustRun(t, c)

	if calls != 1 {
		t.Fatalf("expected prototype factory to run once during Run, got %d", calls)
	}

	first, err := c.Resolve(logToken, mod)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	second, err := c.Resolve(logToken, mod)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if first == second {
		t.Fatal("prototype Resolve should return a fresh Injectable")
	}
	if calls != 3 {
		t.Fatalf("expected prototype factory to run during each Resolve, got %d calls", calls)
	}
}

func TestContainer_ResolveNilModuleReturnsError(t *testing.T) {
	c := NewContainer()

	if _, err := c.Resolve("Logger", nil); err == nil {
		t.Fatal("expected error for nil module")
	}
}

func TestGet_HappyPath(t *testing.T) {
	logToken := CreateToken[Logger]()

	c := NewContainer()
	mod := NewModule("app")
	mod.Provide(ValueProvider[*Logger]("", &Logger{Prefix: "typed"}, false))
	c.AddModules(mod)
	mustRun(t, c)

	log, err := Get[*Logger](c, logToken, mod)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if log.Prefix != "typed" {
		t.Fatalf("expected typed, got %s", log.Prefix)
	}
}

func TestGet_WithoutModulesSearchesRootModules(t *testing.T) {
	logToken := CreateToken[Logger]()

	c := NewContainer()
	mod := NewModule("app")
	mod.Provide(ValueProvider[*Logger]("", &Logger{Prefix: "root"}, false))
	c.AddModules(mod)
	mustRun(t, c)

	log, err := Get[*Logger](c, logToken)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if log.Prefix != "root" {
		t.Fatalf("expected root, got %s", log.Prefix)
	}
}

func TestGet_ResolveError(t *testing.T) {
	c := NewContainer()

	_, err := Get[*Logger](c, "Logger")
	if err == nil {
		t.Fatal("expected error before container Run")
	}

	var depErr *DependencyError
	if !errors.As(err, &depErr) {
		t.Fatalf("expected DependencyError, got %T", err)
	}
}

func TestGet_WrongType(t *testing.T) {
	logToken := CreateToken[Logger]()

	c := NewContainer()
	mod := NewModule("app")
	mod.Provide(ValueProvider[string](logToken, "not-a-logger", false))
	c.AddModules(mod)
	mustRun(t, c)

	_, err := Get[*Logger](c, logToken, mod)
	if err == nil {
		t.Fatal("expected error for wrong type")
	}

	var depErr *DependencyError
	if !errors.As(err, &depErr) {
		t.Fatalf("expected DependencyError, got %T", err)
	}
}

func TestContainer_FactoryProviderNoDeps(t *testing.T) {
	c := NewContainer()
	mod := NewModule("app")
	mod.Provide(FactoryProvider[*Logger]("", Factory[*Logger]{
		Constructor: func(deps Injections) (*Logger, error) {
			return &Logger{Prefix: "factory"}, nil
		},
	}, false))
	c.AddModules(mod)
	mustRun(t, c)
}

func TestContainer_FactoryWithSingleDep(t *testing.T) {
	logToken := CreateToken[Logger]()

	c := NewContainer()
	mod := NewModule("app")
	mod.Provide(
		ValueProvider[*Logger]("", &Logger{Prefix: "injected"}, false),
		FactoryProvider[*UserService]("", Factory[*UserService]{
			Injects: Inject(logToken),
			Constructor: func(deps Injections) (*UserService, error) {
				log, err := Resolve[*Logger](logToken, deps)
				if err != nil {
					return nil, err
				}
				return &UserService{Log: log}, nil
			},
		}, false),
	)
	c.AddModules(mod)
	mustRun(t, c)
}

func TestContainer_FactoryWithMultipleDeps(t *testing.T) {
	logToken := CreateToken[Logger]()
	dbToken := CreateToken[Database]()

	c := NewContainer()
	mod := NewModule("app")
	mod.Provide(
		ValueProvider[*Logger]("", &Logger{Prefix: "test"}, false),
		ValueProvider[*Database]("", &Database{DSN: "pg"}, false),
		FactoryProvider[*UserService]("", Factory[*UserService]{
			Injects: Inject(logToken, dbToken),
			Constructor: func(deps Injections) (*UserService, error) {
				log, _ := Resolve[*Logger](logToken, deps)
				db, _ := Resolve[*Database](dbToken, deps)
				return &UserService{Log: log, DB: db}, nil
			},
		}, false),
	)
	c.AddModules(mod)
	mustRun(t, c)
}

func TestContainer_FactoryDependsOnFactory(t *testing.T) {
	logToken := CreateToken[Logger]()
	dbToken := CreateToken[Database]()
	userSvcToken := CreateToken[UserService]()

	c := NewContainer()
	mod := NewModule("app")
	mod.Provide(
		ValueProvider[*Logger]("", &Logger{Prefix: "deep"}, false),
		ValueProvider[*Database]("", &Database{DSN: "pg"}, false),
		FactoryProvider[*UserService]("", Factory[*UserService]{
			Injects: Inject(logToken, dbToken),
			Constructor: func(deps Injections) (*UserService, error) {
				log, _ := Resolve[*Logger](logToken, deps)
				db, _ := Resolve[*Database](dbToken, deps)
				return &UserService{Log: log, DB: db}, nil
			},
		}, false),
		FactoryProvider[*OrderService]("", Factory[*OrderService]{
			Injects: Inject(userSvcToken),
			Constructor: func(deps Injections) (*OrderService, error) {
				users, _ := Resolve[*UserService](userSvcToken, deps)
				return &OrderService{Users: users}, nil
			},
		}, false),
	)
	c.AddModules(mod)
	mustRun(t, c)
}

func TestContainer_CrossModuleInjection(t *testing.T) {
	logToken := CreateToken[Logger]()

	infra := NewModule("infra")
	infra.Provide(ValueProvider[*Logger]("", &Logger{Prefix: "infra"}, true))

	app := NewModule("app")
	app.Import(infra)
	app.Provide(FactoryProvider[*UserService]("", Factory[*UserService]{
		Injects: Inject(logToken),
		Constructor: func(deps Injections) (*UserService, error) {
			log, _ := Resolve[*Logger](logToken, deps)
			return &UserService{Log: log}, nil
		},
	}, false))

	c := NewContainer()
	c.AddModules(infra, app)
	mustRun(t, c)
}

func TestCircularModuleDependency(t *testing.T) {
	a := NewModule("A")
	b := NewModule("B")
	a.Import(b)
	b.Import(a)

	c := NewContainer()
	c.AddModules(a)
	err := c.Run()
	if err == nil {
		t.Fatal("expected circular dependency error")
	}
	if !strings.Contains(err.Error(), "circular") {
		t.Fatalf("expected 'circular' in error, got: %v", err)
	}
}

func TestCircularModuleDependency_ThreeWay(t *testing.T) {
	a := NewModule("A")
	b := NewModule("B")
	cc := NewModule("C")
	a.Import(b)
	b.Import(cc)
	cc.Import(a)

	container := NewContainer()
	container.AddModules(a)
	err := container.Run()
	if err == nil {
		t.Fatal("expected circular dependency error for A->B->C->A")
	}
	if !strings.Contains(err.Error(), "circular") {
		t.Fatalf("expected 'circular' in error, got: %v", err)
	}
}

func TestModuleSelfImport(t *testing.T) {
	a := NewModule("A")
	a.Import(a)

	c := NewContainer()
	c.AddModules(a)
	err := c.Run()
	if err == nil {
		t.Fatal("expected error for self-importing module")
	}
}

type ServiceA struct{ B *ServiceB }
type ServiceB struct{ A *ServiceA }
type ServiceC struct{ Name string }
type ServiceD struct{ Name string }

func TestCircularProviderDependency_TwoWay(t *testing.T) {
	tokenA := "ServiceA"
	tokenB := "ServiceB"

	mod := NewModule("app")
	mod.Provide(
		FactoryProvider[*ServiceA](tokenA, Factory[*ServiceA]{
			Injects: Inject(tokenB),
			Constructor: func(deps Injections) (*ServiceA, error) {
				b, _ := Resolve[*ServiceB](tokenB, deps)
				return &ServiceA{B: b}, nil
			},
		}, false),
		FactoryProvider[*ServiceB](tokenB, Factory[*ServiceB]{
			Injects: Inject(tokenA),
			Constructor: func(deps Injections) (*ServiceB, error) {
				a, _ := Resolve[*ServiceA](tokenA, deps)
				return &ServiceB{A: a}, nil
			},
		}, false),
	)

	c := NewContainer()
	c.AddModules(mod)
	err := c.Run()
	if err == nil {
		t.Fatal("expected circular dependency injection error for A<->B")
	}
	if !strings.Contains(err.Error(), "circular") {
		t.Fatalf("expected 'circular' in error, got: %v", err)
	}
}

func TestContainer_ResolveBeforeRunReturnsNotReady(t *testing.T) {
	tokenA := "ServiceA"
	tokenB := "ServiceB"

	mod := NewModule("app")
	mod.Provide(
		FactoryProvider[*ServiceA](tokenA, Factory[*ServiceA]{
			Injects: Inject(tokenB),
			Constructor: func(deps Injections) (*ServiceA, error) {
				return &ServiceA{}, nil
			},
		}, false),
		FactoryProvider[*ServiceB](tokenB, Factory[*ServiceB]{
			Injects: Inject(tokenA),
			Constructor: func(deps Injections) (*ServiceB, error) {
				return &ServiceB{}, nil
			},
		}, false),
	)

	c := NewContainer()
	c.AddModules(mod)

	_, err := c.Resolve(tokenA, mod)
	if err == nil {
		t.Fatal("expected error before Run")
	}
	if !strings.Contains(err.Error(), "Run successfully") {
		t.Fatalf("expected not-ready error, got: %v", err)
	}
}

func TestCircularProviderDependency_ThreeWay(t *testing.T) {
	tokenA := "ServiceA"
	tokenB := "ServiceB"
	tokenC := "ServiceC"

	mod := NewModule("app")
	mod.Provide(
		FactoryProvider[*ServiceA](tokenA, Factory[*ServiceA]{
			Injects: Inject(tokenB),
			Constructor: func(deps Injections) (*ServiceA, error) {
				return &ServiceA{}, nil
			},
		}, false),
		FactoryProvider[*ServiceB](tokenB, Factory[*ServiceB]{
			Injects: Inject(tokenC),
			Constructor: func(deps Injections) (*ServiceB, error) {
				return &ServiceB{}, nil
			},
		}, false),
		FactoryProvider[*ServiceC](tokenC, Factory[*ServiceC]{
			Injects: Inject(tokenA),
			Constructor: func(deps Injections) (*ServiceC, error) {
				return &ServiceC{}, nil
			},
		}, false),
	)

	c := NewContainer()
	c.AddModules(mod)
	err := c.Run()
	if err == nil {
		t.Fatal("expected circular dependency injection error for A->B->C->A")
	}
	if !strings.Contains(err.Error(), "circular") {
		t.Fatalf("expected 'circular' in error, got: %v", err)
	}
}

func TestCircularProviderDependency_SelfInjection(t *testing.T) {
	tokenA := "ServiceA"

	mod := NewModule("app")
	mod.Provide(
		FactoryProvider[*ServiceA](tokenA, Factory[*ServiceA]{
			Injects: Inject(tokenA),
			Constructor: func(deps Injections) (*ServiceA, error) {
				return &ServiceA{}, nil
			},
		}, false),
	)

	c := NewContainer()
	c.AddModules(mod)
	err := c.Run()
	if err == nil {
		t.Fatal("expected circular dependency error for self-injecting provider")
	}
	if !strings.Contains(err.Error(), "circular") {
		t.Fatalf("expected 'circular' in error, got: %v", err)
	}
}

func TestCircularProviderDependency_FourWayChain(t *testing.T) {
	tokenA := "ServiceA"
	tokenB := "ServiceB"
	tokenC := "ServiceC"
	tokenD := "ServiceD"

	mod := NewModule("app")
	mod.Provide(
		FactoryProvider[*ServiceA](tokenA, Factory[*ServiceA]{
			Injects: Inject(tokenB),
			Constructor: func(deps Injections) (*ServiceA, error) {
				return &ServiceA{}, nil
			},
		}, false),
		FactoryProvider[*ServiceB](tokenB, Factory[*ServiceB]{
			Injects: Inject(tokenC),
			Constructor: func(deps Injections) (*ServiceB, error) {
				return &ServiceB{}, nil
			},
		}, false),
		FactoryProvider[*ServiceC](tokenC, Factory[*ServiceC]{
			Injects: Inject(tokenD),
			Constructor: func(deps Injections) (*ServiceC, error) {
				return &ServiceC{}, nil
			},
		}, false),
		FactoryProvider[*ServiceD](tokenD, Factory[*ServiceD]{
			Injects: Inject(tokenA),
			Constructor: func(deps Injections) (*ServiceD, error) {
				return &ServiceD{}, nil
			},
		}, false),
	)

	c := NewContainer()
	c.AddModules(mod)
	err := c.Run()
	if err == nil {
		t.Fatal("expected circular dependency error for A->B->C->D->A")
	}
	if !strings.Contains(err.Error(), "circular") {
		t.Fatalf("expected 'circular' in error, got: %v", err)
	}
}

func TestCircularProviderDependency_PartialCycleInChain(t *testing.T) {
	tokenA := "ServiceA"
	tokenB := "ServiceB"
	tokenC := "ServiceC"

	mod := NewModule("app")
	mod.Provide(
		FactoryProvider[*ServiceA](tokenA, Factory[*ServiceA]{
			Injects: Inject(tokenB),
			Constructor: func(deps Injections) (*ServiceA, error) {
				return &ServiceA{}, nil
			},
		}, false),
		FactoryProvider[*ServiceB](tokenB, Factory[*ServiceB]{
			Injects: Inject(tokenC),
			Constructor: func(deps Injections) (*ServiceB, error) {
				return &ServiceB{}, nil
			},
		}, false),
		FactoryProvider[*ServiceC](tokenC, Factory[*ServiceC]{
			Injects: Inject(tokenB),
			Constructor: func(deps Injections) (*ServiceC, error) {
				return &ServiceC{}, nil
			},
		}, false),
	)

	c := NewContainer()
	c.AddModules(mod)
	err := c.Run()
	if err == nil {
		t.Fatal("expected circular dependency error for B<->C (reached via A)")
	}
	if !strings.Contains(err.Error(), "circular") {
		t.Fatalf("expected 'circular' in error, got: %v", err)
	}
}

func TestCircularProviderDependency_DiamondNoCycle(t *testing.T) {
	tokenA := "ServiceA"
	tokenB := "ServiceB"
	tokenC := "ServiceC"
	tokenD := "ServiceD"

	mod := NewModule("app")
	mod.Provide(
		FactoryProvider[*ServiceD](tokenD, Factory[*ServiceD]{
			Constructor: func(deps Injections) (*ServiceD, error) {
				return &ServiceD{Name: "d"}, nil
			},
		}, false),
		FactoryProvider[*ServiceB](tokenB, Factory[*ServiceB]{
			Injects: Inject(tokenD),
			Constructor: func(deps Injections) (*ServiceB, error) {
				return &ServiceB{}, nil
			},
		}, false),
		FactoryProvider[*ServiceC](tokenC, Factory[*ServiceC]{
			Injects: Inject(tokenD),
			Constructor: func(deps Injections) (*ServiceC, error) {
				return &ServiceC{}, nil
			},
		}, false),
		FactoryProvider[*ServiceA](tokenA, Factory[*ServiceA]{
			Injects: Inject(tokenB, tokenC),
			Constructor: func(deps Injections) (*ServiceA, error) {
				return &ServiceA{}, nil
			},
		}, false),
	)

	c := NewContainer()
	c.AddModules(mod)
	mustRun(t, c)
}

func TestCircularProviderDependency_DiamondWithCycle(t *testing.T) {
	tokenA := "ServiceA"
	tokenB := "ServiceB"
	tokenC := "ServiceC"
	tokenD := "ServiceD"

	mod := NewModule("app")
	mod.Provide(
		FactoryProvider[*ServiceA](tokenA, Factory[*ServiceA]{
			Injects: Inject(tokenB, tokenC),
			Constructor: func(deps Injections) (*ServiceA, error) {
				return &ServiceA{}, nil
			},
		}, false),
		FactoryProvider[*ServiceB](tokenB, Factory[*ServiceB]{
			Injects: Inject(tokenD),
			Constructor: func(deps Injections) (*ServiceB, error) {
				return &ServiceB{}, nil
			},
		}, false),
		FactoryProvider[*ServiceC](tokenC, Factory[*ServiceC]{
			Injects: Inject(tokenD),
			Constructor: func(deps Injections) (*ServiceC, error) {
				return &ServiceC{}, nil
			},
		}, false),
		FactoryProvider[*ServiceD](tokenD, Factory[*ServiceD]{
			Injects: Inject(tokenA),
			Constructor: func(deps Injections) (*ServiceD, error) {
				return &ServiceD{}, nil
			},
		}, false),
	)

	c := NewContainer()
	c.AddModules(mod)
	err := c.Run()
	if err == nil {
		t.Fatal("expected circular dependency error for diamond with back-edge D->A")
	}
	if !strings.Contains(err.Error(), "circular") {
		t.Fatalf("expected 'circular' in error, got: %v", err)
	}
}

func TestCircularProviderDependency_CrossModule(t *testing.T) {
	tokenA := "ServiceA"
	tokenB := "ServiceB"

	modX := NewModule("X")
	modY := NewModule("Y")

	modX.Import(modY)
	modY.Import(modX)

	modX.Provide(FactoryProvider[*ServiceA](tokenA, Factory[*ServiceA]{
		Injects: Inject(tokenB),
		Constructor: func(deps Injections) (*ServiceA, error) {
			return &ServiceA{}, nil
		},
	}, true))

	modY.Provide(FactoryProvider[*ServiceB](tokenB, Factory[*ServiceB]{
		Injects: Inject(tokenA),
		Constructor: func(deps Injections) (*ServiceB, error) {
			return &ServiceB{}, nil
		},
	}, true))

	c := NewContainer()
	c.AddModules(modX, modY)
	err := c.Run()
	if err == nil {
		t.Fatal("expected circular dependency error for cross-module provider cycle")
	}
	if !strings.Contains(err.Error(), "circular") {
		t.Fatalf("expected 'circular' in error, got: %v", err)
	}
}

func TestCircularProviderDependency_GlobalProviderCycle(t *testing.T) {
	tokenA := "ServiceA"
	tokenB := "ServiceB"

	global := NewModule("global")
	global.Global()
	global.Provide(
		FactoryProvider[*ServiceA](tokenA, Factory[*ServiceA]{
			Injects: Inject(tokenB),
			Constructor: func(deps Injections) (*ServiceA, error) {
				return &ServiceA{}, nil
			},
		}, true),
		FactoryProvider[*ServiceB](tokenB, Factory[*ServiceB]{
			Injects: Inject(tokenA),
			Constructor: func(deps Injections) (*ServiceB, error) {
				return &ServiceB{}, nil
			},
		}, true),
	)

	c := NewContainer()
	c.AddModules(global)
	err := c.Run()
	if err == nil {
		t.Fatal("expected circular dependency error within global module providers")
	}
	if !strings.Contains(err.Error(), "circular") {
		t.Fatalf("expected 'circular' in error, got: %v", err)
	}
}

func TestCircularProviderDependency_ErrorContainsCyclePath(t *testing.T) {
	tokenA := "ServiceA"
	tokenB := "ServiceB"

	mod := NewModule("app")
	mod.Provide(
		FactoryProvider[*ServiceA](tokenA, Factory[*ServiceA]{
			Injects: Inject(tokenB),
			Constructor: func(deps Injections) (*ServiceA, error) {
				return &ServiceA{}, nil
			},
		}, false),
		FactoryProvider[*ServiceB](tokenB, Factory[*ServiceB]{
			Injects: Inject(tokenA),
			Constructor: func(deps Injections) (*ServiceB, error) {
				return &ServiceB{}, nil
			},
		}, false),
	)

	c := NewContainer()
	c.AddModules(mod)
	err := c.Run()
	if err == nil {
		t.Fatal("expected error")
	}
	errMsg := err.Error()
	if !strings.Contains(errMsg, tokenA) || !strings.Contains(errMsg, tokenB) {
		t.Fatalf("error should contain both %s and %s in the cycle path, got: %v", tokenA, tokenB, errMsg)
	}
}

func TestCircularProviderDependency_ErrorExposesCycleTokens(t *testing.T) {
	tokenA := "ServiceA"
	tokenB := "ServiceB"

	mod := NewModule("app")
	mod.Provide(
		FactoryProvider[*ServiceA](tokenA, Factory[*ServiceA]{
			Injects: Inject(tokenB),
			Constructor: func(deps Injections) (*ServiceA, error) {
				return &ServiceA{}, nil
			},
		}, false),
		FactoryProvider[*ServiceB](tokenB, Factory[*ServiceB]{
			Injects: Inject(tokenA),
			Constructor: func(deps Injections) (*ServiceB, error) {
				return &ServiceB{}, nil
			},
		}, false),
	)

	c := NewContainer()
	c.AddModules(mod)
	err := c.Run()
	if err == nil {
		t.Fatal("expected error")
	}

	var cycleErr *CircularInjectionError
	if !errors.As(err, &cycleErr) {
		t.Fatalf("expected CircularInjectionError, got %T", err)
	}
	if len(cycleErr.Tokens) == 0 {
		t.Fatal("expected cycle tokens to be exposed")
	}
}

func TestCircularProviderDependency_CycleAmidHealthyProviders(t *testing.T) {
	tokenA := "ServiceA"
	tokenB := "ServiceB"

	mod := NewModule("app")
	mod.Provide(
		ValueProvider[*Logger]("", &Logger{Prefix: "ok"}, false),
		ValueProvider[*Database]("", &Database{DSN: "pg"}, false),
		FactoryProvider[*ServiceA](tokenA, Factory[*ServiceA]{
			Injects: Inject(tokenB),
			Constructor: func(deps Injections) (*ServiceA, error) {
				return &ServiceA{}, nil
			},
		}, false),
		FactoryProvider[*ServiceB](tokenB, Factory[*ServiceB]{
			Injects: Inject(tokenA),
			Constructor: func(deps Injections) (*ServiceB, error) {
				return &ServiceB{}, nil
			},
		}, false),
	)

	c := NewContainer()
	c.AddModules(mod)
	err := c.Run()
	if err == nil {
		t.Fatal("expected circular dependency error even with healthy providers present")
	}
	if !strings.Contains(err.Error(), "circular") {
		t.Fatalf("expected 'circular' in error, got: %v", err)
	}
}

type Counter struct{ N int }

type ReaderA struct{ Counter *Counter }
type ReaderB struct{ Counter *Counter }

func TestSingleton_SharedIdentityAcrossConsumers(t *testing.T) {
	counterToken := "Counter"

	var gotA, gotB *Counter

	mod := NewModule("app")
	mod.Provide(
		FactoryProvider[*Counter](counterToken, Factory[*Counter]{
			ValueScope: Singleton,
			Constructor: func(deps Injections) (*Counter, error) {
				return &Counter{N: 0}, nil
			},
		}, false),
		FactoryProvider[*ReaderA]("", Factory[*ReaderA]{
			Injects: Inject(counterToken),
			Constructor: func(deps Injections) (*ReaderA, error) {
				c, err := Resolve[*Counter](counterToken, deps)
				if err != nil {
					return nil, err
				}
				gotA = c
				return &ReaderA{Counter: c}, nil
			},
		}, false),
		FactoryProvider[*ReaderB]("", Factory[*ReaderB]{
			Injects: Inject(counterToken),
			Constructor: func(deps Injections) (*ReaderB, error) {
				c, err := Resolve[*Counter](counterToken, deps)
				if err != nil {
					return nil, err
				}
				gotB = c
				return &ReaderB{Counter: c}, nil
			},
		}, false),
	)

	c := NewContainer()
	c.AddModules(mod)
	mustRun(t, c)

	if gotA == nil || gotB == nil {
		t.Fatal("both consumers should have received a Counter")
	}
	if gotA != gotB {
		t.Fatal("Singleton: both consumers should receive the same pointer")
	}
}

func TestSingleton_StateMutationVisibleAcrossConsumers(t *testing.T) {
	counterToken := "Counter"

	var gotA, gotB *Counter

	mod := NewModule("app")
	mod.Provide(
		FactoryProvider[*Counter](counterToken, Factory[*Counter]{
			ValueScope: Singleton,
			Constructor: func(deps Injections) (*Counter, error) {
				return &Counter{N: 10}, nil
			},
		}, false),
		FactoryProvider[*ReaderA]("", Factory[*ReaderA]{
			Injects: Inject(counterToken),
			Constructor: func(deps Injections) (*ReaderA, error) {
				c, _ := Resolve[*Counter](counterToken, deps)
				gotA = c
				return &ReaderA{Counter: c}, nil
			},
		}, false),
		FactoryProvider[*ReaderB]("", Factory[*ReaderB]{
			Injects: Inject(counterToken),
			Constructor: func(deps Injections) (*ReaderB, error) {
				c, _ := Resolve[*Counter](counterToken, deps)
				gotB = c
				return &ReaderB{Counter: c}, nil
			},
		}, false),
	)

	c := NewContainer()
	c.AddModules(mod)
	mustRun(t, c)

	gotA.N = 42
	if gotB.N != 42 {
		t.Fatalf("Singleton state mutation should be visible: expected 42, got %d", gotB.N)
	}
}

func TestPrototype_DifferentInstancePerConsumer(t *testing.T) {
	counterToken := "Counter"

	var gotA, gotB *Counter

	mod := NewModule("app")
	mod.Provide(
		FactoryProvider[*Counter](counterToken, Factory[*Counter]{
			ValueScope: Prototype,
			Constructor: func(deps Injections) (*Counter, error) {
				return &Counter{N: 0}, nil
			},
		}, false),
		FactoryProvider[*ReaderA]("", Factory[*ReaderA]{
			Injects: Inject(counterToken),
			Constructor: func(deps Injections) (*ReaderA, error) {
				c, _ := Resolve[*Counter](counterToken, deps)
				gotA = c
				return &ReaderA{Counter: c}, nil
			},
		}, false),
		FactoryProvider[*ReaderB]("", Factory[*ReaderB]{
			Injects: Inject(counterToken),
			Constructor: func(deps Injections) (*ReaderB, error) {
				c, _ := Resolve[*Counter](counterToken, deps)
				gotB = c
				return &ReaderB{Counter: c}, nil
			},
		}, false),
	)

	c := NewContainer()
	c.AddModules(mod)
	mustRun(t, c)

	if gotA == nil || gotB == nil {
		t.Fatal("both consumers should have received a Counter")
	}
	if gotA == gotB {
		t.Fatal("Prototype: each consumer should receive a different pointer")
	}
}

func TestPrototype_MutationIsolatedBetweenConsumers(t *testing.T) {
	counterToken := "Counter"

	var gotA, gotB *Counter

	mod := NewModule("app")
	mod.Provide(
		FactoryProvider[*Counter](counterToken, Factory[*Counter]{
			ValueScope: Prototype,
			Constructor: func(deps Injections) (*Counter, error) {
				return &Counter{N: 0}, nil
			},
		}, false),
		FactoryProvider[*ReaderA]("", Factory[*ReaderA]{
			Injects: Inject(counterToken),
			Constructor: func(deps Injections) (*ReaderA, error) {
				c, _ := Resolve[*Counter](counterToken, deps)
				gotA = c
				return &ReaderA{Counter: c}, nil
			},
		}, false),
		FactoryProvider[*ReaderB]("", Factory[*ReaderB]{
			Injects: Inject(counterToken),
			Constructor: func(deps Injections) (*ReaderB, error) {
				c, _ := Resolve[*Counter](counterToken, deps)
				gotB = c
				return &ReaderB{Counter: c}, nil
			},
		}, false),
	)

	c := NewContainer()
	c.AddModules(mod)
	mustRun(t, c)

	gotA.N = 99
	if gotB.N != 0 {
		t.Fatalf("Prototype mutation should be isolated: expected 0, got %d", gotB.N)
	}
}

func TestSingleton_ValueProviderSharedAcrossConsumers(t *testing.T) {
	counter := &Counter{N: 5}
	counterToken := "Counter"

	var gotA, gotB *Counter

	mod := NewModule("app")
	mod.Provide(
		ValueProvider[*Counter](counterToken, counter, false),
		FactoryProvider[*ReaderA]("", Factory[*ReaderA]{
			Injects: Inject(counterToken),
			Constructor: func(deps Injections) (*ReaderA, error) {
				c, _ := Resolve[*Counter](counterToken, deps)
				gotA = c
				return &ReaderA{Counter: c}, nil
			},
		}, false),
		FactoryProvider[*ReaderB]("", Factory[*ReaderB]{
			Injects: Inject(counterToken),
			Constructor: func(deps Injections) (*ReaderB, error) {
				c, _ := Resolve[*Counter](counterToken, deps)
				gotB = c
				return &ReaderB{Counter: c}, nil
			},
		}, false),
	)

	c := NewContainer()
	c.AddModules(mod)
	mustRun(t, c)

	if gotA != counter || gotB != counter {
		t.Fatal("ValueProvider should deliver the exact same pointer to all consumers")
	}
	gotA.N = 100
	if gotB.N != 100 {
		t.Fatalf("ValueProvider state should be shared: expected 100, got %d", gotB.N)
	}
}

func TestSingleton_CrossModuleSharedState(t *testing.T) {
	counterToken := "Counter"
	var gotA, gotB *Counter

	infra := NewModule("infra")
	infra.Provide(FactoryProvider[*Counter](counterToken, Factory[*Counter]{
		ValueScope: Singleton,
		Constructor: func(deps Injections) (*Counter, error) {
			return &Counter{N: 0}, nil
		},
	}, true))

	modA := NewModule("modA")
	modA.Import(infra)
	modA.Provide(FactoryProvider[*ReaderA]("", Factory[*ReaderA]{
		Injects: Inject(counterToken),
		Constructor: func(deps Injections) (*ReaderA, error) {
			c, _ := Resolve[*Counter](counterToken, deps)
			gotA = c
			return &ReaderA{Counter: c}, nil
		},
	}, false))

	modB := NewModule("modB")
	modB.Import(infra)
	modB.Provide(FactoryProvider[*ReaderB]("", Factory[*ReaderB]{
		Injects: Inject(counterToken),
		Constructor: func(deps Injections) (*ReaderB, error) {
			c, _ := Resolve[*Counter](counterToken, deps)
			gotB = c
			return &ReaderB{Counter: c}, nil
		},
	}, false))

	c := NewContainer()
	c.AddModules(infra, modA, modB)
	mustRun(t, c)

	if gotA == nil || gotB == nil {
		t.Fatal("both cross-module consumers should have received a Counter")
	}
	if gotA != gotB {
		t.Fatal("Singleton should be shared across modules")
	}
	gotA.N = 77
	if gotB.N != 77 {
		t.Fatalf("cross-module Singleton mutation should propagate: expected 77, got %d", gotB.N)
	}
}

type Registry struct{ Entries []string }
type LookupTable struct{ Data map[string]int }
type SliceConsumer struct{ Items []string }
type MapConsumer struct{ Table map[string]int }

func TestValueProvider_InjectSlice(t *testing.T) {
	items := []string{"alpha", "beta", "gamma"}
	sliceToken := "StringSlice"

	var received []string

	mod := NewModule("app")
	mod.Provide(
		ValueProvider[[]string](sliceToken, items, false),
		FactoryProvider[*SliceConsumer]("", Factory[*SliceConsumer]{
			Injects: Inject(sliceToken),
			Constructor: func(deps Injections) (*SliceConsumer, error) {
				s, err := Resolve[[]string](sliceToken, deps)
				if err != nil {
					return nil, err
				}
				received = s
				return &SliceConsumer{Items: s}, nil
			},
		}, false),
	)

	c := NewContainer()
	c.AddModules(mod)
	mustRun(t, c)

	if len(received) != 3 {
		t.Fatalf("expected 3 items, got %d", len(received))
	}
	if received[0] != "alpha" || received[1] != "beta" || received[2] != "gamma" {
		t.Fatalf("unexpected slice contents: %v", received)
	}
}

func TestValueProvider_InjectMap(t *testing.T) {
	table := map[string]int{"x": 1, "y": 2, "z": 3}
	mapToken := "IntMap"

	var received map[string]int

	mod := NewModule("app")
	mod.Provide(
		ValueProvider[map[string]int](mapToken, table, false),
		FactoryProvider[*MapConsumer]("", Factory[*MapConsumer]{
			Injects: Inject(mapToken),
			Constructor: func(deps Injections) (*MapConsumer, error) {
				m, err := Resolve[map[string]int](mapToken, deps)
				if err != nil {
					return nil, err
				}
				received = m
				return &MapConsumer{Table: m}, nil
			},
		}, false),
	)

	c := NewContainer()
	c.AddModules(mod)
	mustRun(t, c)

	if len(received) != 3 {
		t.Fatalf("expected 3 entries, got %d", len(received))
	}
	if received["x"] != 1 || received["y"] != 2 || received["z"] != 3 {
		t.Fatalf("unexpected map contents: %v", received)
	}
}

func TestFactoryProvider_InjectSlice(t *testing.T) {
	sliceToken := "StringSlice"

	var received []string

	mod := NewModule("app")
	mod.Provide(
		FactoryProvider[*[]string](sliceToken, Factory[*[]string]{
			ValueScope: Singleton,
			Constructor: func(deps Injections) (*[]string, error) {
				s := []string{"one", "two", "three"}
				return &s, nil
			},
		}, false),
		FactoryProvider[*SliceConsumer]("", Factory[*SliceConsumer]{
			Injects: Inject(sliceToken),
			Constructor: func(deps Injections) (*SliceConsumer, error) {
				s, err := Resolve[*[]string](sliceToken, deps)
				if err != nil {
					return nil, err
				}
				received = *s
				return &SliceConsumer{Items: *s}, nil
			},
		}, false),
	)

	c := NewContainer()
	c.AddModules(mod)
	mustRun(t, c)

	if len(received) != 3 {
		t.Fatalf("expected 3 items, got %d", len(received))
	}
	if received[0] != "one" || received[2] != "three" {
		t.Fatalf("unexpected slice contents: %v", received)
	}
}

func TestFactoryProvider_InjectMap(t *testing.T) {
	mapToken := "IntMap"

	var received map[string]int

	mod := NewModule("app")
	mod.Provide(
		FactoryProvider[*map[string]int](mapToken, Factory[*map[string]int]{
			ValueScope: Singleton,
			Constructor: func(deps Injections) (*map[string]int, error) {
				m := map[string]int{"a": 10, "b": 20}
				return &m, nil
			},
		}, false),
		FactoryProvider[*MapConsumer]("", Factory[*MapConsumer]{
			Injects: Inject(mapToken),
			Constructor: func(deps Injections) (*MapConsumer, error) {
				m, err := Resolve[*map[string]int](mapToken, deps)
				if err != nil {
					return nil, err
				}
				received = *m
				return &MapConsumer{Table: *m}, nil
			},
		}, false),
	)

	c := NewContainer()
	c.AddModules(mod)
	mustRun(t, c)

	if len(received) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(received))
	}
	if received["a"] != 10 || received["b"] != 20 {
		t.Fatalf("unexpected map contents: %v", received)
	}
}

func TestValueProvider_SharedSliceStateBetweenConsumers(t *testing.T) {
	items := []string{"initial"}
	sliceToken := "SharedSlice"

	var gotA, gotB []string

	mod := NewModule("app")
	mod.Provide(
		ValueProvider[[]string](sliceToken, items, false),
		FactoryProvider[*ReaderA]("SliceReaderA", Factory[*ReaderA]{
			Injects: Inject(sliceToken),
			Constructor: func(deps Injections) (*ReaderA, error) {
				s, _ := Resolve[[]string](sliceToken, deps)
				gotA = s
				return &ReaderA{}, nil
			},
		}, false),
		FactoryProvider[*ReaderB]("SliceReaderB", Factory[*ReaderB]{
			Injects: Inject(sliceToken),
			Constructor: func(deps Injections) (*ReaderB, error) {
				s, _ := Resolve[[]string](sliceToken, deps)
				gotB = s
				return &ReaderB{}, nil
			},
		}, false),
	)

	c := NewContainer()
	c.AddModules(mod)
	mustRun(t, c)

	gotA[0] = "mutated"
	if gotB[0] != "mutated" {
		t.Fatalf("slice state should be shared via ValueProvider: expected 'mutated', got %q", gotB[0])
	}
}

func TestValueProvider_SharedMapStateBetweenConsumers(t *testing.T) {
	table := map[string]int{"key": 1}
	mapToken := "SharedMap"

	var gotA, gotB map[string]int

	mod := NewModule("app")
	mod.Provide(
		ValueProvider[map[string]int](mapToken, table, false),
		FactoryProvider[*ReaderA]("MapReaderA", Factory[*ReaderA]{
			Injects: Inject(mapToken),
			Constructor: func(deps Injections) (*ReaderA, error) {
				m, _ := Resolve[map[string]int](mapToken, deps)
				gotA = m
				return &ReaderA{}, nil
			},
		}, false),
		FactoryProvider[*ReaderB]("MapReaderB", Factory[*ReaderB]{
			Injects: Inject(mapToken),
			Constructor: func(deps Injections) (*ReaderB, error) {
				m, _ := Resolve[map[string]int](mapToken, deps)
				gotB = m
				return &ReaderB{}, nil
			},
		}, false),
	)

	c := NewContainer()
	c.AddModules(mod)
	mustRun(t, c)

	gotA["key"] = 999
	gotA["new"] = 42
	if gotB["key"] != 999 {
		t.Fatalf("map mutation should be visible: expected 999, got %d", gotB["key"])
	}
	if gotB["new"] != 42 {
		t.Fatalf("map insertion should be visible: expected 42, got %d", gotB["new"])
	}
}

func TestSingleton_FactorySliceSharedState(t *testing.T) {
	sliceToken := "FactorySlice"

	var gotA, gotB *[]string

	mod := NewModule("app")
	mod.Provide(
		FactoryProvider[*[]string](sliceToken, Factory[*[]string]{
			ValueScope: Singleton,
			Constructor: func(deps Injections) (*[]string, error) {
				s := []string{"hello"}
				return &s, nil
			},
		}, false),
		FactoryProvider[*ReaderA]("FSliceReaderA", Factory[*ReaderA]{
			Injects: Inject(sliceToken),
			Constructor: func(deps Injections) (*ReaderA, error) {
				s, _ := Resolve[*[]string](sliceToken, deps)
				gotA = s
				return &ReaderA{}, nil
			},
		}, false),
		FactoryProvider[*ReaderB]("FSliceReaderB", Factory[*ReaderB]{
			Injects: Inject(sliceToken),
			Constructor: func(deps Injections) (*ReaderB, error) {
				s, _ := Resolve[*[]string](sliceToken, deps)
				gotB = s
				return &ReaderB{}, nil
			},
		}, false),
	)

	c := NewContainer()
	c.AddModules(mod)
	mustRun(t, c)

	if gotA != gotB {
		t.Fatal("Singleton FactoryProvider should return same slice pointer")
	}
	*gotA = append(*gotA, "world")
	if len(*gotB) != 2 || (*gotB)[1] != "world" {
		t.Fatalf("Singleton slice append should be visible: got %v", *gotB)
	}
}

func TestSingleton_FactoryMapSharedState(t *testing.T) {
	mapToken := "FactoryMap"

	var gotA, gotB *map[string]int

	mod := NewModule("app")
	mod.Provide(
		FactoryProvider[*map[string]int](mapToken, Factory[*map[string]int]{
			ValueScope: Singleton,
			Constructor: func(deps Injections) (*map[string]int, error) {
				m := map[string]int{"init": 1}
				return &m, nil
			},
		}, false),
		FactoryProvider[*ReaderA]("FMapReaderA", Factory[*ReaderA]{
			Injects: Inject(mapToken),
			Constructor: func(deps Injections) (*ReaderA, error) {
				m, _ := Resolve[*map[string]int](mapToken, deps)
				gotA = m
				return &ReaderA{}, nil
			},
		}, false),
		FactoryProvider[*ReaderB]("FMapReaderB", Factory[*ReaderB]{
			Injects: Inject(mapToken),
			Constructor: func(deps Injections) (*ReaderB, error) {
				m, _ := Resolve[*map[string]int](mapToken, deps)
				gotB = m
				return &ReaderB{}, nil
			},
		}, false),
	)

	c := NewContainer()
	c.AddModules(mod)
	mustRun(t, c)

	if gotA != gotB {
		t.Fatal("Singleton FactoryProvider should return same map pointer")
	}
	(*gotA)["added"] = 55
	if (*gotB)["added"] != 55 {
		t.Fatalf("Singleton map mutation should be visible: got %v", *gotB)
	}
}

func TestPrototype_FactorySliceIsolated(t *testing.T) {
	sliceToken := "ProtoSlice"

	var gotA, gotB *[]string

	mod := NewModule("app")
	mod.Provide(
		FactoryProvider[*[]string](sliceToken, Factory[*[]string]{
			ValueScope: Prototype,
			Constructor: func(deps Injections) (*[]string, error) {
				s := []string{"base"}
				return &s, nil
			},
		}, false),
		FactoryProvider[*ReaderA]("PSliceReaderA", Factory[*ReaderA]{
			Injects: Inject(sliceToken),
			Constructor: func(deps Injections) (*ReaderA, error) {
				s, _ := Resolve[*[]string](sliceToken, deps)
				gotA = s
				return &ReaderA{}, nil
			},
		}, false),
		FactoryProvider[*ReaderB]("PSliceReaderB", Factory[*ReaderB]{
			Injects: Inject(sliceToken),
			Constructor: func(deps Injections) (*ReaderB, error) {
				s, _ := Resolve[*[]string](sliceToken, deps)
				gotB = s
				return &ReaderB{}, nil
			},
		}, false),
	)

	c := NewContainer()
	c.AddModules(mod)
	mustRun(t, c)

	if gotA == gotB {
		t.Fatal("Prototype FactoryProvider should return different slice pointers")
	}
	*gotA = append(*gotA, "only-in-A")
	if len(*gotB) != 1 {
		t.Fatalf("Prototype slice mutation should be isolated: got %v", *gotB)
	}
}

func TestPrototype_FactoryMapIsolated(t *testing.T) {
	mapToken := "ProtoMap"

	var gotA, gotB *map[string]int

	mod := NewModule("app")
	mod.Provide(
		FactoryProvider[*map[string]int](mapToken, Factory[*map[string]int]{
			ValueScope: Prototype,
			Constructor: func(deps Injections) (*map[string]int, error) {
				m := map[string]int{"base": 1}
				return &m, nil
			},
		}, false),
		FactoryProvider[*ReaderA]("PMapReaderA", Factory[*ReaderA]{
			Injects: Inject(mapToken),
			Constructor: func(deps Injections) (*ReaderA, error) {
				m, _ := Resolve[*map[string]int](mapToken, deps)
				gotA = m
				return &ReaderA{}, nil
			},
		}, false),
		FactoryProvider[*ReaderB]("PMapReaderB", Factory[*ReaderB]{
			Injects: Inject(mapToken),
			Constructor: func(deps Injections) (*ReaderB, error) {
				m, _ := Resolve[*map[string]int](mapToken, deps)
				gotB = m
				return &ReaderB{}, nil
			},
		}, false),
	)

	c := NewContainer()
	c.AddModules(mod)
	mustRun(t, c)

	if gotA == gotB {
		t.Fatal("Prototype FactoryProvider should return different map pointers")
	}
	(*gotA)["only-in-A"] = 99
	if _, exists := (*gotB)["only-in-A"]; exists {
		t.Fatal("Prototype map mutation should be isolated")
	}
}

// ---------------------------------------------------------------------------
// Resolve helper
// ---------------------------------------------------------------------------

func TestResolve_HappyPath(t *testing.T) {
	inj := &Injection{Token: "Logger", Instance: &Logger{Prefix: "ok"}}
	log, err := Resolve[*Logger]("Logger", Injections{inj})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if log.Prefix != "ok" {
		t.Fatalf("expected prefix ok, got %s", log.Prefix)
	}
}

func TestResolve_WrongType(t *testing.T) {
	inj := &Injection{Token: "Logger", Instance: "not-a-logger"}
	_, err := Resolve[*Logger]("Logger", Injections{inj})
	if err == nil {
		t.Fatal("expected error for wrong type")
	}
}

func TestResolve_NilInjectionReturnsError(t *testing.T) {
	_, err := Resolve[*Logger]("Logger", Injections{nil})
	if err == nil {
		t.Fatal("expected error for nil injection")
	}
}

func TestResolve_NilInstanceWrongTypeReturnsError(t *testing.T) {
	inj := &Injection{Token: "Logger", Instance: nil}
	_, err := Resolve[*Logger]("Logger", Injections{inj})
	if err == nil {
		t.Fatal("expected error for nil instance")
	}
}

func TestResolve_Interface(t *testing.T) {
	var w strings.Builder
	w.WriteString("hello")
	inj := &Injection{Token: "Builder", Instance: &w}
	got, err := Resolve[*strings.Builder]("Builder", Injections{inj})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.String() != "hello" {
		t.Fatalf("expected hello, got %s", got.String())
	}
}

func TestResolve_FromInjections(t *testing.T) {
	injections := Injections{
		{Token: "Logger", Instance: &Logger{Prefix: "ok"}},
	}

	log, err := Resolve[*Logger]("Logger", injections)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if log.Prefix != "ok" {
		t.Fatalf("expected prefix ok, got %s", log.Prefix)
	}
}

func TestInjections_Resolve(t *testing.T) {
	want := &Logger{}
	injections := Injections{{Token: "Logger", Instance: want}}

	got, err := injections.Resolve("Logger")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != want {
		t.Fatal("expected Resolve to return the matching instance")
	}
}

func TestMustResolve_HappyPath(t *testing.T) {
	injections := Injections{
		{Token: "Logger", Instance: &Logger{Prefix: "ok"}},
	}

	log := MustResolve[*Logger]("Logger", injections)
	if log.Prefix != "ok" {
		t.Fatalf("expected prefix ok, got %s", log.Prefix)
	}
}

func TestMustResolve_PanicsOnError(t *testing.T) {
	defer func() {
		r := recover()
		if r == nil {
			t.Fatal("expected panic for missing token")
		}
		if _, ok := r.(*DependencyError); !ok {
			t.Fatalf("expected DependencyError panic, got %T", r)
		}
	}()

	MustResolve[*Logger]("Logger", nil)
}

func TestInjections_MustResolve(t *testing.T) {
	want := &Logger{}
	injections := Injections{{Token: "Logger", Instance: want}}

	got := injections.MustResolve("Logger")
	if got != want {
		t.Fatal("expected MustResolve to return the matching instance")
	}
}

func TestInjections_MustResolvePanicsOnError(t *testing.T) {
	defer func() {
		r := recover()
		if r == nil {
			t.Fatal("expected panic for missing token")
		}
		if _, ok := r.(*DependencyError); !ok {
			t.Fatalf("expected DependencyError panic, got %T", r)
		}
	}()

	Injections(nil).MustResolve("Logger")
}

// ---------------------------------------------------------------------------
// Require helper
// ---------------------------------------------------------------------------

func TestRequire_AllPresent(t *testing.T) {
	injections := []*Injection{
		{Token: "Logger", Instance: &Logger{}},
		{Token: "Database", Instance: &Database{}},
	}
	// should not panic
	Require(injections, "Logger", "Database")
}

func TestRequire_MissingToken_Panics(t *testing.T) {
	injections := []*Injection{
		{Token: "Logger", Instance: &Logger{}},
	}
	defer func() {
		if r := recover(); r == nil {
			t.Fatal("expected panic for missing token")
		}
	}()
	Require(injections, "Logger", "Database")
}

func TestRequire_EmptyInjections_Panics(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Fatal("expected panic when injections are empty")
		}
	}()
	Require(nil, "Logger")
}

func TestRequire_NilInjection_PanicsWithDependencyError(t *testing.T) {
	defer func() {
		r := recover()
		if r == nil {
			t.Fatal("expected panic for nil injection")
		}
		if _, ok := r.(*DependencyError); !ok {
			t.Fatalf("expected DependencyError panic, got %T", r)
		}
	}()

	Require([]*Injection{nil}, "Logger")
}

func TestRequire_NoTokens_NoOp(t *testing.T) {
	// zero required tokens — must not panic even with empty injections
	Require(nil)
}

func TestInjections_Require(t *testing.T) {
	injections := Injections{
		{Token: "Logger", Instance: &Logger{}},
	}

	// should not panic
	injections.Require("Logger")
}

// ---------------------------------------------------------------------------
// CreateToken
// ---------------------------------------------------------------------------

func TestCreateToken_PointerToStruct(t *testing.T) {
	token := CreateToken[*Logger]()
	if token != "Logger" {
		t.Fatalf("expected Logger, got %s", token)
	}
}

func TestCreateToken_BareStruct(t *testing.T) {
	token := CreateToken[Logger]()
	if token != "Logger" {
		t.Fatalf("expected Logger, got %s", token)
	}
}

func TestCreateToken_InvalidType_Panics(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Fatal("expected panic for non-struct pointer type")
		}
	}()
	CreateToken[*string]()
}

func TestCreateToken_NonStructValue_Panics(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Fatal("expected panic for non-struct value type")
		}
	}()
	CreateToken[int]()
}

func TestCreateToken_InterfaceType_Panics(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Fatal("expected panic for interface type")
		}
	}()
	CreateToken[error]()
}

// ---------------------------------------------------------------------------
// Error propagation
// ---------------------------------------------------------------------------

func TestContainer_FactoryConstructorError_PropagatesOnRun(t *testing.T) {
	mod := NewModule("app")
	mod.Provide(FactoryProvider[*Logger]("", Factory[*Logger]{
		ValueScope: Singleton,
		Constructor: func(deps Injections) (*Logger, error) {
			return nil, &DependencyError{reason: "simulated construction failure"}
		},
	}, false))

	c := NewContainer()
	c.AddModules(mod)
	if err := c.Run(); err == nil {
		t.Fatal("expected error when factory constructor fails")
	}
}

func TestContainer_MissingDependency_ReturnsError(t *testing.T) {
	mod := NewModule("app")
	mod.Provide(FactoryProvider[*UserService]("", Factory[*UserService]{
		Injects: Inject("Logger"), // Logger is never provided
		Constructor: func(deps Injections) (*UserService, error) {
			return &UserService{}, nil
		},
	}, false))

	c := NewContainer()
	c.AddModules(mod)
	if err := c.Run(); err == nil {
		t.Fatal("expected error for missing dependency")
	}
}

func TestContainer_MissingDependency_ErrorMentionsMissingToken(t *testing.T) {
	missingToken := "NonExistentService"
	mod := NewModule("app")
	mod.Provide(FactoryProvider[*UserService]("", Factory[*UserService]{
		Injects: Inject(missingToken),
		Constructor: func(deps Injections) (*UserService, error) {
			return &UserService{}, nil
		},
	}, false))

	c := NewContainer()
	c.AddModules(mod)
	err := c.Run()
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), missingToken) {
		t.Fatalf("error should mention missing token %q, got: %v", missingToken, err)
	}
}

// ---------------------------------------------------------------------------
// Export flag: enforced by module.lookup.
// Only providers marked as exportable are visible to importing modules.
// Non-exported providers are hidden even from direct importers.
// ---------------------------------------------------------------------------

func TestModule_NonExportedProviderNotVisibleViaDirectImport(t *testing.T) {
	infra := NewModule("infra")
	// Logger is provided but NOT exported (false)
	infra.Provide(ValueProvider[*Logger]("", &Logger{Prefix: "private"}, false))

	app := NewModule("app")
	app.Import(infra)

	// module.lookup enforces exports — non-exported provider is NOT accessible
	found := app.lookup(CreateToken[Logger]())
	if found != nil {
		t.Fatal("non-exported provider should not be visible to importing module")
	}
}

func TestContainer_NonExportedDep_NotAccessibleWhenDirectlyImported(t *testing.T) {
	logToken := CreateToken[Logger]()

	infra := NewModule("infra")
	infra.Provide(ValueProvider[*Logger]("", &Logger{Prefix: "private"}, false /* not exported */))

	app := NewModule("app")
	app.Import(infra)
	app.Provide(FactoryProvider[*UserService]("", Factory[*UserService]{
		Injects: Inject(logToken),
		Constructor: func(deps Injections) (*UserService, error) {
			log, err := Resolve[*Logger](logToken, deps)
			if err != nil {
				return nil, err
			}
			return &UserService{Log: log}, nil
		},
	}, false))

	c := NewContainer()
	c.AddModules(infra, app)
	if err := c.Run(); err == nil {
		t.Fatal("expected dependency error: non-exported provider should not be visible to importing module")
	}
}

// ---------------------------------------------------------------------------
// Global modules
// ---------------------------------------------------------------------------

func TestContainer_MultipleGlobalModules_BothAccessible(t *testing.T) {
	logToken := CreateToken[Logger]()
	dbToken := CreateToken[Database]()

	gLog := NewModule("global-log").Global()
	gLog.Provide(ValueProvider[*Logger]("", &Logger{Prefix: "global"}, true))

	gDB := NewModule("global-db").Global()
	gDB.Provide(ValueProvider[*Database]("", &Database{DSN: "global-pg"}, true))

	var gotLog *Logger
	var gotDB *Database

	app := NewModule("app") // no explicit import — globals are visible container-wide
	app.Provide(FactoryProvider[*UserService]("", Factory[*UserService]{
		Injects: Inject(logToken, dbToken),
		Constructor: func(deps Injections) (*UserService, error) {
			log, err := Resolve[*Logger](logToken, deps)
			if err != nil {
				return nil, err
			}
			db, err := Resolve[*Database](dbToken, deps)
			if err != nil {
				return nil, err
			}
			gotLog = log
			gotDB = db
			return &UserService{Log: log, DB: db}, nil
		},
	}, false))

	c := NewContainer()
	c.AddModules(gLog, gDB, app)
	mustRun(t, c)

	if gotLog == nil || gotDB == nil {
		t.Fatal("app module should resolve providers from both global modules")
	}
	if gotLog.Prefix != "global" {
		t.Fatalf("expected prefix global, got %s", gotLog.Prefix)
	}
	if gotDB.DSN != "global-pg" {
		t.Fatalf("expected DSN global-pg, got %s", gotDB.DSN)
	}
}

func TestContainer_GlobalModuleCanResolveLaterGlobalModule(t *testing.T) {
	logToken := CreateToken[Logger]()
	dbToken := CreateToken[Database]()

	firstGlobal := NewModule("first-global").Global()
	var gotLog *Logger
	firstGlobal.Provide(FactoryProvider[*Database](dbToken, Factory[*Database]{
		Injects: Inject(logToken),
		Constructor: func(deps Injections) (*Database, error) {
			log, err := Resolve[*Logger](logToken, deps)
			if err != nil {
				return nil, err
			}
			gotLog = log
			return &Database{DSN: "from-first"}, nil
		},
	}, true))

	secondGlobal := NewModule("second-global").Global()
	secondGlobal.Provide(ValueProvider[*Logger]("", &Logger{Prefix: "later"}, true))

	c := NewContainer()
	c.AddModules(firstGlobal, secondGlobal)
	mustRun(t, c)

	if gotLog == nil {
		t.Fatal("earlier global module should resolve providers from later global modules")
	}
	if gotLog.Prefix != "later" {
		t.Fatalf("expected later global logger, got %s", gotLog.Prefix)
	}
}

func TestContainer_GlobalModule_OwnModuleBeatsGlobal(t *testing.T) {
	logToken := CreateToken[Logger]()

	global := NewModule("global").Global()
	global.Provide(ValueProvider[*Logger]("", &Logger{Prefix: "global"}, true))

	var resolved *Logger

	app := NewModule("app")
	// app provides its own Logger — should shadow global
	app.Provide(
		ValueProvider[*Logger]("", &Logger{Prefix: "local"}, false),
		FactoryProvider[*UserService]("", Factory[*UserService]{
			Injects: Inject(logToken),
			Constructor: func(deps Injections) (*UserService, error) {
				log, err := Resolve[*Logger](logToken, deps)
				if err != nil {
					return nil, err
				}
				resolved = log
				return &UserService{Log: log}, nil
			},
		}, false),
	)

	c := NewContainer()
	c.AddModules(global, app)
	mustRun(t, c)

	if resolved == nil {
		t.Fatal("UserService should have been constructed")
	}
	if resolved.Prefix != "local" {
		t.Fatalf("own module provider should shadow global: expected local, got %s", resolved.Prefix)
	}
}

type AppConfig struct {
	DSN      string
	LogLevel string
}

type DBPool struct {
	DSN string
}

type UserRepository struct {
	Pool   *DBPool
	Config *AppConfig
}

type UserController struct {
	Service *UserService
	Log     *Logger
}

// TestProductionLike_LayeredApp simulates a realistic layered app:
//
//	infra (global) → Config, Logger          — available container-wide, no import needed
//	services       → DBPool, UserRepository, UserService
//	api            → UserController          (imports services)
func TestProductionLike_LayeredApp(t *testing.T) {
	configToken := "AppConfig"
	poolToken := "DBPool"
	repoToken := "UserRepository"
	userSvcToken := "UserService"
	logToken := CreateToken[Logger]()

	// --- infrastructure (global) ---
	infraMod := NewModule("infra").Global()
	infraMod.Provide(
		ValueProvider[*AppConfig](configToken, &AppConfig{DSN: "postgres://prod", LogLevel: "info"}, true),
		ValueProvider[*Logger]("", &Logger{Prefix: "[app]"}, true),
	)

	// --- services module: DB pool + domain services ---
	// infra is global so its providers (Config, Logger) are visible here without import.
	servicesMod := NewModule("services")
	servicesMod.Provide(
		FactoryProvider[*DBPool](poolToken, Factory[*DBPool]{
			Injects:    Inject(configToken),
			ValueScope: Singleton,
			Constructor: func(deps Injections) (*DBPool, error) {
				cfg, err := Resolve[*AppConfig](configToken, deps)
				if err != nil {
					return nil, err
				}
				return &DBPool{DSN: cfg.DSN}, nil
			},
		}, true),
		FactoryProvider[*UserRepository](repoToken, Factory[*UserRepository]{
			Injects:    Inject(poolToken, configToken),
			ValueScope: Singleton,
			Constructor: func(deps Injections) (*UserRepository, error) {
				pool, err := Resolve[*DBPool](poolToken, deps)
				if err != nil {
					return nil, err
				}
				cfg, err := Resolve[*AppConfig](configToken, deps)
				if err != nil {
					return nil, err
				}
				return &UserRepository{Pool: pool, Config: cfg}, nil
			},
		}, true),
		FactoryProvider[*UserService](userSvcToken, Factory[*UserService]{
			Injects:    Inject(logToken, repoToken),
			ValueScope: Singleton,
			Constructor: func(deps Injections) (*UserService, error) {
				log, err := Resolve[*Logger](logToken, deps)
				if err != nil {
					return nil, err
				}
				repo, err := Resolve[*UserRepository](repoToken, deps)
				if err != nil {
					return nil, err
				}
				// repurpose UserService.DB to carry the resolved DSN for assertion
				return &UserService{Log: log, DB: &Database{DSN: repo.Pool.DSN}}, nil
			},
		}, true),
	)

	// --- API module ---
	var ctrl *UserController
	apiMod := NewModule("api")
	apiMod.Import(servicesMod) // Logger comes from global infra, no import needed
	apiMod.Provide(FactoryProvider[*UserController]("", Factory[*UserController]{
		Injects:    Inject(userSvcToken, logToken),
		ValueScope: Singleton,
		Constructor: func(deps Injections) (*UserController, error) {
			svc, err := Resolve[*UserService](userSvcToken, deps)
			if err != nil {
				return nil, err
			}
			log, err := Resolve[*Logger](logToken, deps)
			if err != nil {
				return nil, err
			}
			ctrl = &UserController{Service: svc, Log: log}
			return ctrl, nil
		},
	}, false))

	c := NewContainer()
	c.AddModules(infraMod, servicesMod, apiMod)
	mustRun(t, c)

	if ctrl == nil {
		t.Fatal("UserController should have been constructed")
	}
	if ctrl.Service == nil {
		t.Fatal("UserController.Service should be injected")
	}
	if ctrl.Service.DB.DSN != "postgres://prod" {
		t.Fatalf("expected DSN postgres://prod, got %s", ctrl.Service.DB.DSN)
	}
	if ctrl.Log.Prefix != "[app]" {
		t.Fatalf("expected log prefix [app], got %s", ctrl.Log.Prefix)
	}
}

func TestProductionLike_FactoryErrorBubblesUp(t *testing.T) {
	// Simulate a misconfigured connection string that causes DB init failure.
	configToken := "AppConfig"
	poolToken := "DBPool"

	infraMod := NewModule("infra").Global()
	infraMod.Provide(
		ValueProvider[*AppConfig](configToken, &AppConfig{DSN: ""}, true),
	)

	dbMod := NewModule("database") // infra is global — AppConfig visible without import
	dbMod.Provide(FactoryProvider[*DBPool](poolToken, Factory[*DBPool]{
		Injects:    Inject(configToken),
		ValueScope: Singleton,
		Constructor: func(deps Injections) (*DBPool, error) {
			cfg, _ := Resolve[*AppConfig](configToken, deps)
			if cfg.DSN == "" {
				return nil, &DependencyError{reason: "DSN must not be empty"}
			}
			return &DBPool{DSN: cfg.DSN}, nil
		},
	}, true))

	c := NewContainer()
	c.AddModules(infraMod, dbMod)
	if err := c.Run(); err == nil {
		t.Fatal("expected error when factory constructor rejects empty DSN")
	}
}
