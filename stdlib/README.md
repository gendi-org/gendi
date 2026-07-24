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
    return stdlib.NewChan[myapp.Event](100), nil
}
```

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
slice when called with none). This is primarily the helper the generated
container uses to assemble tagged collections.

```yaml
services:
  handlers:
    constructor:
      func: "github.com/gendi-org/gendi/stdlib.MakeSlice[github.com/myapp.Handler]"
```

## Parameter Overrides

Override default parameters in your configuration:

```yaml
imports:
  - github.com/gendi-org/gendi/stdlib/gendi.yaml

parameters:
  # Override HTTP timeout
  stdlib.http.timeout: "60s"

  # Override log level
  stdlib.slog.level: -4  # Debug

  # Override connection pool settings
  stdlib.http.max_idle_conns: 200
  stdlib.http.max_idle_conns_per_host: 20
  stdlib.http.idle_conn_timeout: "120s"
```

## Default Parameters

The stdlib module defines these parameters:

```yaml
parameters:
  stdlib.http.timeout: "30s"
  stdlib.http.max_idle_conns: 100
  stdlib.http.max_idle_conns_per_host: 10
  stdlib.http.idle_conn_timeout: "90s"
  stdlib.slog.level: 0  # Info
```

## Complete Service Definitions

The stdlib `gendi.yaml` file contains:

```yaml
parameters:
  # HTTP client settings
  stdlib.http.timeout: "30s"
  stdlib.http.max_idle_conns: 100
  stdlib.http.max_idle_conns_per_host: 10
  stdlib.http.idle_conn_timeout: "90s"
  # Logging settings
  stdlib.slog.level: 0  # slog.LevelInfo

services:
  stdlib.http.client:
    constructor:
      func: "github.com/gendi-org/gendi/stdlib.NewHTTPClient"
      args:
        - "%stdlib.http.timeout%"

  stdlib.http.transport:
    constructor:
      func: "github.com/gendi-org/gendi/stdlib.NewHTTPTransport"
      args:
        - "%stdlib.http.max_idle_conns%"
        - "%stdlib.http.max_idle_conns_per_host%"
        - "%stdlib.http.idle_conn_timeout%"

  stdlib.http.client_with_transport:
    constructor:
      func: "github.com/gendi-org/gendi/stdlib.NewHTTPClientWithTransport"
      args:
        - "%stdlib.http.timeout%"
        - "@stdlib.http.transport"

  stdlib.slog.writer:
    constructor:
      func: "github.com/gendi-org/gendi/stdlib.NewSlogWriter"
      args:
        - "!go:os.Stderr"

  stdlib.slog.handler.text:
    constructor:
      func: "github.com/gendi-org/gendi/stdlib.NewSlogTextHandler"
      args:
        - "@stdlib.slog.writer"
        - "%stdlib.slog.level%"

  stdlib.slog.handler.json:
    constructor:
      func: "github.com/gendi-org/gendi/stdlib.NewSlogJSONHandler"
      args:
        - "@stdlib.slog.writer"
        - "%stdlib.slog.level%"

  stdlib.slog.handler: '@stdlib.slog.handler.text'

  stdlib.slog:
    constructor:
      func: "log/slog.New"
      args:
        - "@stdlib.slog.handler"

  stdlib.logger: '@stdlib.slog'
```

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

Automatically wires structured logging into services that follow the slog naming convention. Use it in a custom generator built with `cmd.Run` or `cmd.MustRun`:

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

To make SLogPass selectable via `--enable-pass=slog`, put it in the second parameter:

```go
func main() {
    passes := []gendi.Pass{}
    selectablePasses := []gendi.Pass{
        &stdlib.SLogPass{},
    }
    cmd.MustRun(flag.CommandLine, passes, selectablePasses)
}
```

## See Also

- [Configuration Reference](../doc/configuration.md)
- [API Documentation](https://pkg.go.dev/github.com/gendi-org/gendi/stdlib)
- [Example App](https://github.com/gendi-org/gendi-example-app)
