---
title: CLI
weight: 3
description: "The generator's flags, the two built-in selectable passes, and the go:generate line that keeps a container in sync."
---

```bash
go tool gendi [flags]
```

| Flag | Description | |
|------|-------------|---|
| `--config string` | Root YAML configuration file | required |
| `--out string` | Output directory or file | required |
| `--pkg string` | Go package name | required |
| `--container string` | Container struct name | default `Container` |
| `--build-tags string` | Go build tags | used for type resolution *and* emitted as the generated file's `//go:build` header |
| `--enable-pass value` | Enable a specific compiler pass | repeat the flag to enable several |
| `--verbose` | Verbose logging | prints the path of the file that was written |

A successful run is otherwise silent.

## Selectable Passes

The stock binary ships two selectable passes; an unknown name is an error
raised before generation starts:

| `--enable-pass` | Effect |
|-----------------|--------|
| `slog` | Gives every service tagged `slog` with a `channel` attribute its own channel-scoped logger derived from the `logger` service (see [stdlib/README.md](https://github.com/gendi-org/gendi/blob/master/stdlib/README.md#slogpass)) |
| `expose-all` | Marks every service public, so each one gets a getter. For test containers — it overrides explicit `public: false` and disables unreachable-service pruning |

Registering passes of your own means building your own generator binary — see
[Building a Custom Generator](embedding.md).

## Examples

```bash
# Generate into a directory
go tool gendi --config=gendi.yaml --out=./di --pkg=di

# Generate a specific file
go tool gendi --config=gendi.yaml --out=./di/container_gen.go --pkg=di

# Resolve types under a build tag, and emit it as the file's //go:build header
go tool gendi --config=gendi.yaml --out=./di --pkg=di --build-tags=integration
```

Keeping the command next to the package it writes is what makes regeneration
routine:

```go
//go:generate go tool gendi --config=gendi.yaml --out=./di --pkg=di
```
