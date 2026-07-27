---
title: Decorators
weight: 7
description: "Wrapping a service with decorators, their priority order, and how @.inner and tags behave inside a chain."
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
    decoration_priority: 20  # Higher priority wraps first
```

**Execution order:** `metrics(logging(logger))`

## Rules

- Must use `@.inner` in constructor args to receive the decorated service
- Multiple decorators on same service are ordered by `decoration_priority` (descending)
- The decorated service's own ID becomes an alias to the outermost decorator,
  so `@logger` resolves to `metrics(logging(logger))`
- The original definition is moved to `<decorator>.inner`, which `@.inner`
  points at (the `.inner` suffix is reserved and cannot be used in service IDs)
- The decorator type must be compatible with the decorated service type
- The base service and each decorator keep their own `shared` setting: a
  shared service may be wrapped by a non-shared decorator and vice versa
- The decorator does not inherit tags of the decorated base service. Tags
  declared on the base stay on its inner, undecorated definition, so tagged
  collections receive the undecorated instance (Symfony semantics) — tag the
  decorator explicitly if the collection should get the decorated one
- A decorator cannot itself be decorated
