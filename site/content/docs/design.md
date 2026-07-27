---
title: Design
weight: 8
---

Why gendi is shaped the way it is, and what the generated container looks
like. For the YAML surface see the
[Configuration Reference](./configuration/_index.md).

## Purpose and Scope

gendi is a Go library and code generator that produces a statically typed DI
container from declarative YAML configuration.

Four goals shape every decision that follows.

**Nobody should assemble a service graph by hand.** Wiring written as Go grows
into a `main` that is long, order-sensitive and touched by every feature — a
file where the interesting change is one line among fifty of plumbing. Moving
the graph into configuration leaves the plumbing to the generator and keeps the
diff of a new service down to the service.

**A library should be able to ship its wiring without shipping a framework.**
A package can publish its own `gendi.yaml`, and a consumer imports it by module
path (see [Shipping Wiring in a Library](library-wiring.md));
`stdlib/gendi.yaml` in this repository is that mechanism used on itself. The wiring travels as data, so the
library's Go API stays free of container types and its dependency graph does
not grow — a consumer that never uses gendi is unaffected by the file's
existence.

**The generated container must be boring to read.** One build function per
service, direct calls, concrete types, no reflection and no indirection to
follow. Answering "what is injected here" is reading, not reasoning: the call
is written out. That is what makes the output reviewable in a diff, debuggable
in a stack trace, and unambiguous to any tool that reads it — including the one
that wrote the configuration.

**The configuration must be writable by machines, not only by people.** The
surface is a small declarative file with a published JSON schema
(`gendi.schema.json`), not an API to be discovered by exploration, and
`doc/LLM.md` states its rules in short form. What makes it safe to generate is
the other end: a wrong guess fails at generation with the offending line and a
caret under the token, instead of compiling into something subtly wrong.

In scope: YAML service configuration, container code generation, strict
static type validation, decorators and decorator chains, tagged services,
configuration imports, `go generate` workflows.

Out of scope: autowiring, runtime DI / service locator, reflection-based
resolution, an expression language or env interpolation, hot-reload of
configuration.

## Fixed Decisions

1. `services.type` is optional — inferred from the constructor, a contract
   when given
2. Tagged injection produces `[]T` only
3. `tags.element_type` is optional, required when `public` or
   `autoconfigure` is set
4. Strict typing: a service type is never `any`
5. Errors are detected at generation time

## Terminology

| Term | Definition |
|-----|------------|
| Service ID | String identifier of a service in configuration |
| Constructor | Go function or method that creates a service instance |
| Service Type | Type returned by the service getter |
| Shared service | Singleton service |
| Non-shared service | Factory service |
| Decorator | Service that wraps another service |
| Tag | Label assigned to a service |
| Tagged injection | Injection of a collection of services by tag |
| Compiler pass | Programmatic modification of configuration before generation |

## Architecture

1. CLI generator
   - Parses YAML and resolves imports
   - Applies compiler passes
   - Analyzes Go code (`go/packages`)
   - Generates container code
2. Runtime packages
   - `parameters` — the only package generated containers depend on:
     `Provider` (lookup), `Caster`/`StandardCaster` (conversion), the
     `Resolver` facade over both, the shipped providers (map, struct tag,
     composite, null), and the `ErrParameterNotFound`/`ErrCannotCast`
     sentinels
   - `stdlib` — optional factories for common standard library types, plus
     `MakeSlice`/`NewChan`, which the generator inlines rather than calls
3. Generated container
   - `Container` struct with typed getter methods
   - Lazy initialization for shared services
   - No reflection

## Generated Container

```go
type Container struct {
    // private fields
}

func NewContainer(params parameters.Provider, opts ...ContainerOption) *Container

func WithContainerErrorHandler(handler func(serviceName string, err error)) ContainerOption
func WithContainerParameterCaster(caster parameters.Caster) ContainerOption
```

Declared parameter defaults are emitted as a package-level provider:

```go
var DefaultContainerParameters = parameters.NewProviderMap(map[string]any{ /* ... */ })
```

It is what `NewContainer(nil)` falls back to. The map holds only the
parameters a surviving service injects: unreferenced parameters are pruned,
and when none remain the variable is not emitted and the fallback becomes
`parameters.ProviderNullInstance`.

The container stores a `parameters.Resolver` (a facade over `Provider` and
`Caster`, default caster `parameters.StandardCaster`) and resolves each
parameter with one typed call per injection site, e.g.
`c.paramsResolver.Int("port")`.

### Getter Methods

- `GetX() (T, error)`
- `MustX() T` — panics on error and optionally reports via the container
  error handler

All getters are strictly typed. Shared services are lazy singletons;
non-shared services are built anew on every call.

### Reachability Pruning

Public services (including desugared public tags) are the generation roots.
Services unreachable from any root are removed after validation and are not
emitted; the parameters only they injected are pruned with them. Pruning runs
after type checking, so errors in an unreachable service still fail
generation. The `expose-all` pass makes every service public and therefore
leaves nothing to prune.

## Error Reporting

Diagnostics carry the service ID, the configuration field or argument index,
expected versus actual type, and the dependency chain where one applies:

```
service "app" arg[0]: service "cache" type *example.com/ef.Cache is not assignable to *example.com/ef.Store
```

Errors carrying a source location are rendered with the offending config line
and a caret under it:

```
gendi.yaml:4:13: service "thing" constructor.func: constructor must not return the empty interface (any); a service needs a type the container can check statically
3 |     constructor:
4 |       func: "example.com/anytest.NewAny"
                ^
```

Dependency cycles are detected at generation time; generation fails and the
error includes the trace.
