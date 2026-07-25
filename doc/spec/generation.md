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
It is what `NewContainer(nil)` falls back to; a config without parameters emits
no such variable and falls back to `parameters.ProviderNullInstance`.

The container stores a `parameters.Resolver` (a facade over `Provider` and
`Caster`, default caster `parameters.StandardCaster`) and resolves each
parameter with one typed call per injection site, e.g. `c.paramsResolver.Int("port")`.

## Getter Methods

- `GetX() (T, error)`
- `MustX() T` — panics on error and optionally reports via container error handler

All getters are strictly typed.

## Shared vs Non-shared

- Shared: lazy singleton
- Non-shared: new instance per call

## Error Reporting

The generator emits diagnostic errors containing:
- service ID
- configuration field
- expected vs actual type
- dependency chain when applicable

Example:
```
service "payments":
  constructor "NewService":
  arg[0]: expected []payments.Provider, got []any
```

## Circular Dependency Detection

- Cycles are detected at generation time
- Generation fails
- Errors include the dependency trace
