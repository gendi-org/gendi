---
title: Tags
weight: 8
description: "Collecting services by tag into a typed slice, sorting them by attribute, and autoconfiguring tags across a configuration."
---

Tags enable collecting multiple services that implement a common interface.

Tagged collections are desugared into a `stdlib.MakeSlice` constructor during
analysis, so a config that declares any tag makes the generator load
`github.com/gendi-org/gendi/stdlib` — the module must be resolvable from the
module being generated into (installing GenDI as a tool dependency covers this).
This holds even for a tag only a compiler pass consumes. The `MakeSlice` call is
inlined into a slice literal, so the generated file itself does not import
`stdlib`.

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
- **`sort_by`**: Attribute name for sorting (incompatible with `autoconfigure`)
- **`public`**: Generate public getter for tagged collection
- **`autoconfigure`**: Automatically tag every service whose type is assignable
  to `element_type`

## Declared and Implicit Tags

A tag does not have to be declared: referencing a name from a service's `tags`
list or from a `!tagged:` argument creates it implicitly. Declaring a tag is
what buys the optional behaviour — only a declared tag can set
`element_type`, `sort_by`, `public`, or `autoconfigure`.

When `element_type` is omitted, it is inferred from the constructor arguments
that consume the tag via `!tagged:name`; if several constructors consume the
same tag, their element types must be compatible. A tag with neither a
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

The attribute is read as an integer (a numeric scalar or a decimal string); a
service whose tag omits it sorts as `0`. Services with equal values are ordered
by service ID. Without `sort_by`, the collection is ordered by service ID.

## Public Tag Getters

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
