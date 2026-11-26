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
    decoration_priority: 20  # Higher priority is applied first

  metrics_decorator:
    constructor:
      func: "metrics.NewDecorator"
      args:
        - "@.inner"
    decorates: logger
    decoration_priority: 10
```

**Execution order:** `metrics(logging(logger))`

## Rules

- Must use `@.inner` in constructor args to receive the decorated service
- Multiple decorators on same service are applied in the descending order of
  `decoration_priority`, as in Symfony. The decorator with the highest
  priority is applied first, so it is the innermost one. The default
  priority is `0`
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
  own ID and keeps its tags. The tags declared on the alias are dropped
- A decorator cannot itself be decorated
