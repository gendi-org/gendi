---
title: Arguments
weight: 4
---

## Special Tokens

### The `$this` Token

`$this` is replaced with the Go package path of the config file's directory,
found by walking up to the nearest `go.mod` and computing the path relative
to the module root. Every file resolves its own `$this`, so an imported
config anchors at its own location, not at the importing file's. When package
resolution fails, `$this` is left as-is and surfaces as a "symbol not found"
generation error.

**Use in `type` field** — anywhere in the type expression:
```yaml
services:
  logger:
    type: "*$this.Logger"  # → *github.com/myapp/log.Logger
    # also []$this.Handler, map[string]$this.Route, chan $this.Event
```

**Use in `func` field** — at the start of the path, and inside generic type
arguments:
```yaml
services:
  logger:
    constructor:
      func: "$this.NewLogger"  # → github.com/myapp/log.NewLogger

  pool:
    constructor:
      func: "$this.NewPool[$this.Message]"
```

**Use in `method` field:**
```yaml
services:
  child:
    constructor:
      method: "@factory.CreateChild"  # → factory.CreateChild (no $this needed)
```

**Use in `!go:` arguments:**
```yaml
services:
  logger:
    constructor:
      func: "$this.NewLogger"
      args:
        - "!go:$this.DefaultLevel"  # → !go:github.com/myapp/log.DefaultLevel
```

**Use in `!field:!go:` arguments:**
```yaml
services:
  server:
    constructor:
      func: "$this.NewServer"
      args:
        - "!field:!go:$this.DefaultConfig.Host"  # → !field:!go:github.com/myapp.DefaultConfig.Host
```

**Benefits:**
- Eliminates repetitive package paths
- Makes configuration more portable
- Clearer intent (local vs external types)

### Service References

Reference other services using `@service_id`:

```yaml
services:
  database:
    constructor:
      func: "db.New"

  repository:
    constructor:
      func: "repo.New"
      args:
        - "@database"  # Injects database service
```

### Parameter References

Reference parameters using `%param_name%`:

```yaml
parameters:
  db_dsn: "postgres://localhost/app"

services:
  database:
    constructor:
      func: "db.New"
      args:
        - "%db_dsn%"  # Injects parameter value
```

### Tagged References

Reference tagged service collections using `!tagged:tag_name`:

```yaml
services:
  server:
    constructor:
      func: "server.New"
      args:
        - "!tagged:handler"  # Injects []Handler
```

### Inner Service Reference

Reference the decorated service using `@.inner` (decorators only):

```yaml
services:
  decorator:
    constructor:
      func: "decorator.New"
      args:
        - "@.inner"  # The service being decorated
    decorates: base_service
```

`@.inner` must be passed directly as an argument. Composed forms such as `!spread:@.inner` are invalid.

### Spread Operator

Unpack slices into variadic parameters using `!spread:`:

```yaml
services:
  server:
    constructor:
      func: "server.New"  # func New(handlers ...Handler)
      args:
        - "!spread:!tagged:handler"  # Unpacks []Handler into ...Handler
```

**Spread with service reference:**
```yaml
services:
  handlers:
    constructor:
      func: "app.GetHandlers"  # Returns []Handler

  server:
    constructor:
      func: "server.New"  # func New(handlers ...Handler)
      args:
        - "!spread:@handlers"  # Unpacks handlers into ...Handler
```

**Spread Rules:**
- Only one `!spread:` per constructor call
- Must be the last argument
- Inner expression must resolve to a slice
- Target parameter must be variadic

## Argument Syntax

Constructor arguments support multiple syntaxes:

| Syntax | Type | Example |
|--------|------|---------|
| `@service.id` | Service reference | `@database` |
| `@.inner` | Inner service | `@.inner` (decorators only) |
| `%param.name%` | Parameter | `%db_dsn%` |
| `!tagged:tag` | Tagged collection | `!tagged:handler` |
| `!spread:@service` | Spread service slice | `!spread:@handlers` |
| `!spread:!tagged:tag` | Spread tagged slice | `!spread:!tagged:middleware` |
| `!go:pkg.Symbol` | Go package-level var/const | `!go:os.Stdout` |
| `!field:@service.Field` | Service field access | `!field:@config.Host` |
| `!field:!go:pkg.Symbol.Field` | Go symbol field access | `!field:!go:http.DefaultClient.Timeout` |
| `"string"` | String literal | `"localhost"` |
| `123` | Integer literal | `8080` |
| `45.6` | Float literal | `3.14` |
| `true`/`false` | Boolean literal | `true` |
| `null` | Null literal | `null` |

> **Note:** `@service.Method` is not a constructor argument — a method
> constructor is configured via the `constructor.method` field
> (`method: "@factory.Create"`). Used as an argument, `@factory.Create` would
> be parsed as a plain reference to a service whose ID is `factory.Create`.

### Type Compatibility

gendi validates argument types at generation time:

- Service references must match parameter types
- Parameters are converted to target types
- Tagged collections must match slice element types
- Literals are type-checked against parameters

### Variadic Arguments

Variadic constructors accept multiple arguments:

```yaml
services:
  logger:
    constructor:
      func: "log.New"

  named_logger:
    constructor:
      method: "@logger.With"  # func With(args ...any) Logger
      args:
        - "channel"    # First variadic arg
        - "database"   # Second variadic arg
```

Generated code:
```go
return logger.With("channel", "database"), nil
```
