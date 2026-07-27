---
title: Parameters
weight: 3
description: "Scalar configuration values injected as %name%, their contextual target types, and the providers a container reads them through at run time."
---

Parameters are scalar configuration values injected using `%name%` syntax.
A declaration is just a default value — parameters have no declared type.
The target type is contextual: it comes from the constructor argument the
parameter is injected into, so the same parameter can be requested as
different types at different injection sites.

Parameters are immutable: a container reads them through its provider and
never writes them back, so the values a container was built with hold for its
whole lifetime.

## Parameter Definition

```yaml
parameters:
  app_name: "MyApp"
  port: 8080
  timeout: 30s
  debug: true
  rate_limit: 99.5
```

Each default must be a plain scalar; null and mapping values are rejected.

## Supported Target Types

The constructor argument a parameter is injected into must have one of these
types (or a named type whose underlying type is one of them — a static
conversion is generated):

| Target | Conversion method | Notes |
|--------|-------------------|-------|
| `string` | `ToString` | numeric values format canonically |
| `bool` | `ToBool` | strings parse via `strconv.ParseBool` |
| `int`, `int8`–`int64` | `ToInt`–`ToInt64` | range-checked; strings parse base 10 |
| `uint`, `uint8`–`uint64` | `ToUint`–`ToUint64` | sign- and range-checked |
| `float32`, `float64` | `ToFloat32`, `ToFloat64` | integers must convert exactly |
| `time.Duration` | `ToDuration` | strings via `time.ParseDuration`, integers as nanoseconds |
| `time.Time` | `ToTime` | strings parse as RFC3339 |

`uintptr` and complex types are not supported. A parameter injected into any
other target type fails generation.

Exact `time.Duration` targets use `ToDuration`; a named type defined as
`type Timeout time.Duration` has underlying `int64` and therefore uses
`ToInt64` — a string default like `"5s"` for such a target fails at
generation time. Use exact `time.Duration` or a numeric default instead.

## Supplying Values at Runtime

The YAML declarations are defaults. The generator emits them as
`var Default<Container>Parameters = parameters.NewProviderMap(...)`, and
`NewContainer(nil)` uses that provider. Pass a different
`parameters.Provider` to take values from somewhere else:

```go
container := di.NewContainer(parameters.NewProviderComposite(
    di.DefaultContainerParameters,             // YAML defaults
    parameters.NewProviderStructTag(appConfig), // overrides from the app config
))
```

Ready-made providers:

| Provider | Source of values |
|----------|------------------|
| `parameters.NewProviderMap(map[string]any)` | in-memory map |
| `parameters.NewProviderStructTag(v)` | struct fields tagged `di-param` |
| `parameters.NewProviderComposite(p...)` | several providers, later wins |
| `parameters.NewProviderNull()` / `ProviderNullInstance` | nothing; every lookup misses |

Only the parameters an injected argument actually references reach the
generated map: after unreachable services are pruned, every parameter no
surviving service injects is dropped from the defaults. A parameter declared
in an imported file but never injected — a common outcome of importing
`stdlib/gendi.yaml` for a single service — therefore has no entry in
`Default<Container>Parameters`. When nothing survives, the variable is not
emitted at all and `NewContainer(nil)` falls back to
`parameters.ProviderNullInstance`, which reports every name as missing.
Pruning does not skip validation: declared defaults are checked against their
injection sites before pruning runs (see
[Generation-Time Validation](#generation-time-validation)).

`ProviderComposite` queries its providers from last to first, skipping the ones
that report `parameters.ErrParameterNotFound`, so the last provider that has a
name wins. A name no provider knows fails the getter with an error wrapping
`parameters.ErrParameterNotFound`.

`ProviderStructTag` walks a struct (pointers are dereferenced):

```go
type Config struct {
    HTTP     HTTPConfig                        // untagged: searched through
    Port     int               `di-param:"port"`
    Features map[string]string `di-param:"feature"` // feature.beta
    Secret   string            `di-param:"-"`       // never exposed
}

type HTTPConfig struct {
    Timeout time.Duration `di-param:"http.timeout"`
}
```

- A tagged scalar field answers exactly its tag name
- An untagged struct field is traversed transparently — nested fields keep the
  names their own tags give them
- A tagged struct field acts as a prefix (`tag.field`); asked for the tag itself
  it returns the struct as a value, which suits `time.Time` and fails the cast
  for anything else
- A tagged map with string keys answers `tag.key`, nested maps `tag.key.subkey`
- Unexported fields and `di-param:"-"` are skipped
- Values are read reflectively, so named types are normalized to their base kind
  before casting

## Lookup and Casting

At runtime a parameter is resolved in two steps: the provider locates the
raw value (`Provider.Lookup(name)`), then the container's caster converts it
to the injection site's target type. Generated code performs both steps
through a single `parameters.Resolver` facade call (`c.paramsResolver.Int("port")`);
the facade is a concrete struct over `Provider` + `Caster`, so the two
responsibilities stay independently replaceable. The default
`parameters.StandardCaster` rejects lossy conversions: float→integer,
bool→anything, values that overflow the target, inexact integer↔float
conversions, NaN and infinities, and named input types. Cast errors name the
raw value, its type, and the target type; the generated wrapping adds the
parameter name, service ID, and argument index.

A custom caster can override individual conversions:

```go
type LenientCaster struct {
	parameters.StandardCaster
}

func (LenientCaster) ToInt(value any) (int, error) {
	// custom policy
}

container := di.NewContainer(nil, di.WithContainerParameterCaster(LenientCaster{}))
```

Custom casters can build rejection errors with `parameters.NewCastError(value, "int")`
to keep their messages consistent with the standard policy. Every rejection wraps
the `parameters.ErrCannotCast` sentinel, so applications can distinguish cast
failures from missing parameters (`parameters.ErrParameterNotFound`) via `errors.Is`.

## Generation-Time Validation

Declared defaults are validated during generation by executing the real
`StandardCaster` against every usage's target type, so a default like
`timeout: "5x"` injected as `time.Duration` fails generation, not startup.
Values supplied by a runtime provider are checked at construction time.

Validation runs before pruning and covers every injection site, including
those of services that are later pruned as unreachable. A parameter with no
injection site at all has no target type to validate against; it is neither
checked nor emitted.

## Parameter References

Reference parameters in constructor arguments using `%param_name%`:

```yaml
services:
  server:
    constructor:
      func: "server.New"
      args:
        - "%app_name%"
        - "%port%"
```

## Parameter Overrides

Later imports override earlier parameter values:

```yaml
# base.yaml
parameters:
  db_dsn: "postgres://localhost/dev"

# production.yaml
imports:
  - ./base.yaml

parameters:
  db_dsn: "postgres://prod-host/app"  # Overrides base.yaml
```
