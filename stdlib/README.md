# Standard Library Factories

Ready-to-use factory functions and service definitions for common Go standard library types.

## Installation

Import the stdlib services in your gendi configuration:

```yaml
imports:
  - github.com/gendi-org/gendi/stdlib/gendi.yaml
```

This imports pre-configured services for HTTP clients, loggers, and I/O.

## Quick Start

```yaml
imports:
  - github.com/gendi-org/gendi/stdlib/gendi.yaml

services:
  my_service:
    constructor:
      func: "github.com/myapp.NewService"
      args:
        - "@stdlib.http.client"  # Pre-configured HTTP client
        - "@stdlib.logger"       # Pre-configured logger
```

## Available Services

The service definitions themselves live in [`gendi.yaml`](./gendi.yaml) next
to this file.

### HTTP Client

**Service ID:** `stdlib.http.client`

Pre-configured HTTP client with 30-second timeout.

```yaml
services:
  api_client:
    constructor:
      func: "github.com/myapp.NewAPIClient"
      args:
        - "@stdlib.http.client"
```

**Configuration:**
- Timeout: `%stdlib.http.timeout%` (default: `30s`)

### HTTP Client with Custom Transport

**Service ID:** `stdlib.http.client_with_transport`

HTTP client with customizable connection pooling.

```yaml
services:
  api_client:
    constructor:
      func: "github.com/myapp.NewAPIClient"
      args:
        - "@stdlib.http.client_with_transport"
```

**Configuration:**
- Timeout: `%stdlib.http.timeout%` (default: `30s`)
- Max idle connections: `%stdlib.http.max_idle_conns%` (default: `100`)
- Max idle per host: `%stdlib.http.max_idle_conns_per_host%` (default: `10`)
- Idle timeout: `%stdlib.http.idle_conn_timeout%` (default: `90s`)

### HTTP Transport

**Service ID:** `stdlib.http.transport`

Standalone HTTP transport for custom clients.

```yaml
services:
  custom_client:
    constructor:
      func: "github.com/myapp.NewCustomHTTPClient"
      args:
        - "@stdlib.http.transport"
```

### Logger (slog)

**Service ID:** `stdlib.slog` (alias: `stdlib.logger`)

Structured logger using `log/slog` with text handler to stderr.

```yaml
services:
  app:
    constructor:
      func: "github.com/myapp.NewApp"
      args:
        - "@stdlib.logger"
```

**Configuration:**
- Log level: `%stdlib.slog.level%` (default: `Info`)

**Available log levels:**
- `Debug` (-4)
- `Info` (0)
- `Warn` (4)
- `Error` (8)

### Log Handlers

**Service IDs:**
- `stdlib.slog.handler.text` - Text format handler
- `stdlib.slog.handler.json` - JSON format handler
- `stdlib.slog.handler` - Alias to the text handler; this is the handler
  `stdlib.slog` is built from, so overriding it switches the default logger's
  format (`stdlib.slog.handler: '@stdlib.slog.handler.json'`)

Custom logger with JSON output:

```yaml
services:
  json_logger:
    constructor:
      func: "log/slog.New"
      args:
        - "@stdlib.slog.handler.json"
```

### I/O Writers

**Service IDs:**
- `stdlib.slog.writer` - Log output writer, backed by `os.Stderr`

Use the `NewSlogWriter` factory to adapt any standard file (`os.Stderr`,
`os.Stdout`, ...) into an `io.Writer` service:

```yaml
services:
  file_logger.writer:
    constructor:
      func: "github.com/gendi-org/gendi/stdlib.NewSlogWriter"
      args:
        - "!go:os.Stdout"
```

## Factory Functions

The stdlib package provides factory functions you can use directly in your service definitions.

### Channels

**`NewChan[T](size int) chan T`**

Creates a buffered channel of any type.

```yaml
services:
  events:
    constructor:
      func: "github.com/gendi-org/gendi/stdlib.NewChan[github.com/myapp.Event]"
      args:
        - 100  # Buffer size
    public: true
```

Generated code:
```go
func (c *Container) buildEvents() (chan myapp.Event, error) {
    return make(chan myapp.Event, 100), nil
}
```

`NewChan` is an ordinary function of this package, but the generator recognizes
it and emits the equivalent `make` expression through a dedicated inliner rather
than calling it — the generated file therefore does not import `stdlib`.

**Use cases:**
- Event channels
- Work queues
- Message passing

### HTTP

**`NewHTTPClient(timeout time.Duration) *http.Client`**

Creates HTTP client with timeout.

```yaml
services:
  fast_client:
    constructor:
      func: "github.com/gendi-org/gendi/stdlib.NewHTTPClient"
      args:
        - 5000000000  # 5 seconds in nanoseconds
```

**`NewHTTPClientWithTransport(timeout time.Duration, transport http.RoundTripper) *http.Client`**

Creates HTTP client with custom transport.

```yaml
services:
  custom_client:
    constructor:
      func: "github.com/gendi-org/gendi/stdlib.NewHTTPClientWithTransport"
      args:
        - "%http_timeout%"
        - "@custom_transport"
```

**`NewHTTPTransport(maxIdleConns, maxIdleConnsPerHost int, idleConnTimeout time.Duration) *http.Transport`**

Creates HTTP transport with connection pooling.

```yaml
services:
  high_perf_transport:
    constructor:
      func: "github.com/gendi-org/gendi/stdlib.NewHTTPTransport"
      args:
        - 200   # Max idle connections
        - 20    # Max idle per host
        - 120000000000  # 120 seconds
```

### Logging (slog)

**`NewSlogTextHandler(w io.Writer, level slog.Level) slog.Handler`**

Creates text format log handler.

```yaml
services:
  text_handler:
    constructor:
      func: "github.com/gendi-org/gendi/stdlib.NewSlogTextHandler"
      args:
        - "@stdlib.slog.writer"
        - 0  # Info level
```

**`NewSlogJSONHandler(w io.Writer, level slog.Level) slog.Handler`**

Creates JSON format log handler.

```yaml
services:
  json_handler:
    constructor:
      func: "github.com/gendi-org/gendi/stdlib.NewSlogJSONHandler"
      args:
        - "!go:os.Stderr"
        - -4  # Debug level
```

To build the `*slog.Logger` itself, call `log/slog.New` directly — no stdlib
wrapper is needed:

```yaml
services:
  custom_logger:
    constructor:
      func: "log/slog.New"
      args:
        - "@custom_handler"
```

### I/O

**`NewSlogWriter(f *os.File) io.Writer`**

Adapts a standard file such as `os.Stderr` or `os.Stdout` into an `io.Writer`.

```yaml
services:
  writer:
    constructor:
      func: "github.com/gendi-org/gendi/stdlib.NewSlogWriter"
      args:
        - "!go:os.Stdout"
```

### Slices

**`MakeSlice[T](items ...T) []T`**

Builds a slice of any type from its variadic arguments (returns an empty
slice when called with none). This is the constructor tagged collections are
desugared to, so generating a config that uses tags requires this package to be
resolvable — but, like `NewChan`, the call is inlined: the container emits a
slice literal (`[]myapp.Handler{arg0, arg1}`) instead of calling `MakeSlice`.

```yaml
services:
  handlers:
    constructor:
      func: "github.com/gendi-org/gendi/stdlib.MakeSlice[github.com/myapp.Handler]"
```

## Parameters

The imported services read these parameters. Override any of them in your own
config — your `parameters` block wins over the imported one:

| Parameter | Default | Read by |
|-----------|---------|---------|
| `stdlib.http.timeout` | `"30s"` | `stdlib.http.client`, `stdlib.http.client_with_transport` |
| `stdlib.http.max_idle_conns` | `100` | `stdlib.http.transport` |
| `stdlib.http.max_idle_conns_per_host` | `10` | `stdlib.http.transport` |
| `stdlib.http.idle_conn_timeout` | `"90s"` | `stdlib.http.transport` |
| `stdlib.slog.level` | `0` (Info) | `stdlib.slog.handler.text`, `stdlib.slog.handler.json` |

```yaml
imports:
  - github.com/gendi-org/gendi/stdlib/gendi.yaml

parameters:
  stdlib.http.timeout: "60s"
  stdlib.slog.level: -4  # Debug
```

A parameter no reachable service injects is pruned from the generated
defaults, so overriding one has an effect only when the service that reads it
ends up in the container.

## Examples

### HTTP Client with Custom Timeout

```yaml
imports:
  - github.com/gendi-org/gendi/stdlib/gendi.yaml

parameters:
  stdlib.http.timeout: "5s"

services:
  api_client:
    constructor:
      func: "github.com/myapp.NewAPIClient"
      args:
        - "@stdlib.http.client"
```

### JSON Logger with Debug Level

```yaml
imports:
  - github.com/gendi-org/gendi/stdlib/gendi.yaml

parameters:
  stdlib.slog.level: -4  # Debug

services:
  logger:
    constructor:
      func: "log/slog.New"
      args:
        - "@stdlib.slog.handler.json"
    public: true
```

### Event Channel

```yaml
services:
  order_events:
    constructor:
      func: "github.com/gendi-org/gendi/stdlib.NewChan[github.com/myapp/orders.OrderEvent]"
      args:
        - 100  # Buffer size
    public: true

  order_processor:
    constructor:
      func: "github.com/myapp/orders.NewProcessor"
      args:
        - "@order_events"
```

### High-Performance HTTP Client

```yaml
imports:
  - github.com/gendi-org/gendi/stdlib/gendi.yaml

parameters:
  stdlib.http.timeout: "60s"
  stdlib.http.max_idle_conns: 500
  stdlib.http.max_idle_conns_per_host: 50

services:
  api_client:
    constructor:
      func: "github.com/myapp.NewAPIClient"
      args:
        - "@stdlib.http.client_with_transport"
```

## Testing with Stdlib

The stdlib services are shared singletons. For testing, override them in your test configuration:

```yaml
# test/gendi.yaml
imports:
  - ../gendi.yaml

services:
  # Override HTTP client with mock
  stdlib.http.client:
    constructor:
      func: "github.com/myapp/mocks.NewHTTPClient"
    shared: true

  # Override logger with no-op
  stdlib.logger:
    constructor:
      func: "github.com/myapp/mocks.NewNoopLogger"
    shared: true
```

## Compiler Passes

The stdlib package also provides compiler passes for use in custom generator binaries.

### SLogPass

**Pass name:** `slog`

Gives each service its own channel-scoped logger. For every service carrying a
`slog` tag with a `channel` attribute, the pass:

1. adds a non-shared service `<service id>.logger` built from
   `method: "@logger.With"` with the arguments `"channel", "<channel>"`;
2. rewrites that service's `@logger` arguments to point at the new
   `<service id>.logger`.

The receiver is the service ID `logger` literally, so the config must define one
(for example `logger: "@stdlib.logger"` when using the stdlib services).
Services without a `slog` tag, or whose tag has no `channel` attribute, are left
untouched.

```yaml
services:
  logger: "@stdlib.logger"

  repository:
    constructor:
      func: "github.com/myapp/repo.New"
      args:
        - "@logger"      # becomes @repository.logger
    tags:
      - name: slog
        channel: database
```

`--enable-pass=slog` already works with the stock `gendi` binary — the pass is
registered as a built-in selectable pass. Register it yourself only in a custom
generator, e.g. to make it always run:

```go
import (
    "flag"
    gendi "github.com/gendi-org/gendi"
    "github.com/gendi-org/gendi/cmd"
    "github.com/gendi-org/gendi/stdlib"
)

func main() {
    // Always-included passes
    passes := []gendi.Pass{
        &stdlib.SLogPass{},
    }
    cmd.MustRun(flag.CommandLine, passes, nil)
}
```

Passing it as the second parameter instead keeps it opt-in behind
`--enable-pass=slog`, the same way the stock binary registers it
(`cmd.BuiltinSelectablePasses`).

## See Also

- [Configuration Reference](../doc/configuration.md)
- [API Documentation](https://pkg.go.dev/github.com/gendi-org/gendi/stdlib)
- [Example App](https://github.com/gendi-org/gendi-example-app)
