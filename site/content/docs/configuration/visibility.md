---
title: Aliases and Visibility
weight: 6
---

Which services get a getter, what name they answer to, and why a service
you configured may not appear in the generated container at all.

## Service Aliases

Create multiple names for the same service:

```yaml
services:
  logger.impl:
    constructor:
      func: "log.New"

  # String shorthand: the whole service definition is "@target"
  logger: "@logger.impl"

  # Expanded form, needed when the alias also sets public or type
  log:
    alias: "logger.impl"   # the @ prefix is optional here
    public: true
```

Aliases always inherit the target service lifecycle. Do not set `shared` on
an alias; explicit `shared: true` and `shared: false` are both configuration
errors.

## Public Services

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

## Reachability

Public services are the roots of the generated container: only they and the
services reachable from them through constructor arguments are emitted.
Everything else is pruned after validation and never reaches the generated
file.

- Public tags count as roots too — a `public: true` tag keeps every service
  it collects
- A config in which nothing is public generates a container with no services
- Pruning happens after all type checking, so an unreachable service with a
  broken constructor still fails generation
- Importing a large config costs nothing at runtime: the services you do not
  reach are not generated

This is why an imported service can be missing a getter you expected: mark it
`public: true`, reach it from something public, or generate with
`--enable-pass=expose-all`, which makes every service public and thereby
disables pruning.
