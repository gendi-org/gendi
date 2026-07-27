---
title: Troubleshooting
weight: 4
---

gendi reports everything it can before your program is built, so most problems
arrive as a generation error rather than a runtime failure. This page collects
the ones you are most likely to meet and what each of them means. Every message
below is copied from a real run, with the project directory shortened to
`/path/to/myapp`.

## How to read a generation error

```
generate: phase *ir.constructorResolverPhase apply: /path/to/myapp/gendi.yaml:6:11: service "server" arg[0]: unknown service "greeter"
 2 |   server:
 3 |     constructor:
 4 |       func: "example.com/myapp/app.NewServer"
 5 |       args:
 6 |         - "@greeter"
               ^
 7 |     public: true
```

Four parts, in order: the phase that rejected the configuration, the exact
position in the YAML file, what was wrong and where in the service definition
(`arg[0]`, `constructor.func`, …), and the offending lines with a caret under
the token.

The phase name is an implementation detail — it tells you *when* the check ran,
not what to fix. Read from the file position onward.

Some errors have no position because they are properties of the graph rather
than of a single node; a dependency cycle is the usual one.

## unknown service "x"

The argument references a service ID that no loaded configuration declares.
Check for a typo, and check that the file declaring it was actually loaded: a
glob that matches nothing under an existing directory is a silent no-op, so a
mistyped import path leaves its services undeclared without complaining. See
[Imports](configuration/imports.md).

## cannot use string literal "oops" as \*example.com/myapp/greet.Greeter

```
generate: phase *ir.constructorResolverPhase apply: /path/to/myapp/gendi.yaml:6:11: service "server" arg[0]: cannot use string literal "oops" as *example.com/myapp/greet.Greeter
```

The argument's type does not match the constructor parameter at that position.
The most common cause is a missing `@`: without it, `greeter` is a plain string
literal rather than a service reference. See [Arguments](configuration/arguments.md)
for what each prefix means.

## symbol NewGreeter not found in example.com/myapp/greet

```
generate: phase *ir.constructorResolverPhase apply: /path/to/myapp/gendi.yaml:4:13: service "greeter" constructor.func: symbol NewGreeter not found in example.com/myapp/greet
```

The package loaded, but it has no such exported function. If the symbol does
exist, check that it is exported, and that the file declaring it is not behind
a build tag the generator was not given — `--build-tags` is used for type
resolution as well as for the generated file's header.

An unresolved `$this` surfaces as this same message — see
[Arguments](configuration/arguments.md) for how it is expanded.

## constructor must not return the empty interface (any)

```
generate: phase *ir.constructorResolverPhase apply: /path/to/myapp/gendi.yaml:4:13: service "thing" constructor.func: constructor must not return the empty interface (any); a service needs a type the container can check statically
```

A service whose type is `any` makes every assignment to it valid, which leaves
the generator nothing to verify — so it is rejected rather than generated.
Return a concrete type or an interface with methods.

## circular dependency: a -> b -> a

```
generate: phase *ir.validatorPhase apply: circular dependency: a -> b -> a
```

The trace lists the whole cycle. Since the container builds every dependency
before its holder, a cycle has no valid construction order, and gendi has no
lazy service or setter injection to hide one — the cycle has to go.

The usual fixes are to extract what both services share into a third service
they both depend on, or to decouple them at the type level: pass the data one
side actually needs instead of the whole service, or let them communicate over
a shared channel built by `stdlib.NewChan`.

## The getter I expected is not in the generated file

Not an error, and not a bug. Only public services get `GetX`/`MustX`, and only
services reachable from a public one are emitted at all. A service that is
neither public nor reachable is pruned after validation.

Mark it `public: true`, reach it from something public, or generate with
`--enable-pass=expose-all` for a test container. See
[Aliases and Visibility](configuration/visibility.md).

## parameter "x": parameter not found

```
service "greeter" arg[0] param "nope": parameter "nope": parameter not found
```

This one is a *runtime* error, and it is the intended behaviour: generation does
not require a parameter to be declared in YAML, because the values a container
runs with come from its provider. A name that no provider answers therefore
cannot be caught before the program runs.

Declare a default under `parameters:` if the container should work with
`NewContainer(nil)`, or make sure the provider you pass covers the name.
See [Parameters](configuration/parameters.md).

## out is required

```
config finalize: out is required
```

`--config`, `--out` and `--pkg` are all required. `--verbose` adds only the
name of the file that was written:

```
generated di/container_gen.go
```

## unknown pass "nope"

```
resolve passes: --enable-pass: unknown pass "nope"
```

Pass names are validated before generation starts. The stock binary ships
`slog` and `expose-all`; anything else has to be registered by a generator you
built yourself — see [Building a Custom Generator](embedding.md).
