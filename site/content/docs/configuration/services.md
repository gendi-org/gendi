---
title: Services
weight: 2
---

Services are objects constructed and managed by the container.

## Service Definition

```yaml
services:
  service_id:
    # Optional: Explicit type (inferred from constructor if omitted)
    type: "github.com/myapp.Service"

    # Constructor configuration
    constructor:
      # Function constructor
      func: "github.com/myapp.NewService"
      # OR method constructor
      method: "@other_service.CreateService"

      # Constructor arguments
      args:
        - "@dependency"           # Service reference
        - "%parameter%"           # Parameter reference
        - "!tagged:tag.name"      # Tagged services
        - "@.inner"               # Inner service (decorators only)
        - "literal string"        # Literal value
        - 123                     # Literal number
        - true                    # Literal bool

    # Lifecycle (default: shared=true)
    shared: true  # Singleton (cached)
    # shared: false creates new instance each time

    # Public API exposure
    public: true  # Generate public getter method

    # Participation in autoconfigured tags (default: true)
    autoconfigure: true

    # Service aliasing
    alias: "other_service"  # Alias to another service

    # Decoration
    decorates: "base_service"  # Decorate another service
    decoration_priority: 10    # Higher priority decorators wrap first

    # Tagging
    tags:
      - "tag.name"            # String shorthand, no attributes
      - name: "tag.name"      # Every field except 'name' is an attribute
        attribute1: "value1"
        priority: 100
```

A service ID must not end with `.inner` — that suffix is reserved for
decorator expansion.

## The `type` Field

`type` is optional. When omitted, the service type is the constructor's
return type. When present it is a contract: the inferred constructor type
must match it, or generation fails. It is not a conversion — declaring a
supertype does not widen the service.

## Constructor Signatures

A constructor — `func` or `method` — must return exactly one of:

1. `T`
2. `(T, error)`

`T` is any type the container can check statically: concrete types,
pointers, slices, maps, channels, and interfaces with methods.

- A constructor returning `any`/`interface{}`, or a named type whose
  underlying type is an empty interface, is rejected at generation time:
  such a type makes every assignment valid and leaves nothing to verify
- Generic constructors require explicit type arguments, so a bare type
  parameter never becomes a service type
- Argument count and types are validated against the signature

## Service Defaults (`_default`)

The reserved `_default` entry sets the default `shared`, `public`, and
`autoconfigure` flags for the services declared in the same file:

```yaml
services:
  _default:
    shared: true
    public: false
    autoconfigure: false

  logger:
    constructor:
      func: "$this.NewLogger"
    public: true      # explicit value wins over the default
```

- Applies per file — an imported config keeps its own defaults, and inherits
  none from the importing file
- Only `shared`, `public`, and `autoconfigure` are allowed; `type`,
  `constructor`, `alias`, `decorates`, `decoration_priority`, and `tags` in
  `_default` are configuration errors
- Without `_default`, the built-in defaults are `shared: true`,
  `public: false`, `autoconfigure: true`
- `_default` is not a service and gets no getter

## Service Lifecycle

**Shared Services (Singletons):**
```yaml
services:
  database:
    constructor:
      func: "db.New"
    shared: true  # Default
```

- Created once on first access
- Same instance returned on subsequent calls
- Construction is serialized by the container mutex (see below)
- Suitable for: databases, HTTP clients, loggers

**Non-Shared Services (Factories):**
```yaml
services:
  request_context:
    constructor:
      func: "ctx.New"
    shared: false
```

- New instance created on each access
- No caching; the instance is never stored on the container
- Suitable for: request handlers, temporary objects

The container has exactly one mutex, and only the public getters take it —
for both lifecycles. It guards the shared-instance fields of the whole graph,
not just the service being fetched, so one public getter call builds its
entire subgraph under a single lock.

The internal getters services use to reach each other are lock-free,
including those of shared services: they run only inside a public getter that
already holds the mutex. The mutex is a plain `sync.Mutex` and is not
reentrant, so a constructor must not call a public getter of its own
container — that deadlocks. Take dependencies as constructor arguments
instead.

## Service Aliases

Create multiple names for the same service:

```yaml
services:
  logger.impl:
    constructor:
      func: "log.New"

  # String shorthand: the whole service definition is "@target"
  logger: "@logger.impl"

  # Expanded form, needed when the alias also sets public or type
  log:
    alias: "logger.impl"   # the @ prefix is optional here
    public: true
```

Aliases always inherit the target service lifecycle. Do not set `shared` on
an alias; explicit `shared: true` and `shared: false` are both configuration
errors.

## Public Services

Generate public getter methods for services:

```yaml
services:
  user_repo:
    constructor:
      func: "repo.NewUserRepository"
    public: true
```

Generated methods:
```go
func (c *Container) GetUserRepo() (*UserRepository, error)
func (c *Container) MustUserRepo() *UserRepository
```

## Reachability

Public services are the roots of the generated container: only they and the
services reachable from them through constructor arguments are emitted.
Everything else is pruned after validation and never reaches the generated
file.

- Public tags count as roots too — a `public: true` tag keeps every service
  it collects
- A config in which nothing is public generates a container with no services
- Pruning happens after all type checking, so an unreachable service with a
  broken constructor still fails generation
- Importing a large config costs nothing at runtime: the services you do not
  reach are not generated

This is why an imported service can be missing a getter you expected: mark it
`public: true`, reach it from something public, or generate with
`--enable-pass=expose-all`, which makes every service public and thereby
disables pruning.

## Method Constructors

Use service methods to construct other services:

```yaml
services:
  factory:
    constructor:
      func: "factory.New"
    shared: true

  processor:
    constructor:
      method: "@factory.CreateProcessor"
      args:
        - "@dependency"
    shared: false
```

Generated code:
```go
func (c *Container) buildProcessor() (*Processor, error) {
    factory, err := c.getFactory()
    if err != nil {
        return nil, err
    }
    dep, err := c.getDependency()
    if err != nil {
        return nil, err
    }
    return factory.CreateProcessor(dep), nil
}
```

## Generic Constructors

Support for Go generics with type arguments:

```yaml
services:
  events:
    constructor:
      func: "github.com/gendi-org/gendi/stdlib.NewChan[github.com/myapp/events.Event]"
      args:
        - 100  # buffer size
    public: true
```

Generated code:
```go
func (c *Container) buildEvents() (chan events.Event, error) {
    return make(chan events.Event, 100), nil
}
```

`stdlib.NewChan` is a real generic function of the `stdlib` package; the
generator recognizes it and emits the equivalent `make` expression through a
dedicated inliner instead of calling it, so the generated file does not import
`stdlib`. A generic constructor from any other package is instantiated and
called normally: `pkg.NewPool[events.Event](100)`.

## Service Decorators

Decorators wrap existing services to add behavior:

```yaml
services:
  logger:
    constructor:
      func: "log.New"

  logging_decorator:
    constructor:
      func: "log.NewDecorator"
      args:
        - "@.inner"  # Receives the decorated service
    decorates: logger
    decoration_priority: 10

  metrics_decorator:
    constructor:
      func: "metrics.NewDecorator"
      args:
        - "@.inner"
    decorates: logger
    decoration_priority: 20  # Higher priority wraps first
```

**Execution order:** `metrics(logging(logger))`

**Decorator Rules:**
- Must use `@.inner` in constructor args to receive the decorated service
- Multiple decorators on same service are ordered by `decoration_priority` (descending)
- The decorated service's own ID becomes an alias to the outermost decorator,
  so `@logger` resolves to `metrics(logging(logger))`
- The original definition is moved to `<decorator>.inner`, which `@.inner`
  points at (the `.inner` suffix is reserved and cannot be used in service IDs)
- The decorator type must be compatible with the decorated service type
- The base service and each decorator keep their own `shared` setting: a
  shared service may be wrapped by a non-shared decorator and vice versa
- The decorator does not inherit tags of the decorated base service. Tags
  declared on the base stay on its inner, undecorated definition, so tagged
  collections receive the undecorated instance (Symfony semantics) — tag the
  decorator explicitly if the collection should get the decorated one
- A decorator cannot itself be decorated
