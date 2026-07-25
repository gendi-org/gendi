# Generation

## Container Structure

```go
type Container struct {
    // private fields
}
```

Constructor:
```go
func NewContainer(params parameters.Provider, opts ...ContainerOption) *Container
```

Options:
```go
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
parameter with one typed call per injection site, e.g. `c.paramsResolver.Int("port")`.

## Getter Methods

- `GetX() (T, error)`
- `MustX() T` — panics on error and optionally reports via container error handler

All getters are strictly typed.

## Reachability Pruning

Public services (including desugared public tags) are the generation roots.
Services unreachable from any root are removed after validation and are not
emitted; the parameters only they injected are pruned with them. Pruning runs
after type checking, so errors in an unreachable service still fail
generation. The `expose-all` pass makes every service public and therefore
leaves nothing to prune.

## Shared vs Non-shared

- Shared: lazy singleton
- Non-shared: new instance per call

## Error Reporting

The generator emits diagnostic errors containing:
- service ID
- configuration field or argument index
- expected vs actual type
- dependency chain when applicable

Example:
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

## Circular Dependency Detection

- Cycles are detected at generation time
- Generation fails
- Errors include the dependency trace
