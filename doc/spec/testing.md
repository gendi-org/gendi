# Testing

## Unit Tests

- YAML parsing
- Import resolution and sandboxing
- Constructor validation
- Type inference
- IR phases (auto-tagging, desugaring, validation, pruning, optimization)
- Rendering helpers (import manager, identifiers, inliner)

## Integration Tests

No golden files: generated code is asserted on, not diffed against a
checked-in copy.

- End-to-end: a fixture config is generated into a temp directory, compiled
  together with the fixture's `main.go`, executed, and its stdout compared
  with the expected output — covering decorators, tagged injection, imports
  and overrides, spread, generics, and stdlib services
- Codegen: a config is built in Go and emitted, then the generated source is
  checked for the constructs it must and must not contain
- Failure paths: configs expected to fail are asserted on the generation
  error, and an end-to-end case may instead expect the generated program to
  fail compilation or to fail at runtime
