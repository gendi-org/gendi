---
title: Constructors
weight: 2
description: "Declaring a constructor as a plain function, a method of another service, or a generic function with explicit type arguments."
---

How a service's constructor may be declared: a plain function, a method
of another service, or a generic function instantiated with explicit type
arguments. How each argument is spelled is covered in
[Arguments](arguments.md).

## Constructor Signatures

A constructor — `func` or `method` — must return exactly one of:

1. `T`
2. `(T, error)`

`T` is any type the container can check statically: concrete types,
pointers, slices, maps, channels, and interfaces with methods.

- A constructor returning `any`/`interface{}`, or a named type whose
  underlying type is an empty interface, is rejected at generation time:
  such a type makes every assignment valid and leaves nothing to verify
- Generic constructors require explicit type arguments, so a bare type
  parameter never becomes a service type
- Argument count and types are validated against the signature

## Method Constructors

Use service methods to construct other services:

```yaml
services:
  factory:
    constructor:
      func: "factory.New"
    shared: true

  processor:
    constructor:
      method: "@factory.CreateProcessor"
      args:
        - "@dependency"
    shared: false
```

Generated code:
```go
func (c *Container) buildProcessor() (*Processor, error) {
    factory, err := c.getFactory()
    if err != nil {
        return nil, err
    }
    dep, err := c.getDependency()
    if err != nil {
        return nil, err
    }
    return factory.CreateProcessor(dep), nil
}
```

## Generic Constructors

Support for Go generics with type arguments:

```yaml
services:
  events:
    constructor:
      func: "github.com/gendi-org/gendi/stdlib.NewChan[github.com/myapp/events.Event]"
      args:
        - 100  # buffer size
    public: true
```

Generated code:
```go
func (c *Container) buildEvents() (chan events.Event, error) {
    return make(chan events.Event, 100), nil
}
```

`stdlib.NewChan` is a real generic function of the `stdlib` package; the
generator recognizes it and emits the equivalent `make` expression through a
dedicated inliner instead of calling it, so the generated file does not import
`stdlib`. A generic constructor from any other package is instantiated and
called normally: `pkg.NewPool[events.Event](100)`.
