This file provides guidance to LLM tools when working with code in this repository.

## Project Overview

**gendi** is a compile-time dependency injection container generator for Go. It reads YAML configuration files and generates type-safe, efficient container code with full compile-time validation—no runtime reflection.

Key characteristics:
- All dependencies are resolved and type-checked during code generation
- Generated code uses direct type assertions
- YAML-based declarative service definitions with imports and overrides
- Support for service lifecycle (shared/non-shared), decorators, tagged injection, and custom compiler passes

## Essential Commands

### Building and Testing
```bash
# Run all tests
go test ./...

# Build the CLI generator
go build ./cmd/gendi

# Run the generator manually
go run ./cmd/gendi --config=path/to/gendi.yaml --out=path/to/internal/di --pkg=di
```

### Demo Application
A full, realistic service demonstrating gendi end-to-end lives in the separate
repository `github.com/gendi-org/gendi-example-app`.

### Running a Single Test
```bash
# Run specific test
go test -run TestName ./path/to/package

# Example: run generator tests
go test -run TestGenerator ./generator

# Run with verbose output
go test -v -run TestName ./path/to/package
```

## Architecture

### Code Generation Pipeline

The generator follows a multi-stage pipeline:

1. **YAML Loading** (`yaml/` package)
   - Loads root YAML config with
     `yaml.LoadConfig(path, boundary, moduleContext)` — derive the boundary
     with `yaml.DefaultBoundary(path)` and use the finalized pipeline
     `Options.ModuleRoot` as the module context
   - Delegates import addressing to `imprt/`: classify (local vs module),
     compute the anchor and boundary, expand the mask, drop exclusions —
     then confines every returned candidate (`yaml/confiner.go`) immediately
     before load
   - Merges imported configs (later imports override earlier ones)
   - Resolves `$this` tokens to current package paths

2. **Compiler Passes** (`config.go`)
   - Optional transformation stage via `di.ApplyPasses()`
   - Passes implement `Pass` interface with `Process(*Config) (*Config, error)`
   - Used for custom conventions, auto-tagging, argument modification
   - See `github.com/gendi-org/gendi-example-app` (`tools/gendi/`) for a real generator wiring built-in and custom passes

3. **Type Resolution** (`typeres/` package)
   - Uses `golang.org/x/tools/go/packages` to load Go types
   - Resolves type strings to `go/types.Type`
   - Handles generic constructors with type arguments
   - Validates constructor signatures

4. **IR Building** (`ir/` package)
   - Multi-phase transformation from `di.Config` to IR `Container`
   - **Phase 1**: Build parameters, tags, and services
   - **Phase 2**: Resolve constructors, decorators, and dependencies
   - **Phase 3**: Validate (circular dependencies, type compatibility)
   - **Phase 4**: Optimize (prune unreachable services, optimize shared flags)
   - IR represents fully analyzed and validated dependency graph

5. **Code Generation** (`generator/` package)
   - Converts IR to Go source code
   - Renders: parameters map, container struct, getter methods
   - Manages imports with `ImportManager`
   - Handles identifier collision with `IdentGenerator`
   - Formats output with `go/format`

### Key Package Roles

- **`di` (root package)**: Core config types (`Config`, `Service`, `Parameter`, `Tag`), the `Pass` interface, and the mandatory `DecoratorPass`
- **`cmd/`**: CLI implementation with flag parsing and orchestration
- **`pipeline/`**: Library entry point — `Options`/`Options.Finalize()` and `Build`/`Emit`, which chain internal passes, type loading, IR building, and code generation
- **`yaml/`**: YAML parsing, config merging, `$this` token replacement, and the pre-load confinement check
- **`imprt/`**: Import addressing — classification, module lookup, glob expansion, exclusion masks
- **`gomod/`**: `go.mod` graph access (module root discovery, module directory lookup)
- **`ir/`**: Intermediate representation and multi-phase analysis
- **`generator/`**: Code generation from IR to Go source
- **`typeres/`**: Wrapper around `go/packages` for type resolution
- **`parameters/`**: Runtime parameter lookup (`Provider`), conversion (`Caster`/`StandardCaster`), and the `Resolver` facade used by generated code
- **`stdlib/`**: Pre-built factory functions for stdlib types (channels, HTTP clients, loggers) and `SLogPass`
- **`srcloc/`**: Source locations on config nodes and the error renderer that prints the offending YAML line
- **`xmaps/`**: Deterministic map iteration helpers (`OrderedKeys`)
- **`integration/`**: End-to-end and codegen tests with their `testdata/` fixtures

### Important Files

- `config.go`: Core config structures and `Pass` interface
- `pipeline/build.go`: Internal passes → type loading → IR building
- `ir/builder.go`: IR construction orchestration (phase order)
- `generator/generator.go`: Main generator entry point
- `cmd/cli.go`: CLI workflow (`Run()` and `Generate()`)

## Configuration Concepts

### Service References and Arguments

Constructor arguments use special syntax:
- `@service.id` - Reference to another service
- `@.inner` - Inner service (decorators only)
- `%param.name%` - Parameter reference
- `!tagged:tag.name` - Tagged services collection
- `!spread:@service` - Spread slice into variadic parameters
- `!spread:!tagged:tag` - Spread tagged collection into variadic parameters
- `!go:pkg.Var` - Go package-level variable or constant (e.g., `!go:os.Stdout`, `!go:log.LstdFlags`)
- `!field:@service.Field` - Field access on a service (e.g., `!field:@config.Host`, `!field:@config.Database.DSN`)
- `!field:!go:pkg.Symbol.Field` - Field access on a Go package-level variable (e.g., `!field:!go:http.DefaultClient.Timeout`)
- `literal` - YAML scalar literal

Method constructors (`@service.Method`) are not arguments — they are configured
via the `constructor.method` field (e.g. `method: "@factory.Create"`).

### Parameters

Parameters are declared as plain scalar defaults (`parameters: {port: 8080, timeout: 5s}`) — there is no `type` field. The target type is contextual: it comes from each constructor argument the parameter is injected into, so one parameter can be requested as different types at different injection sites. At runtime `parameters.Provider.Lookup(name)` returns the raw value and the container's `parameters.Caster` (default `StandardCaster`, replaceable via the generated `With<Container>ParameterCaster` option) converts it per injection site; generated code performs both steps through one `parameters.Resolver` facade call (`c.paramsResolver.Int("port")`). Unsupported target types and non-convertible declared defaults fail at generation time.

### Service Lifecycle

- **Shared (singleton)**: Created once on first access, cached, thread-safe
- **Non-shared (factory)**: New instance on each access, no caching

### Decorators

Decorators wrap existing services using `@.inner` reference. Multiple decorators are ordered by `decoration_priority` (higher priority wraps first).

### Tagged Injection

Services can be tagged and collected as `[]ElementType`. Tag entries can be a string (shorthand for `{name: "..."}`) or a mapping with `name` and attributes. Tags support:
- Optional `element_type` (inferred from usage if omitted, required when public)
- Sorting by attribute (e.g., `sort_by: "priority"`)
- Public getters when `public: true`

### Imports

Configuration files can import others:
- Relative paths resolved from importing file
- Glob patterns supported (`./services/*.yaml`)
- Later imports override earlier ones
- Each file has its own `$this` context

#### Import Exclusions

Exclude files from imports using glob patterns, file paths, or directory paths:

```yaml
imports:
  # Load all services except test files and an entire directory
  - path: ./services/**/*.yaml
    exclude:
      - ./services/test_*.yaml
      - ./services/internal

  # Glob in subdirectories with exclusions
  - path: ./config/**/*.yaml
    exclude:
      - ./config/**/dev_*.yaml
```

**An `exclude` pattern is addressed exactly like an import `path`** — it is
"one of the things the import could have matched, written the same way":

```yaml
imports:
  # local import → relative exclude (both anchored at the importing file's dir)
  - path: ./services/*.yaml
    exclude: [./services/skip.yaml]

  # module import → module-form exclude
  - path: example.com/mod/services/*.yaml
    exclude: [example.com/mod/services/skip.yaml]
```

**The exclusion's form must match the import's form**: a local import takes
local exclusions and a module import takes exclusions inside the same module
— a mismatch is a generation-time error.

**Exclusions are masks, not lookups.** They filter the files the import
found and never touch the filesystem themselves:
- A mask supports full glob syntax (`*`, `?`, `[]`, `**`) and is anchored
  like the import: local masks at the importing file's directory, module
  masks at the module's root (`example.com/mod/services/skip.yaml`)
- A mask matching a directory on a file's path — literally or via a glob —
  excludes the whole subtree
- A mask that matches none of the found files is a silent no-op, whatever
  its spelling; only a malformed pattern is an error
- Masks are applied BEFORE the sandbox check, so an unwanted match (e.g. a
  symlink leaving the module) can be excluded explicitly
- Backward compatible - `exclude` field is optional

#### Import Sandboxing (Module Root)

Every import entry goes through one fixed pipeline: classify (local directory
or Go module) → compute the anchor and confinement boundary → find the files
the import mask matches → drop the ones matched by exclusion masks → return
addressed candidates → immediately before each load, resolve symlinks and
verify the candidate is inside its boundary. The root config goes through the
same final confinement check.

Imports are confined to the **Go module that contains the importing config
file** (module imports — to the module they name). The module root is found
by walking up to the nearest `go.mod`; a config outside any module falls back
to the root config's own directory as the boundary.

- **Absolute filesystem paths are not allowed** in imports or exclusions —
  they are a generation-time error. Everything inside the module is addressed
  relative to the importing file; for a location-independent anchor at the
  module root, use the module's own import path
  (`path: example.com/app/services/x.yaml` from anywhere inside
  `example.com/app`).
- **Classification is deterministic, by form alone.** A multi-segment path
  whose first segment contains a dot names a Go module; everything else is
  local. Single-segment patterns (`base.yaml`, `test_*.yaml`) are always
  local — a module import must name a file inside the module, so it always
  contains a slash. A local directory whose name contains a dot must use the
  `./` spelling (`./assets.d/*.yaml`) — the bare spelling is resolved as a
  module and fails loudly with a hint if that module does not exist. When the
  module exists, the bare spelling always selects it, regardless of any
  same-spelled local path; use `./` to select the local path explicitly.
- **A file outside its boundary is a generation-time error.** After
  exclusion masks are applied, each remaining candidate is resolved through
  symlinks immediately before loading (`EvalSymlinks` on both the boundary
  and the file) and checked against the boundary — any file whose real path
  is outside is an error,
  whether it got there via a `../` chain or a symlink. A symlink whose real
  target is inside the module works like a regular file; to keep an
  out-pointing symlink from failing a broad glob, exclude it (`exclude:
  [./services/fixtures]`) — masks run before the check. Symlink resolution
  is only for the boundary check and cycle identity: cycle detection sees one
  entry per real file, while every import occurrence is loaded independently.
  A config imported through a symlink keeps its addressed location, so its own
  relative imports and `$this` anchor at the symlink's directory, not the
  target's.
- **Cross-module references go through module-path imports.** Only
  `example.com/dep/...` imports may reach another module, and they are bounded
  by the `go.mod` graph. A local candidate whose real path belongs to a nearer,
  nested `go.mod` is rejected even when it remains physically inside the outer
  module. A dependency's own config is likewise confined to its own module —
  it cannot reach into the consumer's filesystem.
- **Module resolution is CWD-independent.** A module is looked up in the
  module context of the importing file (the module containing it, else the
  explicitly supplied module context) via its go.mod graph — including
  `replace` directives, which is the supported way to use a local checkout.
  A config outside any module can use module imports only when moduleContext
  points inside a Go module; its confinement boundary remains independent.

This guarantees generation only ever reads files within the project module or
its declared Go-module dependencies — never arbitrary filesystem paths.

### The `$this` Token

`$this` is replaced with the Go package path of the config file's directory:
- In `type` field: `*$this.Logger` → `*github.com/user/app.Logger`
- In `func` field: `$this.NewLogger` → `github.com/user/app.NewLogger`
- In a tag's `element_type`: `$this.Handler` → `github.com/user/app.Handler`
- In `!go:` args: `!go:$this.DefaultLevel` → `!go:github.com/user/app.DefaultLevel`
- In `!field:!go:` args: `!field:!go:$this.DefaultConfig.Host` → `!field:!go:github.com/user/app.DefaultConfig.Host`
- Eliminates repetitive package paths

The `method` field takes no `$this`: it addresses a service, not a package
(`method: "@factory.Create"`).

## Testing Strategy

Tests are table-driven. There are no golden files — generated output is
asserted on, never diffed against a checked-in copy:
- `integration/integration_test.go`: end-to-end `TestWorkflow` — copies a
  `integration/testdata/<case>/` fixture to a temp dir, generates the
  container, compiles it together with the fixture's `main.go`, runs the
  binary and compares stdout (or asserts the expected generation/runtime
  failure)
- `integration/codegen_test.go`: builds a `di.Config` in Go, calls
  `pipeline.Emit`, and asserts on substrings that must (or must not) appear
  in the generated source, or on the generation error
- `generator/*_test.go`: unit tests for rendering helpers (import manager,
  identifiers, inliner, build-tag header)
- `ir/*_test.go`: IR phase validation and transformation tests
- `yaml/*_test.go`: config loading and import resolution tests

When updating generator behavior:
1. Run tests to see failures
2. Review generated output carefully
3. Adjust the asserted substrings, or add a `integration/testdata/` fixture
   when the change is worth compiling and running
4. Regenerate the demo app in its own repository

Each `integration/testdata/<case>/` directory commits a stub
`container_gen.go` so the fixture's `main.go` resolves in an IDE; the real
container is generated into a temp dir during the test and never overwrites
it.

## Commit Style

This project uses short, imperative, unscoped commit messages:
- ✅ "Fix circular dependency detection"
- ✅ "Add support for variadic constructors"
- ✅ "Regenerate examples"
- ❌ "feat(ir): add circular dependency detection"

## Key Design Principles

1. **Fail at generation time, not runtime**: All type errors, missing dependencies, and circular references are caught during code generation
2. **No runtime reflection**: Generated code uses direct type assertions and function calls
3. **Explicit over implicit**: No autowiring—all dependencies must be explicitly configured
4. **Type safety**: Constructor signatures are strictly validated, no `any` or `interface{}` service types
5. **Deterministic output**: Generated code is consistent and reproducible

## Code Style

- Do not create package-level helper functions that are called from only one place. If the helper belongs to an object, make it a method on that object instead.

## Custom Compiler Passes

For project-specific conventions, create custom passes:

```go
type MyPass struct{}

func (p *MyPass) Name() string { return "my-pass" }

func (p *MyPass) Process(cfg *di.Config) (*di.Config, error) {
    // Mutate cfg (add services, modify args, etc.)
    return cfg, nil
}
```

Use in custom generator:
```go
// tools/gendi/main.go
func main() {
    // Always-included passes
    passes := []di.Pass{&MyPass{}}
    // Selectable passes (filtered by --enable-pass flag)
    selectablePasses := []di.Pass{}
    cmd.Run(flag.CommandLine, passes, selectablePasses)
}
```

See `github.com/gendi-org/gendi-example-app` (`tools/gendi/`) for a real generator wiring built-in and custom passes.

## Common Patterns

### Reading Configuration
```go
opts := pipeline.Options{
    Out:     "./internal/di",
    Package: "di",
}
opts.Finalize()

boundary, err := yaml.DefaultBoundary("gendi.yaml")
cfg, err := yaml.LoadConfig("gendi.yaml", boundary, opts.ModuleRoot)
cfg, err = di.ApplyPasses(cfg, passes)
```

### Generating Container
```go
code, err := pipeline.Emit(cfg, opts)
```

### Generated Container Usage
```go
container := di.NewContainer(nil) // nil uses default parameters
service, err := container.GetServiceName()
```

## Generated File Conventions

- Generated files follow `*_gen.go` naming
- All contain banner: `// Code generated by gendi; DO NOT EDIT.`
- Never edit generated files manually—modify YAML config or generator instead
