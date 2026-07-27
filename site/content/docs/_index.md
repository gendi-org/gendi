---
title: Documentation
weight: 1
cascade:
  type: docs
---

gendi reads YAML service definitions and generates a Go container: no runtime
reflection, no autowiring, every dependency resolved and type-checked while the
code is generated.

- [Configuration Reference](configuration.md) — the YAML surface: services,
  parameters, tags, decorators, imports, argument syntax
- [Custom Compiler Passes](custom-passes.md) — transforming the configuration
  before generation
- [Design](design.md) — why gendi is shaped this way and what the generated
  container guarantees

The Go API — `di.Pass`, `parameters.Provider`, `parameters.Caster` — is
documented on [pkg.go.dev](https://pkg.go.dev/github.com/gendi-org/gendi).
