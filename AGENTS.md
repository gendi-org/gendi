This file provides guidance to LLM tools when working with code in this repository.

It holds the rules and constraints that are **not** derivable from the code:
conventions, invariants, and prohibitions. The structure of the pipeline, the
role of each package, and the API surface are deliberately not documented
here — read them from the code, which is where they stay correct.

## Project Overview

**gendi** is a compile-time dependency injection container generator for Go. It reads YAML configuration files and generates type-safe, efficient container code with full compile-time validation—no runtime reflection.

Key characteristics:
- All dependencies are resolved and type-checked during code generation
- Generated code uses direct type assertions
- YAML-based declarative service definitions with imports and overrides
- Support for service lifecycle (shared/non-shared), decorators, tagged injection, and custom compiler passes

## Where Things Are Documented

- **YAML semantics** — services, parameters, tags, decorators, imports and
  their sandboxing, `$this`, argument syntax: [`doc/configuration.md`](./doc/configuration.md).
  It is the canonical reference; do not restate it here or in code comments,
  and update it in the same commit as a behaviour change
- **Design rationale, generated container contract, error format**:
  [`doc/design.md`](./doc/design.md)
- **Writing compiler passes**: [`doc/custom-passes.md`](./doc/custom-passes.md)
- **Cheat sheet for agents consuming gendi** (kept intentionally
  self-contained, so it duplicates facts on purpose): [`doc/LLM.md`](./doc/LLM.md)
- **Reference wiring of the whole pipeline**: `cmd/cli.go` — load, apply
  passes, emit, write. Read it instead of a prose description
- **Phase order**: `pipeline/build.go` and `ir/builder.go` are ordered lists;
  read them rather than trusting a summary

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

### Running a Single Test
```bash
# Run specific test
go test -run TestName ./path/to/package

# Run with verbose output
go test -v -run TestName ./path/to/package
```

### Demo Application
A full, realistic service demonstrating gendi end-to-end lives in the separate
repository `github.com/gendi-org/gendi-example-app`.

## Hard Rules

1. **No `recover()`.** There are no exceptions to this. A panic must crash
   loudly; swallowing it hides a generator bug behind broken output. Handle
   expected failures with errors, and let the unexpected ones kill the
   process.
2. **Deterministic output.** Never iterate a map where the order can reach
   the generated code, an error message, or a test assertion — sort the keys
   with `xmaps.OrderedKeys`. Two runs on the same config must produce
   byte-identical files.
3. **Config errors carry a source location.** Report them with
   `srcloc.Errorf`/`srcloc.WrapError` and the node's `SourceLoc`, so the
   renderer can print the offending YAML line with a caret. An error raised
   without a location silently degrades that output.
4. **Fail at generation time, not runtime.** Type errors, missing
   dependencies, circular references, and unconvertible parameter defaults
   must be caught while generating.
5. **No reflection in generated code.** Generated containers use direct calls
   and type assertions only.
6. **No autowiring.** Every dependency is explicitly configured; inference is
   limited to types, never to wiring.
7. **No `any` service types.** A constructor returning the empty interface is
   rejected — a service needs a type the container can check statically.

## Code Style

- Do not create package-level helper functions that are called from only one
  place. If the helper belongs to an object, make it a method on that object
  instead.
- Keep the public API surface of the runtime packages small: `Provider` and
  `Caster` are the extension points, and `Resolver` is deliberately a
  concrete facade rather than an interface.

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

## Generated File Conventions

- Generated files follow `*_gen.go` naming
- All contain banner: `// Code generated by gendi; DO NOT EDIT.`
- Never edit generated files manually—modify YAML config or generator instead

## Commit Style

This project uses short, imperative, unscoped commit messages:
- ✅ "Fix circular dependency detection"
- ✅ "Add support for variadic constructors"
- ✅ "Regenerate examples"
- ❌ "feat(ir): add circular dependency detection"
