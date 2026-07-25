# Configuration Reference

Complete reference for gendi YAML configuration files.

## Schema Validation

Add this line at the top of your YAML files for editor autocomplete and validation:

```yaml
# yaml-language-server: $schema=https://raw.githubusercontent.com/gendi-org/gendi/master/gendi.schema.json
```

Supported editors:
- **VS Code**: Install [YAML extension](https://marketplace.visualstudio.com/items?itemName=redhat.vscode-yaml)
- **IntelliJ IDEA**: Built-in YAML support
- **Vim/Neovim**: Use [yaml-language-server](https://github.com/redhat-developer/yaml-language-server)

For local schema validation:
```yaml
# yaml-language-server: $schema=./gendi.schema.json
```

## Table of Contents

- [Parameters](#parameters)
- [Services](#services)
- [Tags](#tags)
- [Imports](#imports)
- [Special Tokens](#special-tokens)
- [Argument Syntax](#argument-syntax)

## Parameters

Parameters are scalar configuration values injected using `%name%` syntax.
A declaration is just a default value — parameters have no declared type.
The target type is contextual: it comes from the constructor argument the
parameter is injected into, so the same parameter can be requested as
different types at different injection sites.

### Parameter Definition

```yaml
parameters:
  app_name: "MyApp"
  port: 8080
  timeout: 30s
  debug: true
  rate_limit: 99.5
```

Each default must be a plain scalar; null and mapping values are rejected.

### Supported Target Types

The constructor argument a parameter is injected into must have one of these
types (or a named type whose underlying type is one of them — a static
conversion is generated):

| Target | Conversion method | Notes |
|--------|-------------------|-------|
| `string` | `ToString` | numeric values format canonically |
| `bool` | `ToBool` | strings parse via `strconv.ParseBool` |
| `int`, `int8`–`int64` | `ToInt`–`ToInt64` | range-checked; strings parse base 10 |
| `uint`, `uint8`–`uint64` | `ToUint`–`ToUint64` | sign- and range-checked |
| `float32`, `float64` | `ToFloat32`, `ToFloat64` | integers must convert exactly |
| `time.Duration` | `ToDuration` | strings via `time.ParseDuration`, integers as nanoseconds |
| `time.Time` | `ToTime` | strings parse as RFC3339 |

`uintptr` and complex types are not supported. A parameter injected into any
other target type fails generation.

Exact `time.Duration` targets use `ToDuration`; a named type defined as
`type Timeout time.Duration` has underlying `int64` and therefore uses
`ToInt64` — a string default like `"5s"` for such a target fails at
generation time. Use exact `time.Duration` or a numeric default instead.

### Lookup and Casting

At runtime a parameter is resolved in two steps: the provider locates the
raw value (`Provider.Lookup(name)`), then the container's caster converts it
to the injection site's target type. Generated code performs both steps
through a single `parameters.Resolver` facade call (`c.paramsResolver.Int("port")`);
the facade is a concrete struct over `Provider` + `Caster`, so the two
responsibilities stay independently replaceable. The default
`parameters.StandardCaster` rejects lossy conversions: float→integer,
bool→anything, values that overflow the target, inexact integer↔float
conversions, NaN and infinities, and named input types. Cast errors name the
raw value, its type, and the target type; the generated wrapping adds the
parameter name, service ID, and argument index.

A custom caster can override individual conversions:

```go
type LenientCaster struct {
	parameters.StandardCaster
}

func (LenientCaster) ToInt(value any) (int, error) {
	// custom policy
}

container := di.NewContainer(nil, di.WithContainerParameterCaster(LenientCaster{}))
```

Custom casters can build rejection errors with `parameters.NewCastError(value, "int")`
to keep their messages consistent with the standard policy. Every rejection wraps
the `parameters.ErrCannotCast` sentinel, so applications can distinguish cast
failures from missing parameters (`parameters.ErrParameterNotFound`) via `errors.Is`.

### Generation-Time Validation

Declared defaults are validated during generation by executing the real
`StandardCaster` against every usage's target type, so a default like
`timeout: "5x"` injected as `time.Duration` fails generation, not startup.
Values supplied by a runtime provider are checked at construction time.

### Parameter References

Reference parameters in constructor arguments using `%param_name%`:

```yaml
services:
  server:
    constructor:
      func: "server.New"
      args:
        - "%app_name%"
        - "%port%"
```

### Parameter Overrides

Later imports override earlier parameter values:

```yaml
# base.yaml
parameters:
  db_dsn: "postgres://localhost/dev"

# production.yaml
imports:
  - ./base.yaml

parameters:
  db_dsn: "postgres://prod-host/app"  # Overrides base.yaml
```

## Services

Services are objects constructed and managed by the container.

### Service Definition

```yaml
services:
  service_id:
    # Optional: Explicit type (inferred from constructor if omitted)
    type: "github.com/myapp.Service"

    # Constructor configuration
    constructor:
      # Function constructor
      func: "github.com/myapp.NewService"
      # OR method constructor
      method: "@other_service.CreateService"

      # Constructor arguments
      args:
        - "@dependency"           # Service reference
        - "%parameter%"           # Parameter reference
        - "!tagged:tag.name"      # Tagged services
        - "@.inner"               # Inner service (decorators only)
        - "literal string"        # Literal value
        - 123                     # Literal number
        - true                    # Literal bool

    # Lifecycle (default: shared=true)
    shared: true  # Singleton (cached)
    # shared: false creates new instance each time

    # Public API exposure
    public: true  # Generate public getter method

    # Service aliasing
    alias: "other_service"  # Alias to another service

    # Decoration
    decorates: "base_service"  # Decorate another service
    decoration_priority: 10    # Higher priority decorators wrap first

    # Tagging
    tags:
      - name: "tag.name"
        attribute1: "value1"
        priority: 100
```

### Service Lifecycle

**Shared Services (Singletons):**
```yaml
services:
  database:
    constructor:
      func: "db.New"
    shared: true  # Default
```

- Created once on first access
- Same instance returned on subsequent calls
- Thread-safe with mutex locking
- Suitable for: databases, HTTP clients, loggers

**Non-Shared Services (Factories):**
```yaml
services:
  request_context:
    constructor:
      func: "ctx.New"
    shared: false
```

- New instance created on each access
- No caching or locking overhead
- Suitable for: request handlers, temporary objects

### Service Aliases

Create multiple names for the same service:

```yaml
services:
  logger.impl:
    constructor:
      func: "log.New"

  logger:
    alias: "logger.impl"

  log:
    alias: "logger.impl"
```

Aliases always inherit the target service lifecycle. Do not set `shared` on
an alias; explicit `shared: true` and `shared: false` are both configuration
errors.

### Public Services

Generate public getter methods for services:

```yaml
services:
  user_repo:
    constructor:
      func: "repo.NewUserRepository"
    public: true
```

Generated methods:
```go
func (c *Container) GetUserRepo() (*UserRepository, error)
func (c *Container) MustUserRepo() *UserRepository
```

### Method Constructors

Use service methods to construct other services:

```yaml
services:
  factory:
    constructor:
      func: "factory.New"
    shared: true

  processor:
    constructor:
      method: "@factory.CreateProcessor"
      args:
        - "@dependency"
    shared: false
```

Generated code:
```go
func (c *Container) buildProcessor() (*Processor, error) {
    factory, err := c.getFactory()
    if err != nil {
        return nil, err
    }
    dep, err := c.getDependency()
    if err != nil {
        return nil, err
    }
    return factory.CreateProcessor(dep), nil
}
```

### Generic Constructors

Support for Go generics with type arguments:

```yaml
services:
  events:
    constructor:
      func: "github.com/gendi-org/gendi/stdlib.NewChan[github.com/myapp/events.Event]"
      args:
        - 100  # buffer size
    public: true
```

Generated code:
```go
func (c *Container) buildEvents() (chan events.Event, error) {
    return stdlib.NewChan[events.Event](100), nil
}
```

### Service Decorators

Decorators wrap existing services to add behavior:

```yaml
services:
  logger:
    constructor:
      func: "log.New"

  logging_decorator:
    constructor:
      func: "log.NewDecorator"
      args:
        - "@.inner"  # Receives the decorated service
    decorates: logger
    decoration_priority: 10

  metrics_decorator:
    constructor:
      func: "metrics.NewDecorator"
      args:
        - "@.inner"
    decorates: logger
    decoration_priority: 20  # Higher priority wraps first
```

**Execution order:** `metrics(logging(logger))`

**Decorator Rules:**
- Must use `@.inner` in constructor args to receive the decorated service
- Multiple decorators on same service are ordered by `decoration_priority` (descending)
- Service ID of decorator becomes the alias after decoration
- Original service is renamed to `<decorator>.inner`
- The decorator does not inherit tags of the decorated base service

## Tags

Tags enable collecting multiple services that implement a common interface.

### Tag Definition

```yaml
tags:
  handler:
    # Optional: Element type inferred from usage if omitted
    element_type: "github.com/myapp.Handler"

    # Optional: Sort by tag attribute
    sort_by: "priority"

    # Optional: Generate public tag getter
    public: true

    # Optional: Auto-tag services implementing the interface
    autoconfigure: true
```

### Tag Attributes

- **`element_type`**: Go type of tagged services (required when `public: true` or `autoconfigure: true`)
- **`sort_by`**: Attribute name for sorting (incompatible with `autoconfigure`)
- **`public`**: Generate public getter for tagged collection
- **`autoconfigure`**: Automatically tag services implementing the interface type

### Tagged Services

Each tag entry is either a string (shorthand for `{name: "..."}`) or a mapping with `name` and optional attributes:

```yaml
services:
  handler1:
    constructor:
      func: "github.com/myapp.NewHandler1"
    tags:
      - name: "handler"
        priority: 100

  handler2:
    constructor:
      func: "github.com/myapp.NewHandler2"
    tags:
      - name: "handler"
        priority: 200

  handler3:
    constructor:
      func: "github.com/myapp.NewHandler3"
    tags:
      - "handler"  # string shorthand — no attributes

  server:
    constructor:
      func: "github.com/myapp.NewServer"
      args:
        - "!tagged:handler"  # Receives []Handler ordered by service ID
                             # (the handler tag declares no sort_by)
```

### Tag Sorting

When `sort_by` is specified, services are sorted by the tag attribute:

```yaml
tags:
  middleware:
    element_type: "github.com/myapp.Middleware"
    sort_by: "order"  # Sort by "order" attribute

services:
  auth:
    tags:
      - name: "middleware"
        order: 10

  logging:
    tags:
      - name: "middleware"
        order: 1

  metrics:
    tags:
      - name: "middleware"
        order: 5
```

Result: `[auth, metrics, logging]` — descending by the attribute, higher value
first.

The attribute is read as an integer (a numeric scalar or a decimal string); a
service whose tag omits it sorts as `0`. Services with equal values are ordered
by service ID. Without `sort_by`, the collection is ordered by service ID.

### Public Tag Getters

```yaml
tags:
  handler:
    element_type: "github.com/myapp.Handler"
    public: true
```

Generated method:
```go
func (c *Container) GetTaggedWithHandler() ([]Handler, error)
```

### Auto-Configuration

```yaml
tags:
  handler:
    element_type: "github.com/myapp.Handler"
    autoconfigure: true
```

All services whose constructor returns `Handler` interface are automatically tagged with `handler`.

**Rules:**
- `element_type` must be an interface type
- Cannot be combined with `sort_by`
- Services are tagged at IR build time

See `doc/spec/tags.md` for complete autoconfigure specification.

## Imports

Configuration files can import and override other configurations.

### Import Syntax

```yaml
imports:
  - ./base.yaml                     # Relative path
  - ./services/*.yaml               # Glob pattern
  - ./**/gendi.yaml                 # Recursive glob
  - github.com/pkg/stdlib/gendi.yaml # Module import (must name a file)
```

### Import Resolution

- An import is classified by its form: a multi-segment path whose first
  segment contains a dot (`example.com/...`) names a Go module; everything
  else — including single-segment names like `base.yaml` — is a local path.
  For a local directory whose name contains a dot, use the `./` spelling
  (`./assets.d/*.yaml`). A bare module-shaped spelling always selects the
  module when it exists, regardless of a same-spelled local path; use `./` to
  select the local path explicitly
- Absolute filesystem paths are not allowed
- Relative paths resolved from importing file's directory
- Glob patterns expanded using doublestar matching; a glob over an existing
  directory that matches nothing is a silent no-op, but a glob whose base
  directory does not exist is a generation-time error
- Every config, including the root, is confined immediately before loading.
  Imported candidates use the module of the importing file (or the named
  module) as their boundary: after exclusions are applied, each candidate is
  resolved through symlinks and checked against the boundary — a file whose
  real path is outside is a generation-time error (exclude unwanted
  symlinked matches to keep a broad glob loadable). A candidate whose real
  path belongs to a nested Go module is also rejected unless that module was
  selected through a module-path import. Every import occurrence is loaded
  independently and keeps its addressed path, so a config imported through a
  symlink anchors its own relative imports and `$this` at the symlink's
  directory. Cycle detection identifies only active imports by real path
- Imports are merged depth-first in declaration order: each imported file's
  imports are merged before that file, and the importing file is merged after
  all of its imports
- Later definitions override earlier ones
- Services with same ID are replaced completely
- Every occurrence in the import graph participates in the merge. With diamond
  imports where A imports B and then C, and both import D, the merge order is
  D, B, D, C, A. The second occurrence of D therefore re-introduces definitions
  that B overrode. Put final overrides in A, or in a file imported after every
  branch that can re-introduce the original definition

### Import Exclusions

Exclude specific files from glob pattern imports:

```yaml
imports:
  # Load all services except test files and internal files
  - path: ./services/*.yaml
    exclude:
      - ./services/test_*.yaml
      - ./services/internal/*.yaml

  # Glob in subdirectories with exclusions
  - path: ./config/**/*.yaml
    exclude:
      - ./config/**/dev_*.yaml
```

**Exclusion Features:**
- Exclusions are masks over the files the import found — they never touch
  the filesystem themselves
- Supports full glob syntax (`*`, `?`, `[]`, `**`)
- Addressed like the import and must use the same form: a local import takes
  local masks, a module import takes masks inside the same module
  (`example.com/mod/services/skip.yaml`)
- A mask matching a directory on a file's path excludes the whole subtree
- A mask that matches nothing is a silent no-op; only a malformed pattern is
  an error
- Applied before the sandbox check, so unwanted symlinked matches can be
  excluded explicitly

### Import Merging

When merging configurations:

1. **Parameters**: Later values override earlier ones
2. **Tags**: Later definitions override earlier ones
3. **Services**: Later services completely replace earlier ones with same ID

### Module Imports

Import from Go modules:

```yaml
imports:
  - github.com/gendi-org/gendi/stdlib/gendi.yaml
```

The module is located through the `go.mod` graph (including `replace`
directives) and the named file is loaded from it. A module import must name a
file or glob explicitly — a bare module path is an error.

Module lookup uses the module containing the importing config. When the root
config is outside every Go module, the CLI uses the module containing the
generated output as the lookup context while keeping the root config confined
to its own directory. Library callers provide these independently as
`yaml.LoadConfig(path, boundary, moduleContext)`. Resolution never depends on
the process working directory.

### Best Practices

**Recommended structure:**

```
project/
├── gendi.yaml                 # Root (imports only)
├── services/
│   ├── database.yaml
│   ├── http.yaml
│   └── logging.yaml
└── config/
    ├── dev.yaml
    └── prod.yaml
```

**Root file:**
```yaml
imports:
  - ./services/*.yaml
  - ./config/dev.yaml
```

**Benefits:**
- Small, focused configuration files
- Easy to navigate and maintain
- Clear separation of concerns
- Environment-specific overrides

## Special Tokens

### The `$this` Token

`$this` is replaced with the Go package path of the config file's directory.

**Use in `type` field:**
```yaml
services:
  logger:
    type: "*$this.Logger"  # → *github.com/myapp/log.Logger
```

**Use in `func` field:**
```yaml
services:
  logger:
    constructor:
      func: "$this.NewLogger"  # → github.com/myapp/log.NewLogger
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

See `doc/spec/services.md` for complete spread specification.

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

## Complete Example

```yaml
# Import stdlib and base services
imports:
  - github.com/gendi-org/gendi/stdlib/gendi.yaml
  - ./services/base.yaml

# Configuration parameters
parameters:
  app_name: "MyApp"

  db_dsn: "postgres://localhost/myapp"

  http_timeout: "30s"

# Tag definitions
tags:
  handler:
    element_type: "github.com/myapp.Handler"
    sort_by: "priority"
    public: true

# Service definitions
services:
  database:
    constructor:
      func: "github.com/myapp/db.New"
      args:
        - "%db_dsn%"
    shared: true

  user_repo:
    constructor:
      func: "github.com/myapp/repo.NewUserRepository"
      args:
        - "@database"
    shared: true
    public: true

  home_handler:
    constructor:
      func: "github.com/myapp/handlers.NewHome"
      args:
        - "@user_repo"
    tags:
      - name: "handler"
        priority: 10

  api_handler:
    constructor:
      func: "github.com/myapp/handlers.NewAPI"
      args:
        - "@user_repo"
    tags:
      - name: "handler"
        priority: 20

  http_server:
    constructor:
      func: "github.com/myapp/server.New"
      args:
        - "%app_name%"
        - "!spread:!tagged:handler"
    public: true

  # Decorator for logging
  logging_middleware:
    constructor:
      func: "github.com/myapp/middleware.NewLogging"
      args:
        - "@.inner"
        - "@stdlib.logger"
    decorates: http_server
    decoration_priority: 10
```

## See Also

- [Technical Specification](./spec/README.md)
- [Custom Compiler Passes](./custom-passes.md)
- [Standard Library Services](../stdlib/README.md)
- [Example App](https://github.com/gendi-org/gendi-example-app)
