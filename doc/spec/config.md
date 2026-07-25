# Configuration

## Root Structure

```yaml
imports: []
parameters: {}
tags: {}
services: {}
```

## Imports

Imports allow configuration decomposition, reuse, and overriding.

Format:
```yaml
imports:
  - ./services/db.yaml
  - ./services/*.yaml
  - path: github.com/acme/billing/config/*.yaml
```

Rules:
- Imports are processed in declaration order
- An import is classified by its form: multi-segment paths whose first
  segment contains a dot are module imports; everything else (single-segment
  names and explicit `./`/`../` prefixes included) is a local path resolved
  relative to the importing file. A module-shaped spelling always selects the
  module when it exists, regardless of a same-spelled local path; use `./` to
  select the local path
- Absolute filesystem paths are not allowed
- Module imports must name a file or glob inside the module
  (`github.com/acme/billing/gendi.yaml`, not `github.com/acme/billing`)
- Module imports resolve through the `go.mod` graph of the importing config's
  module. For a root config outside every Go module, the CLI uses the module
  containing the generated output as a separate lookup context; the root
  config remains confined to its own directory
- Recursive imports are allowed; cyclic imports are forbidden
- Later definitions override earlier ones
- Imports can be a string path or a mapping with `path`
- Glob patterns are supported; matches are expanded in lexicographic order;
  a glob over an existing directory that matches nothing is a silent no-op,
  but a glob whose base directory does not exist is a generation-time error
- Every config, including the root, is confined immediately before loading.
  Imported candidates use the module of the importing file (or the module the
  import names) as their boundary. Exclusion masks are applied first, then
  every remaining candidate is resolved through symlinks and checked against
  its boundary — any file whose real path is outside is a generation-time
  error. A candidate whose real path belongs to a nested Go module is likewise
  rejected unless that module was selected by a module-path import. Symlinks
  whose targets stay inside the module work normally; a config imported
  through a symlink anchors its own relative imports and `$this` at the
  symlink's directory. Every import occurrence is loaded independently; cycle
  detection alone identifies active imports by their real path

Import exclusions:
```yaml
imports:
  - path: ./services/*.yaml
    exclude:
      - ./services/test_*.yaml
      - ./services/internal/*.yaml
```

Exclusions are masks over the files the import found — they never touch the
filesystem. They are addressed like the import and must use the same form: a
local import takes local masks, a module import takes masks inside the same
module. A mask matching a directory on a file's path excludes the subtree; a
mask matching nothing is a silent no-op.

## The `$this` Package Token

`$this` can be used in a service `type`, a constructor `func`, a tag's
`element_type`, and `!go:`/`!field:!go:` arguments to reference the Go package
where the configuration file is located. The `method` field takes no `$this`: a
method constructor addresses a service (`@factory.Create`), not a package.

Usage:
```yaml
services:
  logger:
    type: "*$this.Logger"
    constructor:
      func: "$this.NewLogger"
```

Resolution:
- `$this` is resolved by locating the nearest `go.mod` and computing the package path relative to module root
- Each imported config file has its own `$this` context based on its location

Rules:
- For `type` field: `$this.` can appear anywhere in the type (`*$this.T`, `[]$this.T`, `map[K]$this.V`)
- For the `func` field: `$this` must appear at the start of the path, and may
  also appear inside generic type arguments (`$this.NewPool[$this.Message]`)
- For `!go:` arguments: `$this.` is replaced in the argument value (e.g., `!go:$this.DefaultLevel`)
- For `!field:!go:` arguments: `$this.` is replaced in the `!go:` portion (e.g., `!field:!go:$this.DefaultConfig.Host`)
- If package resolution fails, `$this` remains unchanged and will cause a generation error if the symbol is not found

## Constructor Arguments

| Syntax | Meaning |
| --- | --- |
| `@service.id` | Reference to another service |
| `@.inner` | Inner service in a decorator |
| `%param.name%` | Parameter reference |
| `!tagged:tag.name` | Tagged injection |
| `!spread:@service` | Spread slice into variadic parameters |
| `!spread:!tagged:tag` | Spread tagged collection into variadic parameters |
| `!go:pkg.Symbol` | Go package-level variable or constant |
| `!field:@service.Field` | Field access on a service |
| `!field:!go:pkg.Symbol.Field` | Field access on a Go package-level variable |
| literal | YAML scalar literal |

Argument count and types are strictly validated.

### Spread Operator

The `!spread:` operator unpacks slice values into variadic parameters using Go's `...` syntax.

Example:
```yaml
services:
  all_handlers:
    constructor:
      func: "app.GetHandlers"  # Returns []Handler

  server:
    constructor:
      func: "app.NewServer"     # NewServer(handlers ...Handler)
      args:
        - "!spread:@all_handlers"  # Unpacks []Handler into ...Handler
```

Rules:
- Only one spread allowed per constructor
- Spread must be the last argument
- Inner value must be a slice type
- Target parameter must be variadic
- Works with both service references and tagged injection

## Literals

Constructor arguments can be YAML scalar literals:
- string
- int
- float
- bool
- null
