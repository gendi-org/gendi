# Services

## Service Definition

```yaml
services:
  service.id:
    type: "pkg.Type"              # optional
    constructor:
      func: "pkg.NewService"      # or method
      args: []
    shared: true
    public: true
    autoconfigure: true
    decorates: other.service
    decoration_priority: 10
    tags:
      - tag.name                  # string shorthand (name only, no attributes)
      - name: tag.name
        priority: 10              # any field except 'name' becomes an attribute
        custom_field: value
```

Service tags are a list. Each entry is either a string (shorthand for `{name: "..."}`) or a mapping with `name` and optional attributes.

Service ID rules:
- User-defined service IDs must not end with `.inner`; that suffix is reserved for decorator expansion internals

## Service Defaults (`_default`)

`_default` sets default values for `shared`, `public`, and `autoconfigure`:

```yaml
services:
  _default:
    shared: true
    public: false
    autoconfigure: true

  logger:
    type: "*app.Logger"
    constructor:
      func: app.NewLogger
    public: true
```

Rules:
- Only `shared`, `public`, and `autoconfigure` are allowed in `_default`
- Explicit values in individual services override defaults
- Other fields (type, constructor, alias, decorates, decoration_priority, tags) are forbidden in `_default`

## `type` Field

- If omitted, the service type is inferred from the constructor return type
- If present, it is treated as a contract and must match the inferred constructor type

## Constructors

Supported forms:
```yaml
constructor:
  func: "module/pkg.NewX"
```

```yaml
constructor:
  method: "@serviceId.Method"
```

### Allowed Signatures

Constructor must return exactly one of:
1. `T`
2. `(T, error)`
3. `*T`
4. `(*T, error)`

Where `T` is a named concrete type (not `any`, `interface{}`, or a type parameter).

## Decorators

```yaml
service.decorator:
  decorates: base.service
  decoration_priority: 20
```

Rules:
- Base getter returns the outermost decorator
- `@.inner` is only available inside decorators
- `@.inner` must be used as a direct argument; forms like `!spread:@.inner` are invalid
- Decorators are ordered by `decoration_priority`
- Decorator type must be compatible with the decorated service type
- The decorated service and each decorator keep their own `shared` setting;
  a shared service may be wrapped by a non-shared decorator and vice versa
- The decorator does not inherit tags of the decorated base service
- Tags declared on the base service stay on its inner (undecorated) definition,
  so tagged collections receive the undecorated instance (Symfony semantics).
  Tag the decorator explicitly if the collection should get the decorated one

## Aliases

Aliases reference another service without defining a constructor:

```yaml
services:
  logger:
    constructor:
      func: "example.com/app.NewLogger"
    public: true
  logger_alias: "@logger"
```

Aliases have no lifecycle of their own. They always inherit the target
service's `shared` behavior, and setting `shared` on an alias is an error.

Expanded form:
```yaml
services:
  logger_alias:
    alias: "logger"
    public: true
```
