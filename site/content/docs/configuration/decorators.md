---
title: Decorators
weight: 7
description: "Wrapping a service with decorators, their priority order, @.inner, and how the tags of a decorated service move to the decorator."
---

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
    decoration_priority: 20  # Higher priority is the outer decorator
```

**Execution order:** `metrics(logging(logger))`

## Rules

- Must use `@.inner` in constructor args to receive the decorated service
- Multiple decorators on same service are applied in the ascending order of
  `decoration_priority`. The decorator with the highest priority is applied
  last, so it is the outermost one. The default priority is `0`. **This
  differs from Symfony**, which applies the highest priority first. In GenDI,
  a higher value is always further out or earlier: the outer decorator, and
  the earlier element of a tag with `sort_by`
- When two decorators have the same priority, the decorator with the lower
  service ID is applied first. Symfony uses the order of definition here;
  GenDI has no such order
- The decorated service's own ID becomes an alias to the outermost decorator,
  so `@logger` resolves to `metrics(logging(logger))`
- The original definition is moved to `<decorator>.inner`, which `@.inner`
  points at (the `.inner` suffix is reserved and cannot be used in service IDs)
- The decorator type must be compatible with the decorated service type
- The base service and each decorator keep their own `shared` setting: a
  shared service may be wrapped by a non-shared decorator and vice versa
- Tags describe a service, not a name. The decorator replaces the decorated
  service, so the tags of that service move to the outermost decorator, as in
  Symfony. A tagged collection gets the decorated instance, and
  `<decorator>.inner` has no tags
- When the outermost decorator declares a tag of the same name, its entries
  replace all entries of the decorated service for that name. A tag on both
  services gives one element in the collection, not two
- Each decorator keeps the tags that it declares. An inner decorator of a
  chain is still a service under its own ID, so its tags stay on it. Symfony
  moves them to the outermost decorator too
- When the decorated service is an alias, no tags move. The decorator
  replaces only that name: the target of the alias stays a service under its
  own ID and keeps its tags. An alias cannot have tags of its own
- A decorator does not change the [key](./tags.md#keys) of the decorated
  service in a map or a sequence. When a tag entry of the outermost decorator
  has no `id`, its key is the ID of the decorated service, as in Symfony. This
  applies to the tags that moved, to the tags of the decorator, and to
  autoconfigured tags. When the decorated service is an alias, the key is the
  ID of the alias
- An inner decorator of a chain is keyed by its own ID. Symfony keys it by the
  ID of the decorated service, but in Symfony it has no tags
- A decorator cannot itself be decorated

## Tags in a chain

This configuration has a chain of two decorators over a service, and a
decorator over an alias:

```yaml
tags:
  handler:   { element_type: "github.com/myapp.Named" }
  auditable: { element_type: "github.com/myapp.Named" }
  clearable: { element_type: "github.com/myapp.Named" }

services:
  users:
    constructor: { func: "github.com/myapp.NewUsers" }
    tags: [handler, auditable]

  users.cache:                  # priority 10: applied first, the inner decorator
    decorates: users
    decoration_priority: 10
    constructor: { func: "github.com/myapp.NewCache", args: ["@.inner"] }
    tags: [clearable]

  users.logging:                # priority 20: the outermost decorator
    decorates: users
    decoration_priority: 20
    constructor: { func: "github.com/myapp.NewLogging", args: ["@.inner"] }
    tags:
      - name: auditable
        level: debug

  orders:
    constructor: { func: "github.com/myapp.NewOrders" }
    tags: [handler]

  orders.api:
    alias: "@orders"

  orders.api.tracing:           # decorates the alias, not orders
    decorates: orders.api
    constructor: { func: "github.com/myapp.NewTracing", args: ["@.inner"] }
```

After decoration, each service has these tags:

| Service | Tags | Reason |
|---------|------|--------|
| `users` | none | It is now an alias to `users.logging` |
| `users.cache.inner` | none | The original `users` definition. Its tags moved to the outermost decorator |
| `users.cache` | `clearable` | Its own tag. An inner decorator is still a service under its own ID, so it keeps its own tags |
| `users.logging` | `handler`, `auditable {level: debug}` | `handler` moved from `users`. Its own `auditable` replaces the `auditable` of `users` |
| `orders` | `handler` | The decorator replaces only the name `orders.api`. `orders` is still a service under its own ID and keeps its tags |
| `orders.api` | none | An alias to `orders.api.tracing`. An alias cannot have tags |
| `orders.api.tracing` | none | It decorates an alias, so no tags move to it |

A consumer of each collection and of each ID gets these instances. The output
comes from the
[`decorator_chain_tagged`](https://github.com/gendi-org/gendi/tree/master/integration/testdata/decorator_chain_tagged)
test fixture:

```
!tagged:handler    orders, logging(cache(users))
!tagged:auditable  logging(cache(users))
!tagged:clearable  cache(users)
@users             logging(cache(users))
@orders.api        tracing(orders)
@orders            orders
```

- `handler` gets the decorated `users`, and the undecorated `orders`.
- `auditable` gets one element: the entry of `users.logging` replaced the
  entry of `users`.
- `clearable` gets `cache(users)`, the middle of the chain. Symfony moves the
  tags of an inner decorator to the outermost one, so in Symfony this
  collection gets `logging(cache(users))`.
- In a map, `handler` has the keys `orders` and `users`, and `clearable` has
  the key `users.cache`.
