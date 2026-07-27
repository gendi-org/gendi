# gendi - Compile-Time Dependency Injection for Go

[![CI](https://github.com/gendi-org/gendi/actions/workflows/ci.yml/badge.svg)](https://github.com/gendi-org/gendi/actions/workflows/ci.yml)
[![codecov](https://codecov.io/gh/gendi-org/gendi/branch/master/graph/badge.svg)](https://codecov.io/gh/gendi-org/gendi)
[![License](https://img.shields.io/badge/license-Apache--2.0-blue.svg)](./LICENSE)

`gendi` reads YAML service definitions and generates a Go container: no runtime
reflection, no autowiring, every dependency resolved and type-checked while the
code is generated.

## Why

- **It fails before your program is built.** Missing dependencies, type
  mismatches, circular references and unconvertible parameter defaults are
  generation errors, each reported with the offending YAML line and a caret
- **Nothing is inferred but types.** The wiring is written down, so it can be
  read and reviewed; gendi never guesses which implementation you meant
- **The output is ordinary Go.** Direct calls and typed getters, deterministic
  byte-for-byte across runs, safe to commit and diff
- **Configuration composes.** Imports with overrides, tagged collections,
  decorators with priorities — a service graph assembled from many files
- **It bends to your conventions.** Compiler passes rewrite the configuration
  before generation, so project-wide rules stay in one place
- **Generated code carries almost no runtime.** One small package,
  `github.com/gendi-org/gendi/parameters`, and nothing else

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
