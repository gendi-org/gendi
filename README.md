# gendi - Compile-Time Dependency Injection for Go

[![CI](https://github.com/gendi-org/gendi/actions/workflows/ci.yml/badge.svg)](https://github.com/gendi-org/gendi/actions/workflows/ci.yml)
[![codecov](https://codecov.io/gh/gendi-org/gendi/branch/master/graph/badge.svg)](https://codecov.io/gh/gendi-org/gendi)
[![License](https://img.shields.io/badge/license-Apache--2.0-blue.svg)](./LICENSE)

`gendi` is a compile-time dependency injection container generator for Go. It reads YAML configuration files and generates type-safe, efficient container code with full compile-time validation.

## Features

- **Compile-time type safety** - All dependencies resolved and type-checked during code generation
- **Zero runtime reflection** - Generated code uses direct type assertions
- **YAML configuration** - Declarative service definitions with imports and overrides
- **Service lifecycle** - Shared (singleton) and non-shared (factory) services
- **Tagged injection** - Collect multiple services by tag with custom sorting
- **Service decoration** - Decorator pattern with priority ordering
- **Method constructors** - Use service methods as constructors
- **Variadic functions** - Full support for variadic constructors
- **Generic constructors** - Support for Go generics with type arguments
- **Custom compiler passes** - Transform configuration before generation
- **Parameter injection** - Type-safe parameter references with automatic conversion
- **Circular dependency detection** - Catches circular references at generation time
- **Public API generation** - Expose selected services via public getter methods
- **Standard library factories** - Ready-to-use factories for common stdlib types

## Installation

Add gendi as a tool dependency to your project:

```bash
go get -tool github.com/gendi-org/gendi/cmd/gendi
```

This adds gendi to your `go.mod` and allows running it via `go tool gendi`.

## Quick Start

### 1. Create a service configuration

**gendi.yaml:**
```yaml
# yaml-language-server: $schema=https://raw.githubusercontent.com/gendi-org/gendi/master/gendi.schema.json

parameters:
  db_dsn: "postgres://localhost/myapp"

services:
  database:
    constructor:
      func: "github.com/myapp/db.New"
      args:
        - "%db_dsn%"  # Parameter reference
    shared: true

  user_repo:
    constructor:
      func: "github.com/myapp/repo.NewUserRepository"
      args:
        - "@database"  # Service reference
    shared: true
    public: true  # Exposed via public getter
```

> **💡 Tip:** Add the schema comment at the top of your YAML files to get autocomplete and validation in editors that support YAML schemas (VS Code, IntelliJ, etc.)

### 2. Generate the container

```bash
go tool gendi --config=gendi.yaml --out=./di --pkg=di
```

Or use `go:generate`:
```go
//go:generate go tool gendi --config=gendi.yaml --out=./di --pkg=di
```

### 3. Use the generated container

```go
package main

import "github.com/myapp/di"

func main() {
    container := di.NewContainer(nil)

    userRepo, err := container.GetUserRepo()
    if err != nil {
        panic(err)
    }

    // Use userRepo...
}
```

## Core Concepts

### Parameters

Untyped configuration values injected using `%name%` syntax. A parameter is a
plain scalar default with no declared type; the target type is resolved
contextually from each constructor argument it is injected into. Supported
target types include `string`, `bool`, all signed and unsigned integer widths,
`float32`, `float64`, `time.Duration`, and `time.Time`.

The YAML values are defaults; at runtime a container reads parameters through a
`parameters.Provider` — a map, your own config struct tagged with `di-param`, or
a composite of several sources.

### Services

Objects constructed and managed by the container. Services can be:
- **Shared (singleton)**: Created once, cached, thread-safe
- **Non-shared (factory)**: New instance on each access
- **Public**: Exposed via public getter methods
- **Decorated**: Wrapped by decorator services

### Tags

Collect multiple services implementing a common interface. Tags support:
- Custom sorting by attributes
- Auto-configuration (automatic tagging)
- Public getters for tagged collections

### Imports

Configuration files can import and override other configurations using relative paths, glob patterns, or module imports.

**📖 See [Configuration Reference](./doc/configuration.md) for complete YAML syntax and examples.**

## CLI Usage

```bash
go tool gendi [flags]
```

| Flag | Description |
|------|-------------|
| `--config string` | Root YAML configuration file (required) |
| `--out string` | Output directory or file (required) |
| `--pkg string` | Go package name (required) |
| `--container string` | Container struct name (default `"Container"`) |
| `--build-tags string` | Go build tags — used for type resolution and emitted as the generated file's `//go:build` header |
| `--enable-pass value` | Enable a specific compiler pass; repeat the flag to enable several |
| `--verbose` | Verbose logging |

The stock binary ships two selectable passes; an unknown name is an error:

| `--enable-pass` | Effect |
|-----------------|--------|
| `slog` | Gives every service tagged `slog` with a `channel` attribute its own channel-scoped logger derived from the `logger` service (see [stdlib/README.md](./stdlib/README.md#slogpass)) |
| `expose-all` | Marks every service public, so each one gets a getter. For test containers — it overrides explicit `public: false` and disables unreachable-service pruning |

**Examples:**

```bash
# Generate to directory
go tool gendi --config=gendi.yaml --out=./di --pkg=di

# Generate specific file
go tool gendi --config=gendi.yaml --out=./di/container_gen.go --pkg=di

# With build tags
go tool gendi --config=gendi.yaml --out=./di --pkg=di --build-tags=integration
```

## Custom Compiler Passes

Compiler passes transform configuration before code generation, enabling project-specific conventions:

```go
type AutoTagPass struct{}

func (p *AutoTagPass) Name() string { return "auto-tag" }

func (p *AutoTagPass) Process(cfg *di.Config) (*di.Config, error) {
    for id, svc := range cfg.Services {
        if strings.HasSuffix(id, ".handler") {
            svc.Tags = append(svc.Tags, di.ServiceTag{Name: "http.handler"})
            cfg.Services[id] = svc
        }
    }
    return cfg, nil
}
```

**📖 See [Custom Passes Guide](./doc/custom-passes.md) for complete documentation and examples.**

## Standard Library Services

Pre-configured services for common stdlib types (HTTP clients, loggers, channels):

```yaml
imports:
  - github.com/gendi-org/gendi/stdlib/gendi.yaml

services:
  my_service:
    constructor:
      func: "github.com/myapp.NewService"
      args:
        - "@stdlib.http.client"  # Pre-configured HTTP client
        - "@stdlib.logger"       # Pre-configured slog logger
```

**📖 See [stdlib/README.md](./stdlib/README.md) for all available services and factory functions.**

## Examples

The flagship demo lives in a separate repo: **[gendi-org/gendi-example-app](https://github.com/gendi-org/gendi-example-app)** — a realistic HTTP task-tracker service wired end-to-end by gendi, covering services, tagged injection, decorators, stdlib and built-in passes, imports, and integration tests.

## Documentation

- **[Configuration Reference](./doc/configuration.md)** - Complete YAML syntax and examples
- **[Custom Passes Guide](./doc/custom-passes.md)** - Writing custom compiler passes
- **[stdlib Services](./stdlib/README.md)** - Pre-configured standard library services
- **[Design](./doc/design.md)** - Architecture, generated container, and design decisions
- **[API Documentation](https://pkg.go.dev/github.com/gendi-org/gendi)** - Go package documentation

## Requirements

- Go 1.25.4 or later
- No runtime dependencies for generated code (except `github.com/gendi-org/gendi/parameters`)
- Generating a config that declares any tag additionally requires the
  `github.com/gendi-org/gendi` module itself to be resolvable from the generated
  package's module — tagged collections are desugared to `stdlib.MakeSlice`
  during analysis. Installing gendi as a tool dependency satisfies this; a
  prebuilt binary run against a module that does not require gendi fails with
  `no required module provides package github.com/gendi-org/gendi/stdlib`

## License

gendi is licensed under the [Apache License 2.0](./LICENSE).

## Related Projects

- [google/wire](https://github.com/google/wire) - Compile-time DI with code generation
- [uber-go/fx](https://github.com/uber-go/fx) - Runtime dependency injection framework
- [Symfony DependencyInjection](https://symfony.com/doc/current/components/dependency_injection.html) - Inspiration for YAML format
