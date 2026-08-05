---
title: Notes for AI Agents
weight: 9
description: "Everything an agent needs to write a GenDI configuration correctly, in short form — also served verbatim at /llms.txt."
---

Short, stable facts about GenDI for tooling and assistants. The same text is
served at `https://gendi.dev/llms.txt`, so an agent can read it in one fetch
instead of crawling this documentation.

The project is `GenDI`; the module, the command and the config file are all
lowercase `gendi`.

## Schema

`gendi.schema.json` in the repository root is the machine-readable contract for
a config file — validate a generated config against it before running the
generator. A test keeps it in step with the loader, so what it rejects, the
loader rejects. Editors get the same checking from a first line of:

```yaml
# yaml-language-server: $schema=https://raw.githubusercontent.com/gendi-org/gendi/master/gendi.schema.json
```

Anything the schema accepts can still fail generation for reasons it cannot
see — unknown service IDs, type mismatches against the real Go signatures,
dependency cycles. Those are reported with the offending line and a caret.

## CLI

```bash
go get -tool github.com/gendi-org/gendi/cmd/gendi   # once, per module
go tool gendi --config=gendi.yaml --out=./di --pkg=di
```

Flags: `--config`, `--out`, `--pkg`, `--container`, `--build-tags`, `--enable-pass`, `--verbose`.

## YAML Syntax

Constructor args:
- `@service.id` service reference
- `@.inner` inner service (decorators only)
- `%param.name%` parameter
- `!tagged:tag.name` tagged injection
- `!spread:@service` spread slice into variadic
- `!spread:!tagged:tag` spread tagged collection into variadic
- `!go:pkg.Symbol` Go package-level var/const (e.g. `!go:os.Stdout`)
- `!field:@service.Field` field access on service (e.g. `!field:@config.Host`)
- `!field:!go:pkg.Symbol.Field` field access on Go symbol (e.g. `!field:!go:http.DefaultClient.Timeout`)
- literal scalars (string/int/float/bool/null)
- a one-level YAML mapping fills a `map[K]V` parameter: keys are literals
  checked against the key type, values take any form from this list except
  `!tagged:`, `!spread:` and `@.inner`; nested mappings and sequences are
  rejected; a key cannot be `null`; a duplicate key is an error, caught by the
  YAML parser before gendi does, comparing keys by their written form (`5` and
  `"5"` collide); entries keep their source order in the generated literal

```yaml
services:
  router:
    constructor:
      func: "app.NewRouter"  # func NewRouter(routes map[string]Handler) *Router
      args:
        - "/":    "@handler.home"
          "/api": "@handler.api"
```

Method constructors are configured via the `constructor.method` field
(`method: "@factory.Create"`), not as a constructor argument.

`$this.` in `type` and `func` fields, a tag's `element_type`, `!go:` arguments,
and `!field:!go:` arguments resolves to the Go package path of the config file.
The `method` field takes no `$this` — it addresses a service, not a package.

Imports support `exclude`:
```yaml
imports:
  - path: ./services/*.yaml
    exclude:
      - ./services/test_*.yaml
```

## Service Defaults

`services._default` sets the default `shared`, `public`, and `autoconfigure`
flags for the services of the same file (per file; not inherited across
imports). Any other field in `_default` is an error. Built-in defaults:
`shared: true`, `public: false`, `autoconfigure: true`.

## Parameters

Declared as plain scalar defaults (no `type` field; null and mapping values
rejected):
```yaml
parameters:
  port: 8080
  timeout: 5s
```

The target type is contextual — taken from each constructor argument. One
parameter may be injected as different types at different sites. Supported
targets: `string`, `bool`, all int/uint widths, `float32`, `float64`,
`time.Duration`, `time.Time`, and named types over those (static conversion
is generated). `uintptr` unsupported.

Runtime split: `Provider.Lookup(name) (any, error)` returns the raw value;
`parameters.Caster` (`StandardCaster` by default, override via
`With<Container>ParameterCaster`) converts it per injection site. Generated
code calls both through the `parameters.Resolver` facade struct:
`c.paramsResolver.Int("port")`. Unsupported
targets and non-convertible defaults fail at generation time; runtime
provider values are checked at construction with parameter name, service ID,
argument index, raw and target types in the error.

## Spread Rules

- Only one `!spread:` per constructor call
- Must be the last argument
- Inner expression must resolve to a slice
- Target parameter must be variadic

## Pruning

Public services (and public tags) are the generation roots. Services
unreachable from a root are dropped after validation, together with the
parameters only they injected. `--enable-pass=expose-all` makes everything
public and disables pruning.

## Generated Container API

```go
var DefaultContainerParameters *parameters.ProviderMap // injected YAML defaults; omitted when none survive pruning
func NewContainer(params parameters.Provider, opts ...ContainerOption) *Container
func (c *Container) GetServiceName() (ServiceType, error)
func (c *Container) MustServiceName() ServiceType
func WithContainerErrorHandler(handler func(serviceName string, err error)) ContainerOption
func WithContainerParameterCaster(caster parameters.Caster) ContainerOption
```

`NewContainer(nil)` uses `DefaultContainerParameters`, or
`parameters.ProviderNullInstance` when it is absent. Parameters no surviving
service injects are pruned from the defaults after unreachable services are
pruned; the variable is emitted only when at least one remains.
Available providers: `NewProviderMap`, `NewProviderStructTag` (`di-param`
struct tags), `NewProviderComposite` (last provider holding a name wins),
`NewProviderNull`.

## Generated File Conventions

- Generated files are `*_gen.go`
- Banner: `// Code generated by gendi; DO NOT EDIT.`
