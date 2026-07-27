---
title: Getting Started
weight: 1
cascade:
  type: docs
---

Everything gendi does happens before your program runs. It reads a YAML file,
resolves every dependency against real Go types, and writes one Go file
containing the container your `main` would otherwise assemble by hand. Nothing
is reflected at runtime and nothing is autowired at generation time: what you
declare is what you get.

This page walks the whole loop once — from an empty project to a running
program. Every step links to the reference page that covers it in full.

## Install

gendi is a generator, so it belongs in your module as a tool dependency:

```bash
go get -tool github.com/gendi-org/gendi/cmd/gendi
```

That records it in `go.mod` and makes it runnable as `go tool gendi`, pinned to
a version like any other dependency.

## Start from real Go code

gendi wires constructors that already exist; it never writes the services
themselves. A constructor is any function returning `T` or `(T, error)`:

```go
// greet/greet.go
package greet

type Greeter struct {
	prefix string
}

func New(prefix string) *Greeter {
	return &Greeter{prefix: prefix}
}

func (g *Greeter) Greet(name string) string {
	return g.prefix + ", " + name + "!"
}
```

## Declare one service

```yaml
# gendi.yaml
services:
  greeter:
    constructor:
      func: "example.com/myapp/greet.New"
      args:
        - "Hello"
    public: true
```

`func` is a full package path followed by the function name — gendi loads that
package and type-checks the call. `public: true` asks for a getter; without it
there is no way into the service from your code.

Generate:

```bash
go tool gendi --config=gendi.yaml --out=./di --pkg=di
```

A successful run prints nothing. `di/container_gen.go` now ends with:

```go
func (c *Container) buildGreeter() (*greet.Greeter, error) {
	return greet.New("Hello"), nil
}

func (c *Container) getGreeter() (*greet.Greeter, error) {
	if c.svc_greeter != nil {
		return c.svc_greeter, nil
	}
	res, err := c.buildGreeter()
	if err != nil {
		return nil, err
	}
	c.svc_greeter = res
	return res, nil
}

func (c *Container) GetGreeter() (*greet.Greeter, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.getGreeter()
}

func (c *Container) MustGreeter() *greet.Greeter {
	res, err := c.GetGreeter()
	if err != nil {
		c.onMustCallFailed("greeter", err)
		panic(err)
	}
	return res
}
```

That is the whole pattern: a `build` function that calls your constructor, a
lock-free internal getter that caches, and a locking public getter in two
flavours. See [Services](configuration/services.md) for every field a service
definition accepts.

## Use it

```go
// main.go
package main

import (
	"fmt"

	"example.com/myapp/di"
)

func main() {
	fmt.Println(di.NewContainer(nil).MustGreeter().Greet("world"))
}
```

```
Hello, world!
```

`NewContainer(nil)` falls back to the parameter defaults compiled into the
container. Every public service gets both `GetX() (T, error)` and `MustX() T`;
`MustX` panics and additionally reports through the container error handler if
one was installed.

## Inject a dependency

A service reference is an argument prefixed with `@`:

```yaml
services:
  greeter:
    constructor:
      func: "example.com/myapp/greet.New"
      args:
        - "Hello"

  server:
    constructor:
      func: "example.com/myapp/app.NewServer"
      args:
        - "@greeter"
    public: true
```

`greeter` is no longer public, and two things follow from that in the
regenerated file:

- there is no `GetGreeter`/`MustGreeter` any more. Public services are the
  roots of the container; a non-public service is reachable only from Go code
  gendi itself generates — see [Aliases and Visibility](configuration/visibility.md)
- `greeter` also lost its cache field, even though services are shared by
  default. With exactly one holder, a cache is unobservable, so the generator
  drops it — see [Service Lifecycle](configuration/lifecycle.md)

Neither is a special case you have to configure. Declaring the dependency was
the whole change; ordering, construction and error propagation are derived.

## Move a value out of the config

An argument in `%percent%` form is a parameter reference:

```yaml
parameters:
  greeting: "Hello"

services:
  greeter:
    constructor:
      func: "example.com/myapp/greet.New"
      args:
        - "%greeting%"
  # ...
```

The YAML value is only a default. It is emitted as a package-level provider:

```go
var DefaultContainerParameters = parameters.NewProviderMap(map[string]any{
	"greeting": "Hello",
})
```

and the constructor argument now goes through the resolver:

```go
func (c *Container) buildGreeter() (*greet.Greeter, error) {
	var zero *greet.Greeter
	param0_greeting, err := c.paramsResolver.String("greeting")
	if err != nil {
		return zero, fmt.Errorf("service %q arg[%d] param %q: %w", "greeter", 0, "greeting", err)
	}
	return greet.New(param0_greeting), nil
}
```

Note which method was emitted: `String`. Parameters have no declared type — the
target type comes from the constructor argument the parameter is injected into,
and the typed call is chosen at generation time.

To supply real values, hand the container a provider instead of `nil`:

```go
custom := di.NewContainer(parameters.NewProviderMap(map[string]any{
	"greeting": "Hola",
}))
fmt.Println(custom.MustServer().Handle("world"))
```

```
Hello, world!
Hola, world!
```

A provider can be a map, your own config struct tagged with `di-param`, or a
composite of several sources — see [Parameters](configuration/parameters.md).

## Keep it in sync

Put the command next to the package it generates and let `go generate` run it:

```go
//go:generate go tool gendi --config=gendi.yaml --out=./di --pkg=di
```

Two runs on the same configuration produce byte-identical output, so the
generated file is safe to commit and reviews as an ordinary diff. Regenerating
is how you find out that a constructor signature changed: the mismatch becomes
a generation error rather than a runtime surprise.

## Where to go next

- [Configuration Reference](configuration/_index.md) — every YAML key, one page
  per concept
- [Compiler Passes](passes.md) — rewriting the configuration before generation
- [Troubleshooting](troubleshooting.md) — what the generation errors mean
- [Design](design.md) — what the container guarantees, and what gendi
  deliberately does not do
