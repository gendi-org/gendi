---
title: Services
weight: 1
description: "Defining a service: the constructor, the optional type contract, and every field a service entry accepts."
---

Services are objects constructed and managed by the container.

## Service Definition

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

    # Participation in autoconfigured tags (default: true)
    autoconfigure: true

    # Service aliasing
    alias: "other_service"  # Alias to another service

    # Decoration
    decorates: "base_service"  # Decorate another service
    decoration_priority: 10    # Higher priority decorators wrap first

    # Tagging
    tags:
      - "tag.name"            # String shorthand, no attributes
      - name: "tag.name"      # Every field except 'name' is an attribute
        attribute1: "value1"
        priority: 100
```

A service ID must not end with `.inner` — that suffix is reserved for
decorator expansion.

## The `type` Field

`type` is optional. When omitted, the service type is the constructor's
return type. When present it is a contract: the inferred constructor type
must match it, or generation fails. It is not a conversion — declaring a
supertype does not widen the service.
