# Custom Compiler Passes

Compiler passes transform configuration before code generation, enabling project-specific conventions and patterns.

## Table of Contents

- [Overview](#overview)
- [Pass Interface](#pass-interface)
- [CLI Passes](#cli-passes)
- [Creating a Pass](#creating-a-pass)
- [Building a Custom Generator](#building-a-custom-generator)
- [Common Use Cases](#common-use-cases)
- [Best Practices](#best-practices)
- [Complete Example](#complete-example)

## Overview

Compiler passes allow you to programmatically modify the DI configuration before container generation. This enables:

- **Auto-tagging** by naming conventions
- **Service registration** from code analysis
- **Argument transformation** (e.g., adding logging)
- **Validation** of custom constraints
- **Configuration normalization**

### When to Use Passes

Use compiler passes when you need:

- Project-specific conventions (e.g., auto-tag all `*Handler` services)
- Dynamic service registration from external sources
- Custom validation rules beyond type checking
- Configuration preprocessing (e.g., environment variable expansion)
- Code generation from annotations

### When NOT to Use Passes

Don't use passes for:

- Simple configuration changes (use YAML imports/overrides instead)
- Type transformations (use decorators instead)
- Runtime behavior modification (implement in your services)

## Pass Interface

All compiler passes implement the `Pass` interface:

```go
package di

type Pass interface {
    Name() string
    Process(cfg *Config) (*Config, error)
}
```

### Interface Methods

**`Name() string`**
- Returns a unique identifier for the pass
- Used in error messages and logging
- Should be lowercase with hyphens (e.g., `"auto-tag"`)

**`Process(cfg *Config) (*Config, error)`**
- Receives the current configuration
- Returns the transformed configuration
- Returns an error if transformation fails
- **Mutates the config in place** and returns it for chaining

## CLI Passes

Custom generator binaries built with `cmd.Run` or `cmd.MustRun` register two types of passes:

- **Always-included passes**: Passed as the first `passes` parameter, always run
- **Selectable passes**: Passed as the second `selectablePasses` parameter, filtered by `--enable-pass` flag

Pass names come from `Name()`. If the same pass name is registered more than once, only the first included pass runs.

`cmd.BuiltinSelectablePasses()` returns the set the stock `gendi` binary
registers — `stdlib.SLogPass` (`slog`) and `di.ExposeAllPass` (`expose-all`);
include it in your own selectable list to keep those available:

```go
selectablePasses := append(cmd.BuiltinSelectablePasses(), &passes.ValidationPass{})
```

`cmd.Run` validates pass flags before generation and returns an error if a name passed to `--enable-pass` does not match any registered selectable pass.

Use `di.Pass` when calling `di.ApplyPasses`, `cmd.Generate`, `cmd.Run`, or `cmd.MustRun`.

## Creating a Pass

### Basic Pass Structure

```go
package passes

import (
    di "github.com/gendi-org/gendi"
)

type MyPass struct {
    // Optional: configuration fields
}

func (p *MyPass) Name() string {
    return "my-pass"
}

func (p *MyPass) Process(cfg *di.Config) (*di.Config, error) {
    // Transform cfg
    // Return modified config or error
    return cfg, nil
}
```

The worked examples below fill this skeleton in — see
[Auto-Tagging by Convention](#auto-tagging-by-convention) for a complete pass.

### Modifying the Configuration

The `Config` struct contains three maps:

```go
type Config struct {
    Parameters map[string]Parameter
    Tags       map[string]Tag
    Services   map[string]Service
}
```

**Safe modification pattern:**

```go
func (p *MyPass) Process(cfg *di.Config) (*di.Config, error) {
    // Iterate over services
    for id, svc := range cfg.Services {
        // Modify service
        svc.Tags = append(svc.Tags, di.ServiceTag{
            Name: "my-tag",
        })

        // Write back to map (services are value types)
        cfg.Services[id] = svc
    }

    return cfg, nil
}
```

**Important:** Services, parameters, and tags are value types. Always write back to the map after modification.

### Adding Services

```go
func (p *MyPass) Process(cfg *di.Config) (*di.Config, error) {
    cfg.Services["new_service"] = di.Service{
        Constructor: di.Constructor{
            Func: "github.com/myapp.NewService",
            Args: []di.Argument{
                {Kind: di.ArgLiteral, Literal: di.NewStringLiteral("value")},
            },
        },
        Shared: true,
    }

    return cfg, nil
}
```

### Adding Tags

```go
func (p *MyPass) Process(cfg *di.Config) (*di.Config, error) {
    cfg.Tags["my-tag"] = di.Tag{
        ElementType: "github.com/myapp.Handler",
        SortBy:      "priority",
    }

    return cfg, nil
}
```

### Adding Parameters

```go
func (p *MyPass) Process(cfg *di.Config) (*di.Config, error) {
    cfg.Parameters["new_param"] = di.Parameter{
        Value: di.NewStringLiteral("default-value"),
    }

    return cfg, nil
}
```

## Building a Custom Generator

To use custom passes, create a custom generator binary:

### 1. Create Generator Package

**tools/gendi/main.go:**
```go
package main

import (
    "flag"
    "fmt"
    "os"

    di "github.com/gendi-org/gendi"
    "github.com/gendi-org/gendi/cmd"
    "github.com/myapp/internal/passes"
)

func main() {
    // Define always-included custom passes
    customPasses := []di.Pass{
        &passes.AutoTagPass{},
    }

    // Define selectable passes (filtered by --enable-pass flag)
    selectablePasses := []di.Pass{
        &passes.ValidationPass{},
    }

    // Run gendi with custom passes
    if err := cmd.Run(flag.CommandLine, customPasses, selectablePasses); err != nil {
        fmt.Fprintf(os.Stderr, "%v\n", err)
        os.Exit(1)
    }
}
```

### 2. Run Custom Generator

```bash
# Build custom generator
go build -o bin/gendi ./tools/gendi

# Run custom generator (AutoTagPass always runs)
./bin/gendi --config=gendi.yaml --out=./di --pkg=di

# Enable ValidationPass via flag
./bin/gendi --config=gendi.yaml --out=./di --pkg=di --enable-pass=validation

# Or use go run
go run ./tools/gendi --config=gendi.yaml --out=./di --pkg=di
```

### 3. Integrate with go:generate

```go
//go:generate go run ./tools/gendi --config=gendi.yaml --out=./di --pkg=di
```

## Common Use Cases

### Auto-Tagging by Convention

Tag services based on naming patterns:

```go
type AutoTagPass struct{}

func (p *AutoTagPass) Name() string {
    return "auto-tag"
}

func (p *AutoTagPass) Process(cfg *di.Config) (*di.Config, error) {
    for id, svc := range cfg.Services {
        // Tag all services ending with ".handler"
        if strings.HasSuffix(id, ".handler") {
            svc.Tags = append(svc.Tags, di.ServiceTag{
                Name: "http.handler",
            })
            cfg.Services[id] = svc
        }

        // Tag all services starting with "middleware."
        if strings.HasPrefix(id, "middleware.") {
            svc.Tags = append(svc.Tags, di.ServiceTag{
                Name: "http.middleware",
            })
            cfg.Services[id] = svc
        }
    }

    return cfg, nil
}
```

### Adding Logging to Services

Automatically inject logger into all services:

```go
type LoggingPass struct{}

func (p *LoggingPass) Name() string {
    return "auto-logging"
}

func (p *LoggingPass) Process(cfg *di.Config) (*di.Config, error) {
    for id, svc := range cfg.Services {
        // Skip logger itself
        if id == "logger" {
            continue
        }

        // Add logger as first argument
        svc.Constructor.Args = append(
            []di.Argument{{Kind: di.ArgServiceRef, Value: "logger"}},
            svc.Constructor.Args...,
        )
        cfg.Services[id] = svc
    }

    return cfg, nil
}
```

### Custom Validation

Validate custom business rules:

```go
type ValidationPass struct{}

func (p *ValidationPass) Name() string {
    return "validation"
}

func (p *ValidationPass) Process(cfg *di.Config) (*di.Config, error) {
    // Ensure all public services are shared
    for id, svc := range cfg.Services {
        if svc.Public && !svc.Shared {
            return nil, fmt.Errorf(
                "service %q is public but not shared", id,
            )
        }
    }

    // Ensure required services exist
    requiredServices := []string{"logger", "database"}
    for _, required := range requiredServices {
        if _, exists := cfg.Services[required]; !exists {
            return nil, fmt.Errorf(
                "required service %q not found", required,
            )
        }
    }

    return cfg, nil
}
```

### Priority-Based Ordering

Automatically assign priorities based on registration order:

```go
type PriorityPass struct{}

func (p *PriorityPass) Name() string {
    return "auto-priority"
}

func (p *PriorityPass) Process(cfg *di.Config) (*di.Config, error) {
    priority := 1000

    for id, svc := range cfg.Services {
        // Add priority to all handler tags
        for i, tag := range svc.Tags {
            if tag.Name == "handler" && tag.Attributes["priority"] == nil {
                if svc.Tags[i].Attributes == nil {
                    svc.Tags[i].Attributes = make(map[string]interface{})
                }
                svc.Tags[i].Attributes["priority"] = priority
                priority += 10
            }
        }

        cfg.Services[id] = svc
    }

    return cfg, nil
}
```

### Environment-Based Configuration

Load configuration from environment:

```go
type EnvPass struct{}

func (p *EnvPass) Name() string {
    return "env-config"
}

func (p *EnvPass) Process(cfg *di.Config) (*di.Config, error) {
    // Override parameters from environment
    for name, param := range cfg.Parameters {
        envKey := "APP_" + strings.ToUpper(
            strings.ReplaceAll(name, ".", "_"),
        )

        if envValue := os.Getenv(envKey); envValue != "" {
            param.Value = di.NewStringLiteral(envValue)
            cfg.Parameters[name] = param
        }
    }

    return cfg, nil
}
```

## Best Practices

### 1. Keep Passes Focused

Each pass should do one thing well:

```go
// ✅ Good: Focused pass
type AutoTagHandlerPass struct{}

// ❌ Bad: Does too much
type AutoConfigureEverythingPass struct{}
```

### 2. Document Pass Behavior

Add clear documentation:

```go
// AutoTagPass automatically tags services by naming convention:
// - Services ending with ".handler" → "http.handler" tag
// - Services starting with "middleware." → "http.middleware" tag
type AutoTagPass struct{}
```

### 3. Fail Fast with Clear Errors

Return descriptive errors:

```go
if svc.Constructor.Func == "" && svc.Constructor.Method == "" {
    return nil, fmt.Errorf(
        "service %q: missing constructor (required by auto-tag pass)",
        id,
    )
}
```

### 4. Don't Assume Configuration State

Validate your assumptions:

```go
func (p *MyPass) Process(cfg *di.Config) (*di.Config, error) {
    if cfg.Services == nil {
        cfg.Services = make(map[string]di.Service)
    }

    // Now safe to modify cfg.Services
    // ...
}
```

### 5. Use Source Locations

Add source location tracking for better error messages:

```go
svc.Constructor.SourceLoc = &srcloc.Location{
    File: "auto-generated",
    Line: 0,
}
```

### 6. Test Your Passes

Write unit tests:

```go
func TestAutoTagPass(t *testing.T) {
    cfg := &di.Config{
        Services: map[string]di.Service{
            "home.handler": {
                Constructor: di.Constructor{
                    Func: "app.NewHomeHandler",
                },
            },
        },
    }

    pass := &AutoTagPass{}
    result, err := pass.Process(cfg)

    if err != nil {
        t.Fatalf("unexpected error: %v", err)
    }

    svc := result.Services["home.handler"]
    if len(svc.Tags) != 1 || svc.Tags[0].Name != "http.handler" {
        t.Errorf("expected http.handler tag, got: %v", svc.Tags)
    }
}
```

### 7. Order Matters

Passes run in sequence. Be aware of dependencies:

```go
customPasses := []di.Pass{
    &NormalizationPass{},  // Run first: normalize config
    &AutoTagPass{},        // Then: add tags
    &ValidationPass{},     // Finally: validate result
}
```

If these passes are registered with `cmd.Run`, use `[]di.Pass`.

## Complete Example

See [gendi-example-app](https://github.com/gendi-org/gendi-example-app) (`tools/gendi/`) for a production-ready implementation featuring:

### Custom Pass: Channel Logger

Automatically adds structured logging with channel names to method constructors:

```go
type ChannelLoggerPass struct{}

func (p *ChannelLoggerPass) Name() string {
    return "channel-logger"
}

func (p *ChannelLoggerPass) Process(cfg *di.Config) (*di.Config, error) {
    for id, svc := range cfg.Services {
        // Only process method constructors
        if svc.Constructor.Method == "" {
            continue
        }

        // Extract service name from ID (e.g., "log.database" → "database")
        parts := strings.Split(id, ".")
        if len(parts) < 2 {
            continue
        }
        channel := parts[len(parts)-1]

        // Add channel and channel name as variadic arguments
        svc.Constructor.Args = append(
            svc.Constructor.Args,
            di.Argument{
                Kind:  di.ArgLiteral,
                Literal: di.NewStringLiteral("channel"),
            },
            di.Argument{
                Kind:  di.ArgLiteral,
                Literal: di.NewStringLiteral(channel),
            },
        )

        cfg.Services[id] = svc
    }

    return cfg, nil
}
```

### Running the Example

Clone [gendi-example-app](https://github.com/gendi-org/gendi-example-app), then:

```bash
# Run custom generator
go run ./tools/gendi --config=./cmd/gendi.yaml --out=./cmd --pkg=main

# Run the application
go run ./cmd
```

### Example Output

```
Config before pass:
  Services: 2
  log.database: method constructor with 0 args
  log.auth: method constructor with 0 args

Config after pass:
  Services: 2
  log.database: method constructor with 2 args
  log.auth: method constructor with 2 args

Generated container with 2 services
```

## API Reference

The types a pass works with — `Config`, `Service`, `Constructor`,
`Argument`/`ArgumentKind`, `Tag`, `ServiceTag`, `Parameter` — and the
`NewStringLiteral`/`NewIntLiteral`/`NewFloatLiteral`/`NewBoolLiteral`/`NewNullLiteral`
constructors are declared in `config.go` and `literal.go` of the root `di`
package. Read them there or on
[pkg.go.dev](https://pkg.go.dev/github.com/gendi-org/gendi#Config) rather
than from a copy that can drift.

Two properties worth knowing before writing a pass:

- Every config struct carries an optional `SourceLoc *srcloc.Location`. Keep
  it when you rewrite an entry — it is what puts the offending YAML line into
  generation errors
- `Packages` fields are derived, not authored: the pipeline recomputes them
  after passes run, so a pass never has to maintain them

## See Also

- [Configuration Reference](./configuration.md)
- [Custom Pass Example](https://github.com/gendi-org/gendi-example-app) (`tools/gendi/`)
- [Design](./design.md)
- [API Documentation](https://pkg.go.dev/github.com/gendi-org/gendi)
