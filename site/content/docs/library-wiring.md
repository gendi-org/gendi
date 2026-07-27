---
title: Shipping Wiring in a Library
weight: 4
---

A library that wants to be easy to wire usually settles for exposing
constructors and a paragraph of README explaining the order to call them in.
The alternative most DI frameworks offer is worse: publish a module object, and
now every consumer of the library depends on the framework, whether they use it
or not.

gendi lets a library ship its wiring as a file. The library's Go API keeps no
trace of it, its dependency graph does not grow, and a consumer who never runs
gendi is unaffected.

## The library side

A config sits next to the code it wires:

```
greetlib/
├── go.mod
├── greet.go
└── gendi.yaml
```

Nothing is added to `go.mod` — the wiring is data, not a dependency:

```
module example.com/greetlib

go 1.25.4
```

```yaml
# gendi.yaml
parameters:
  greetlib.greeting: "Hello"

services:
  greetlib.greeter:
    constructor:
      func: "$this.New"
      args:
        - "%greetlib.greeting%"
```

`$this` is the Go package path of the directory the config lives in, so the
file keeps working whoever imports it and wherever the module is checked out.

## The consumer side

Depend on the library the ordinary way, then import its config by module path:

```yaml
# gendi.yaml
imports:
  - example.com/greetlib/gendi.yaml

parameters:
  greetlib.greeting: "Hola"

services:
  greeter:
    alias: "greetlib.greeter"
    public: true
```

The module is located through the consumer's `go.mod` graph, `replace`
directives included. See [Imports](configuration/imports.md) for the resolution
rules and the sandboxing that goes with them.

Generating produces a container that calls the library directly:

```go
var DefaultContainerParameters = parameters.NewProviderMap(map[string]any{
	"greetlib.greeting": "Hola",
})
```

```go
func (c *Container) buildGreetlibGreeter() (*greetlib.Greeter, error) {
	var zero *greetlib.Greeter
	param0_greetlib_greeting, err := c.paramsResolver.String("greetlib.greeting")
	if err != nil {
		return zero, fmt.Errorf("service %q arg[%d] param %q: %w", "greetlib.greeter", 0, "greetlib.greeting", err)
	}
	return greetlib.New(param0_greetlib_greeting), nil
}
```

```
Hola, world!
```

Two things to notice. The consumer's `greetlib.greeting` replaced the library's
default — a library's parameters are defaults, and the importing file is merged
last. And the generated code imports `example.com/greetlib` and calls its
constructor: nothing about the wiring survives into the running program.

## Conventions

### Namespace every ID and parameter

Service IDs and parameter names share one flat space across every imported
file, and later definitions win silently. Prefix both with the library name —
`greetlib.greeter`, `greetlib.greeting` — the way
[`stdlib/gendi.yaml`](https://github.com/gendi-org/gendi/blob/master/stdlib/gendi.yaml)
does with `stdlib.`. Without a prefix, two libraries that both declare `logger`
will quietly overwrite each other in whichever order they were imported.

### Do not mark anything public

Visibility is the application's decision. `public: true` is also a pruning
root, so a public service in a library is emitted into *every* consumer's
container and can never be pruned away:

```go
func (c *Container) GetGreetlibShouty() (*greetlib.Greeter, error)
func (c *Container) MustGreetlibShouty() *greetlib.Greeter
```

Leave the services internal and let the consumer alias the ones it wants, as
above. See [Aliases and Visibility](configuration/visibility.md).

### Use explicit package paths outside the config's own directory

`$this` follows the directory of the file it appears in, not the module root.
Splitting a library config into `services/*.yaml` and writing `$this.New` there
makes gendi look for a Go package that does not exist:

```
generate: -: no required module provides package example.com/greetlib/services; to add it:
	go get example.com/greetlib/services
```

In a split config, write the package path out in full — which is what
`stdlib/gendi.yaml` does.

### What the consumer needs

- the library as a normal module dependency, so the import resolves through
  `go.mod`
- the `github.com/gendi-org/gendi` module itself, if any config in the graph
  declares a tag — see
  [Troubleshooting](troubleshooting.md#no-required-module-provides-package-githubcomgendi-orggendistdlib)

## What this is not

It is not a plugin mechanism. The config is read at generation time by the
consumer's own run of gendi; the library never executes and never registers
anything. That is the point: the file is inert until someone chooses to
generate against it.
