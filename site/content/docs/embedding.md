---
title: Building a Custom Generator
weight: 6
---

Passes only run inside a generator binary that registers them. The stock
`gendi` binary registers the built-in ones; to add your own, build a
generator of your own around `cmd.Run`.

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
