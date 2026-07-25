# CLI

## Command

```
gendi
```

## Flags

| Flag | Description |
|----|------------|
| `--config` | Root YAML configuration file |
| `--out` | Output directory or file |
| `--pkg` | Go package name |
| `--container` | Container struct name |
| `--build-tags` | Go build tags |
| `--enable-pass` | Enable a selectable compiler pass by name; repeat for multiple passes; errors on unknown name or if pass is not registered as selectable |
| `--verbose` | Verbose logging |

## Built-in Selectable Passes

The stock binary registers these as selectable (`cmd.BuiltinSelectablePasses`);
custom generators choose their own set:

| Name | Effect |
|----|------------|
| `slog` | Per-service channel logger for services tagged `slog` with a `channel` attribute |
| `expose-all` | Makes every service public; overrides `public: false` and disables unreachable pruning |

## go:generate

```go
//go:generate go tool gendi --config=di.yaml --out=./internal/di --pkg=di
```
