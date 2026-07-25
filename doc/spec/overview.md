# Overview

## Purpose and Scope

gendi is a Go library and code generator that produces a statically typed DI container from declarative YAML configuration.

Key objectives:
- Generate Go code without runtime reflection
- Generate dedicated getter methods with concrete Go types
- Explicit dependency configuration (no autowiring)
- Support decorators, tags, compiler passes, and imports
- Detect configuration and type errors at generation time

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

## In Scope / Out of Scope

In scope:
- YAML-based service configuration
- Go code generation for DI container
- Strict static type validation
- Decorators and decorator chains
- Tagged services
- Configuration imports
- `go generate` workflows

Out of scope:
- Autowiring
- Runtime DI / service locator
- Reflection-based resolution
- Expression language or env interpolation
- Hot-reload of configuration

## Architecture Overview

Components:
1. CLI generator
   - Parses YAML
   - Resolves imports
   - Applies compiler passes
   - Analyzes Go code (`go/packages`)
   - Generates container code
2. Runtime packages (minimal)
   - `parameters` — the only package generated containers depend on:
     `Provider` (lookup), `Caster`/`StandardCaster` (conversion), the
     `Resolver` facade over both, the shipped providers (map, struct tag,
     composite, null), and the `ErrParameterNotFound`/`ErrCannotCast`
     sentinels
   - `stdlib` — optional factories for common standard library types, plus
     `MakeSlice`/`NewChan`, which the generator inlines rather than calls
3. Generated container
   - `Container` struct
   - Typed getter methods
   - Lazy initialization for shared services
   - No reflection
