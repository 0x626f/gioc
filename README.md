<div align="center">
    <pre style="background: none;">
   █████████  █████    ███████      █████████    
  ███░░░░░███░░███   ███░░░░░███   ███░░░░░███   
 ███     ░░░  ░███  ███     ░░███ ███     ░░░    
░███          ░███ ░███      ░███░███            
░███    █████ ░███ ░███      ░███░███            
░░███  ░░███  ░███ ░░███     ███ ░░███     ███   
 ░░█████████  █████ ░░░███████░   ░░█████████    
  ░░░░░░░░░  ░░░░░    ░░░░░░░      ░░░░░░░░░     
    </pre>
</div>

<div align="center">
    <h3>A lightweight, type-safe inversion-of-control container for Go.</h3>
    <h6>Currently under active development and breaking changes are possible</h6>
</div>

gioc organises dependencies into **modules**, resolves the full dependency graph at startup, and wires every provider in topological order — catching circular dependencies before your application runs.

```
go get github.com/0x626f/gioc
```

---

## Concepts

| Concept | Description |
|---|---|
| **Token** | A string that uniquely identifies a provider within a module. Derived automatically from the type name or set explicitly. |
| **Provider** | Describes how to create one dependency — either a pre-built value (`ValueProvider`) or a constructor function (`FactoryProvider`). |
| **Module** | Groups related providers. A module can import other modules to access their providers. |
| **Container** | Holds all modules, validates the graph, and instantiates everything on `Run`. |
| **Scope** | `Singleton` (one shared instance) or `Prototype` (new instance per consumer). |

---

## Quick start

```go
package main

import (
    "fmt"
    "github.com/0x626f/gioc"
)

type Config struct{ DSN string }
type DB     struct{ DSN string }
type App    struct{ DB *DB }

func main() {
    configToken := "Config"
    dbToken     := "DB"

    mod := gioc.NewModule("app")
    mod.Provide(
        gioc.ValueProvider[*Config](configToken, &Config{DSN: "postgres://localhost/mydb"}, false),

        gioc.FactoryProvider[*DB](dbToken, gioc.Factory[*DB]{
            Injects:    gioc.Inject(configToken),
            ValueScope: gioc.Singleton,
            Constructor: func(deps ...*gioc.Injectable) (*DB, error) {
                cfg, err := gioc.ResolveFrom[*Config](configToken, deps)
                if err != nil {
                    return nil, err
                }
                return &DB{DSN: cfg.DSN}, nil
            },
        }, false),

        gioc.FactoryProvider[*App]("", gioc.Factory[*App]{
            Injects:    gioc.Inject(dbToken),
            ValueScope: gioc.Singleton,
            Constructor: func(deps ...*gioc.Injectable) (*App, error) {
                db, err := gioc.ResolveFrom[*DB](dbToken, deps)
                if err != nil {
                    return nil, err
                }
                return &App{DB: db}, nil
            },
        }, false),
    )

    c := gioc.NewContainer()
    c.AddModules(mod)
    if err := c.Run(); err != nil {
        panic(err)
    }

    fmt.Println("container wired successfully")
}
```

---

## Providers

### ValueProvider

Wraps an already-constructed value. Always `Singleton` — every consumer receives the same pointer.

```go
gioc.ValueProvider[*Config]("", &Config{DSN: "..."}, true)
//                           ^     ^                  ^
//                           |     value              exportable
//                           token (empty = derived from type)
```

### FactoryProvider

Constructs the instance via a function. Supports both `Singleton` and `Prototype` scopes.

```go
gioc.FactoryProvider[*Service]("", gioc.Factory[*Service]{
    Injects:     gioc.Inject("Logger", "Database"),
    ValueScope:  gioc.Singleton,
    Constructor: func(deps ...*gioc.Injectable) (*Service, error) {
        log, _ := gioc.ResolveFrom[*Logger]("Logger", deps)
        db,  _ := gioc.ResolveFrom[*Database]("Database", deps)
        return &Service{Log: log, DB: db}, nil
    },
}, false)
```

### Token auto-derivation

When the token argument is an empty string, it is derived from the type name:

```go
gioc.CreateToken[*MyService]() // → "MyService"
gioc.CreateToken[MyService]()  // → "MyService"
```

---

## Modules

```go
infraMod := gioc.NewModule("infra")
infraMod.Provide(
    gioc.ValueProvider[*Logger]("", &Logger{}, true), // exported
)

appMod := gioc.NewModule("app")
appMod.Import(infraMod)   // providers of infraMod are visible here
appMod.Provide(
    gioc.FactoryProvider[*Service]("", gioc.Factory[*Service]{
        Injects: gioc.Inject(gioc.CreateToken[Logger]()),
        Constructor: func(deps ...*gioc.Injectable) (*Service, error) {
            log, _ := gioc.ResolveFrom[*Logger](gioc.CreateToken[Logger](), deps)
            return &Service{Log: log}, nil
        },
    }, false),
)
```

### Global modules

Mark a module as global to make its providers available to every other module in the container — no explicit `Import` call needed.

```go
sharedMod := gioc.NewModule("shared").Global()
sharedMod.Provide(gioc.ValueProvider[*Logger]("", &Logger{}, false))

appMod := gioc.NewModule("app") // no Import(sharedMod) required
appMod.Provide(gioc.FactoryProvider[*Service]("", gioc.Factory[*Service]{
    Injects: gioc.Inject(gioc.CreateToken[Logger]()),
    Constructor: func(deps ...*gioc.Injectable) (*Service, error) {
        log, _ := gioc.ResolveFrom[*Logger](gioc.CreateToken[Logger](), deps)
        return &Service{Log: log}, nil
    },
}, false))

c := gioc.NewContainer()
c.AddModules(sharedMod, appMod) // sharedMod's Logger is injected into appMod automatically
c.Run()
```

Global modules are initialised before regular modules, so their singletons are ready when the rest of the container starts up. If the same token is registered both locally and in a global module, the local provider takes priority.

### Chaining

`NewModule`, `Global`, `Import`, and `Provide` all return `*Module`, so they can be chained:

```go
gioc.NewModule("app").
    Import(infraMod).
    Provide(providerA, providerB)
```

---

## Resolving dependencies in constructors

### ResolveFrom — find by token in the deps slice

```go
Constructor: func(deps ...*gioc.Injectable) (*Service, error) {
    db, err := gioc.ResolveFrom[*Database]("Database", deps)
    if err != nil {
        return nil, err
    }
    return &Service{DB: db}, nil
},
```

### Resolve — unwrap a single Injectable

```go
db, err := gioc.Resolve[*Database](inj)
```

### Container.Resolve — fetch a provider after Run

Singleton providers return the instance created during `Run`; prototype providers create a fresh instance for each call.

```go
inj, err := c.Resolve("Database", appMod)
if err != nil {
    return err
}
db, err := gioc.Resolve[*Database](inj)
```

### Require — panic-guard at the top of a constructor

```go
Constructor: func(deps ...*gioc.Injectable) (*Service, error) {
    gioc.Require(deps, "Logger", "Database") // panics if either is missing
    ...
},
```

---

## Scopes

```go
// Singleton — one instance shared across all consumers
gioc.Factory[*Pool]{ValueScope: gioc.Singleton, Constructor: ...}

// Prototype — new instance created for every consumer
gioc.Factory[*Request]{ValueScope: gioc.Prototype, Constructor: ...}
```

---

## Error handling

`Run` returns typed errors that can be inspected:

```go
if err := c.Run(); err != nil {
    var cycleErr *gioc.CircularInjectionError
    var depErr   *gioc.DependencyError
    switch {
    case errors.As(err, &cycleErr):
        log.Fatalf("cycle detected: %v", cycleErr)
    case errors.As(err, &depErr):
        log.Fatalf("missing dependency: %v", depErr)
    }
}
```

| Error type | Cause |
|---|---|
| `CircularInjectionError` | A cycle was found in the module import graph or the provider dependency graph. |
| `DependencyError` | A required token is not registered, a constructor returned an error, or an `Injectable` could not be cast to the expected type. |

---

## Full wiring example

```go
type AppConfig struct{ DSN string }
type DBPool   struct{ DSN string }
type UserRepo struct{ Pool *DBPool }
type UserSvc  struct{ Repo *UserRepo }

const (
    tokenConfig  = "AppConfig"
    tokenPool    = "DBPool"
    tokenRepo    = "UserRepo"
    tokenSvc     = "UserSvc"
)

// infrastructure module — global, AppConfig is visible to all modules automatically
infraMod := gioc.NewModule("infra").Global()
infraMod.Provide(
    gioc.ValueProvider[*AppConfig](tokenConfig, &AppConfig{DSN: "postgres://prod"}, false),
)

// services module — no Import needed; AppConfig comes from the global infra module
svcMod := gioc.NewModule("services")
svcMod.Provide(
    gioc.FactoryProvider[*DBPool](tokenPool, gioc.Factory[*DBPool]{
        Injects:    gioc.Inject(tokenConfig),
        ValueScope: gioc.Singleton,
        Constructor: func(deps ...*gioc.Injectable) (*DBPool, error) {
            cfg, _ := gioc.ResolveFrom[*AppConfig](tokenConfig, deps)
            return &DBPool{DSN: cfg.DSN}, nil
        },
    }, true),
    gioc.FactoryProvider[*UserRepo](tokenRepo, gioc.Factory[*UserRepo]{
        Injects:    gioc.Inject(tokenPool),
        ValueScope: gioc.Singleton,
        Constructor: func(deps ...*gioc.Injectable) (*UserRepo, error) {
            pool, _ := gioc.ResolveFrom[*DBPool](tokenPool, deps)
            return &UserRepo{Pool: pool}, nil
        },
    }, true),
    gioc.FactoryProvider[*UserSvc](tokenSvc, gioc.Factory[*UserSvc]{
        Injects:    gioc.Inject(tokenRepo),
        ValueScope: gioc.Singleton,
        Constructor: func(deps ...*gioc.Injectable) (*UserSvc, error) {
            repo, _ := gioc.ResolveFrom[*UserRepo](tokenRepo, deps)
            return &UserSvc{Repo: repo}, nil
        },
    }, true),
)

c := gioc.NewContainer()
c.AddModules(infraMod, svcMod)
if err := c.Run(); err != nil {
    log.Fatal(err)
}
```
