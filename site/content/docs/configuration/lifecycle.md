---
title: Service Lifecycle
weight: 5
description: "Shared and non-shared services, the single container mutex, and when the generator drops a cache that could not be observed anyway."
---

Every service is either shared — built once and cached on the container — or
non-shared, built anew on every access. `shared: true` is the default.

## Shared Services (Singletons)

```yaml
services:
  database:
    constructor:
      func: "db.New"
    shared: true  # Default
```

- Created once on first access
- Same instance returned on subsequent calls
- Construction is serialized by the container mutex (see [Locking](#locking))
- Suitable for: databases, HTTP clients, loggers

## Non-Shared Services (Factories)

```yaml
services:
  request_context:
    constructor:
      func: "ctx.New"
    shared: false
```

- New instance created on each access
- No caching; the instance is never stored on the container
- Suitable for: request handlers, temporary objects

## When the Cache Disappears

A shared service that is neither public nor an alias, and that exactly one
service depends on, is generated without a cache field: with a single holder
anchored by a shared ancestor, caching is unobservable, so the generator drops
it. The guarantee that survives is the observable one — that holder still sees
one instance for the lifetime of the container.

This is why reading the generated file can suggest that `shared: true` was
ignored. It was not; there was simply nothing for the cache to protect.

## Locking

The container has exactly one mutex, and only the public getters take it —
for both lifecycles. It guards the shared-instance fields of the whole graph,
not just the service being fetched, so one public getter call builds its
entire subgraph under a single lock.

The internal getters services use to reach each other are lock-free,
including those of shared services: they run only inside a public getter that
already holds the mutex. The mutex is a plain `sync.Mutex` and is not
reentrant, so a constructor must not call a public getter of its own
container — that deadlocks. Take dependencies as constructor arguments
instead.
