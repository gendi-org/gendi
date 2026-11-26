---
title: Tags
weight: 8
description: "Collecting services by tag into a typed slice, map or sequence, keying and sorting them by attribute, and autoconfiguring tags across a configuration."
---

Tags enable collecting multiple services that implement a common interface.

Each tag is desugared into one service during analysis: a sequence of key and
service, built by `stdlib.MakeSeq2`. A config that declares any tag therefore
makes the generator load `github.com/gendi-org/gendi/stdlib` — the module must
be resolvable from the module being generated into (installing GenDI as a tool
dependency covers this). This holds even for a tag only a compiler pass
consumes. The generator inlines the call into a function literal, so the
generated file itself does not import `stdlib`. Each consumer converts the
sequence to the [shape](#collection-shapes) of its parameter.

## Tag Definition

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

## Tag Attributes

- **`element_type`**: Go type of tagged services (required when `public: true` or `autoconfigure: true`)
- **`sort_by`**: Attribute name for sorting (incompatible with `autoconfigure`;
  cannot be `id`)
- **`public`**: Generate a public getter for the tagged collection. The getter
  returns a sequence of key and service
- **`autoconfigure`**: Automatically tag every service whose type is assignable
  to `element_type`

## Declared and Implicit Tags

A tag does not have to be declared: referencing a name from a service's `tags`
list or from a `!tagged:` argument creates it implicitly. Declaring a tag is
what buys the optional behaviour — only a declared tag can set
`element_type`, `sort_by`, `public`, or `autoconfigure`.

When `element_type` is omitted, it is inferred from the constructor arguments
that consume the tag via `!tagged:name` — the `T` of any
[shape](#collection-shapes); if several constructors consume the same tag,
their element types must be compatible. A tag with neither a
declared nor an inferable element type — nothing consumes it — is silently
skipped: no collection is built and the services carrying it keep their tag
without effect. `public` and `autoconfigure` tags always need an explicit
`element_type`, since a public getter has to have a type and autoconfigure
matches against one.

Once the element type is known, every tagged service's type must be
assignable to it, or generation fails naming the service and both types.

## Tagged Services

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

An alias cannot have tags: put them on the target service, as in Symfony.

## Collection Shapes

`!tagged:name` has one syntax. The type of the target parameter selects the
shape of the collection:

| Parameter type | The constructor receives |
|----------------|--------------------------|
| `[]T` | the services, in collection order |
| `map[string]T` | the services by [key](#keys) |
| `iter.Seq2[string, T]` | pairs of key and service, in collection order |

```yaml
services:
  router:
    constructor:
      func: "github.com/myapp.NewRouter"  # func NewRouter(routes map[string]Handler)
      args:
        - "!tagged:handler"
```

- The generator compares the underlying type, so a named type is valid:
  `type Registry map[string]Handler`, or
  `type Routes func(yield func(string, Handler) bool)`. The callback of a
  sequence must be an unnamed function type: `iter.Seq2` does not convert to
  `func(Yield)` with a named `Yield`.
- The key type of a map or a sequence must be `string`. A named string type
  such as `type Route string` is a generation error.
- Any other parameter type is a generation error, also an interface such as
  `any`.
- `!spread:!tagged:name` accepts only the slice shape.
- A slice converts its elements: a tag of `*HandlerA` fills a `[]Handler`
  parameter. A map (the generated code calls `maps.Collect`) and a sequence do
  not convert. Their element type must be identical to the element type of the
  tag.
- The container builds all services of the collection before the constructor
  of the consumer runs. A sequence does not build services when you iterate
  it, and you can iterate it more than one time.

The order of a slice and of a sequence comes from [sorting](#tag-sorting). A
map has no order, so use a sequence when the consumer needs one.

## Keys

The map and the sequence give each service a key:

- The tag attribute `id` is the key. This attribute name is reserved.
- When the tag entry has no `id`, the key is the ID of the service that
  declares the tag.

```yaml
services:
  users:
    constructor:
      func: "github.com/myapp.NewUsersHandler"
    tags:
      - name: "handler"
        id: "/users"   # key "/users"

  orders:
    constructor:
      func: "github.com/myapp.NewOrdersHandler"
    tags:
      - "handler"      # key "orders"
```

**Rules:**
- The value of `id` must be a non-empty string. GenDI checks every tag entry,
  also when no consumer uses the keys
- Keys must be unique in a tag. GenDI checks this when a map or a sequence
  consumes the tag, or when the tag is public. A slice consumer does not see
  the keys, so a duplicate key does not stop it
- A service can declare the same tag more than one time. Each entry is one
  element of the collection, with its own key and its own `sort_by` value.
  Use this to register one service under two keys
- The tags of a [decorated](./decorators.md) service move to the outermost
  decorator, so the key is the ID of that decorator. A new decorator changes
  the key
- An [autoconfigured](#auto-configuration) service has no tag attributes. Its
  key is its service ID

## Tag Sorting

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

The attribute value must be an integer: an integer scalar, a float without a
fraction, or a decimal string. A service whose tag omits the attribute sorts as
`0`. Any other value is a generation error, for example `"first"`, `1.5`,
`true`, or `null`. Services with equal values are ordered by service ID. Without `sort_by`, the collection is ordered by service ID.
Two entries of one service are ordered by their [keys](#keys).
`sort_by: id` is a generation error: the key is a string, not an integer.

## Public Tag Getters

```yaml
tags:
  handler:
    element_type: "github.com/myapp.Handler"
    public: true
```

Generated methods, for a tag whose element type is `app.Handler`:
```go
func (c *Container) GetTaggedWithHandler() (iter.Seq2[string, app.Handler], error)
func (c *Container) MustTaggedWithHandler() iter.Seq2[string, app.Handler]
```

The getter returns pairs of [key](#keys) and service, in collection order.
`for _, h := range seq` reads the services as it reads a slice. For a map, use
`maps.Collect(seq)`.

## Auto-Configuration

```yaml
tags:
  handler:
    element_type: "github.com/myapp.Handler"
    autoconfigure: true
```

Every service whose resolved type implements `Handler` — that is, is assignable
to it — is automatically tagged with `handler`. The constructor does not have to
return the interface itself; a concrete return type that satisfies it is enough.

**Rules:**
- Only a declared tag can autoconfigure; `element_type` is required and must
  be an interface type
- Cannot be combined with `sort_by`
- Services are tagged at IR build time, after decorator expansion, so a
  decorated service participates through its outermost decorator's type
- Aliases, decorator `.inner` services, and services with
  `autoconfigure: false` (set directly or through `_default`) are excluded
- An alias cannot set `autoconfigure`: the flag of its target applies. An
  explicit value on an alias is a configuration error
- Explicit tags are kept; autoconfigure only adds the services that are
  missing, and the resulting collection is ordered by service ID
- An autoconfigured collection may legitimately be empty

With the tag above and a decorated handler, only the decorator is
autoconfigured:

```yaml
services:
  base:
    constructor:
      func: "github.com/myapp.NewBaseHandler"
  decorated:
    constructor:
      func: "github.com/myapp.NewDecoratedHandler"
      args:
        - "@.inner"
    decorates: "base"
```

`decorated` is tagged if its type implements `Handler`; `base` — now an alias
to `decorated` — and the generated `decorated.inner` never participate.
