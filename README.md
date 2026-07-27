# gendi - Compile-Time Dependency Injection for Go

[![CI](https://github.com/gendi-org/gendi/actions/workflows/ci.yml/badge.svg)](https://github.com/gendi-org/gendi/actions/workflows/ci.yml)
[![codecov](https://codecov.io/gh/gendi-org/gendi/branch/master/graph/badge.svg)](https://codecov.io/gh/gendi-org/gendi)
[![License](https://img.shields.io/badge/license-Apache--2.0-blue.svg)](./LICENSE)

`gendi` reads YAML service definitions and generates a Go container: no runtime
reflection, no autowiring, every dependency resolved and type-checked while the
code is generated.

## Why

- **Nobody assembles the graph by hand.** The wiring lives in configuration, so
  `main` stops growing with every new service and the diff for a feature
  contains the feature rather than fifty lines of plumbing
- **Libraries can ship their wiring without shipping a framework.** A package
  publishes its own `gendi.yaml` and consumers import it by module path. It
  travels as data: no container types in the library's API, and nothing added
  to the dependency graph of anyone who never uses gendi
- **The generated container is boring on purpose.** One build function per
  service, direct calls, concrete types, no reflection. What is injected where
  is written out, not inferred — legible in a diff and in a stack trace
- **Tools can write the configuration, not just people.** A small declarative
  surface with a published JSON schema, where a wrong guess fails at generation
  with the offending YAML line instead of compiling into something subtly wrong
- **It fails before your program is built.** Missing dependencies, type
  mismatches, circular references and unconvertible parameter defaults are all
  generation errors, each reported with a caret under the offending token
- **Configuration composes.** Imports with overrides, tagged collections,
  decorators with priorities, and compiler passes for project-wide conventions

## Quick Start

```bash
go get -tool github.com/gendi-org/gendi/cmd/gendi
```

Declare a service — `func` is a package path plus a function name, `%greeting%`
a parameter, `public: true` a request for a getter:

```yaml
# gendi.yaml
# yaml-language-server: $schema=https://raw.githubusercontent.com/gendi-org/gendi/master/gendi.schema.json
parameters:
  greeting: "Hello"

services:
  greeter:
    constructor:
      func: "example.com/myapp/greet.New"
      args:
        - "%greeting%"
    public: true
```

The schema comment is optional; it gives you completion and validation in any
editor that reads YAML schemas.

```bash
go tool gendi --config=gendi.yaml --out=./di --pkg=di
```

```go
fmt.Println(di.NewContainer(nil).MustGreeter().Greet("world"))
// Hello, world!
```

[Getting Started](./site/content/docs/_index.md) walks the same loop in full,
including what the generated container looks like and how to supply parameters
at runtime.

## Documentation

- **[Getting Started](./site/content/docs/_index.md)** — the whole loop once: install, declare, generate, use
- **[Configuration Reference](./site/content/docs/configuration/)** — the YAML surface, one page per concept
- **[CLI](./site/content/docs/cli.md)** — flags and the built-in passes
- **[Shipping Wiring in a Library](./site/content/docs/library-wiring.md)** — publishing a `gendi.yaml` your users import by module path
- **[Compiler Passes](./site/content/docs/passes.md)** — rewriting the configuration before generation
- **[Troubleshooting](./site/content/docs/troubleshooting.md)** — what each generation error means
- **[Design](./site/content/docs/design.md)** — what the container guarantees, and what gendi deliberately does not do
- **[stdlib Services](./stdlib/README.md)** — ready-made services for HTTP clients, loggers and channels
- **[API Reference](https://pkg.go.dev/github.com/gendi-org/gendi)** — `di.Pass`, `parameters.Provider`, `parameters.Caster`

A realistic service wired end to end lives in
**[gendi-org/gendi-example-app](https://github.com/gendi-org/gendi-example-app)**.

## Requirements

- Go 1.25.4 or later
- Generated code depends only on `github.com/gendi-org/gendi/parameters`
- A configuration that declares any tag additionally needs the
  `github.com/gendi-org/gendi` module to be resolvable from the module being
  generated into; installing gendi as a tool dependency satisfies this
  ([why](./site/content/docs/troubleshooting.md))

## License

gendi is licensed under the [Apache License 2.0](./LICENSE).

## Related Projects

- [google/wire](https://github.com/google/wire) - Compile-time DI with code generation
- [uber-go/fx](https://github.com/uber-go/fx) - Runtime dependency injection framework
- [Symfony DependencyInjection](https://symfony.com/doc/current/components/dependency_injection.html) - Inspiration for YAML format
