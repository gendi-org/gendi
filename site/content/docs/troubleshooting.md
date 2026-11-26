---
title: Troubleshooting
weight: 6
description: "What each generation error means, keyed by the verbatim message, and what to change to make it go away."
---

GenDI reports everything it can before your program is built, so most problems
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

## unknown field "x"

```
/path/to/myapp/gendi.yaml:5:5: load config: unknown field "publc"
1 | services:
2 |   greeter:
3 |     constructor:
4 |       func: "example.com/myapp/greet.NewGreeter"
5 |     publc: true
        ^
```

The object at that position has no field with this name. Usually the name has
a typo: here `publc` is `public` with a missing letter. GenDI rejects the key
and does not ignore it, so a typo cannot silently remove a setting or a full
section. See [Unknown Fields](configuration/_index.md#unknown-fields) for the
objects that GenDI checks.

## service ID 404 is not a string, quote it

```
/path/to/myapp/gendi.yaml:5:3: load config: service ID 404 is not a string, quote it
1 | services:
2 |   greeter:
3 |     constructor:
4 |       func: "example.com/myapp/greet.NewGreeter"
5 |   404:
      ^
6 |     constructor:
7 |       func: "example.com/myapp/http.NewNotFound"
```

YAML reads the key `404` as an integer, not as a string. Write the key in
quotes: `"404":`. The same error occurs for a parameter name, a tag name, and
a service tag attribute. See [Custom Names](configuration/_index.md#custom-names)
for the keys that GenDI rejects.

## decorator "x" decorates unknown service "y"

```
generate: pass "decorator" failed: /path/to/myapp/gendi.yaml:6:14: decorator "greeter_logging" decorates unknown service "greter"
 2 |   greeter:
 3 |     constructor:
 4 |       func: "example.com/myapp/greet.NewGreeter"
 5 |   greeter_logging:
 6 |     decorates: greter
                  ^
 7 |     constructor:
 8 |       func: "example.com/myapp/greet.NewLogging"
 9 |       args: ["@.inner"]
```

The `decorates` key names a service that no loaded configuration declares.
Usually the ID has a typo: here `greter` is `greeter` with a missing letter.
See [Decorators](configuration/decorators.md).

## alias cannot define tags

```
/path/to/myapp/gendi.yaml:8:5: load config: convert /path/to/myapp/gendi.yaml: service "default_greeter": alias cannot define tags; tag the target "greeter" instead
 4 |       func: "example.com/myapp/greet.NewGreeter"
 5 |   default_greeter:
 6 |     alias: "@greeter"
 7 |     public: true
 8 |     tags:
         ^
 9 |       - handler
```

An alias has no tags of its own, as in Symfony. The loader rejects the key,
also with an empty list. Move the `tags` list to the service that the alias
points to, here `greeter`. If the target is decorated, its tags move to the
decorator. See [Tags](configuration/tags.md) and
[Decorators](configuration/decorators.md).

A config that a compiler pass or Go code builds has no `tags` key. For such a
config, the generator reports the same rule later, with the message
`alias cannot define tags; tag the target service instead`.

## service "x" with tag "t": type A is not assignable to B

```
generate: phase *ir.tagDesugarPhase apply: /path/to/myapp/gendi.yaml:9:12: service "users_logging" with tag "handler": type example.com/myapp/greet.Service is not assignable to example.com/myapp/greet.Handler
 5 |   users:
 6 |     type: "example.com/myapp/greet.Service"
 7 |     constructor:
 8 |       func: "example.com/myapp/greet.NewUsers"
 9 |     tags: [handler]
                ^
10 |   users_logging:
11 |     decorates: users
12 |     constructor:
13 |       func: "example.com/myapp/greet.NewLogging"
```

The `element_type` of the tag is `B`, and the type of service `x` does not
implement it. When the caret points at a different service, the tag moved
there from a decorated service. Here `users` has the tag, and its decorator
`users_logging` gets it, but the decorator returns only `Service`. Change the
type of the decorator, or remove the tag. See
[Decorators](configuration/decorators.md).

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

## load package example.com/myapp/gret: no required module provides package

```
generate: /path/to/myapp/gendi.yaml:4:13: service "greeter" constructor.func: load package example.com/myapp/gret: no required module provides package example.com/myapp/gret; to add it:
	go get example.com/myapp/gret
```

No module of the build provides the package. Usually the import path has a
typo, or the package moved to a different directory. When the path is correct,
add the module that provides the package to `go.mod`.

The position is the first place in the configuration that references the
package. Other places can reference the same package. When the package exists
but does not compile, the message shows the compiler error instead.

## resolve $this: no go.mod found

```
/path/to/wiring/gendi.yaml:4:13: load config: convert /path/to/wiring/gendi.yaml: service "greeter": constructor.func: resolve $this: no go.mod found in /path/to/wiring or any parent directory
```

`$this` is the Go package of the directory of the configuration file. This
directory is not in a Go module, so `$this` has no package. Move the file into
the module, or write the full package path instead of `$this`. A configuration
outside a module loads normally when it does not use `$this`. See
[Arguments](configuration/arguments.md#the-this-token).

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
before its holder, a cycle has no valid construction order, and GenDI has no
lazy service or setter injection to hide one — the cycle has to go.

The usual fixes are to extract what both services share into a third service
they both depend on, or to decouple them at the type level: pass the data one
side actually needs instead of the whole service, or let them communicate over
a shared channel built by `stdlib.NewChan`.

## tagged injection requires []T, map[string]T or iter.Seq2[string, T]

```
generate: phase *ir.constructorResolverPhase apply: /path/to/myapp/gendi.yaml:24:11: service "router" arg[0]: tagged injection requires []T, map[string]T or iter.Seq2[string, T], got chan example.com/myapp/app.Handler
```

The type of the parameter selects the shape of a `!tagged:` collection, and
this parameter type is not one of the three shapes. Change the parameter of
the constructor to a slice, a map with `string` keys, or `iter.Seq2` with
`string` keys. An interface parameter such as `any` also gives this error. See
[Collection Shapes](configuration/tags.md#collection-shapes).

## tagged injection requires string keys

```
generate: phase *ir.constructorResolverPhase apply: /path/to/myapp/gendi.yaml:12:11: service "router" arg[0]: tagged injection requires string keys, got map[example.com/myapp/app.Route]example.com/myapp/app.Handler
```

The key type of a map or a sequence must be `string` exactly. A named string
type such as `type Route string` is not accepted, also when its underlying
type is `string`. Change the key type of the parameter to `string`. See
[Collection Shapes](configuration/tags.md#collection-shapes).

## tag "x": element type A must be identical to B

```
generate: phase *ir.constructorResolverPhase apply: /path/to/myapp/gendi.yaml:16:11: service "router" arg[0]: tag "handler": element type *example.com/myapp/app.UsersHandler must be identical to example.com/myapp/app.Handler: a map or a sequence does not convert its elements
```

A slice converts each element to the element type of the parameter, but a map
and a sequence do not. So the element type of the map or the sequence must be
the element type of the tag. Here the tag holds `*UsersHandler`, and the
parameter is `map[string]Handler`. Make the two types the same: change the
`element_type` of the tag, or the parameter of the constructor. See
[Collection Shapes](configuration/tags.md#collection-shapes).

## tag "x": services "a" and "b" have the same key "k"

```
generate: phase *ir.tagDesugarPhase apply: /path/to/myapp/gendi.yaml:11:13: tag "handler": services "orders" and "users" have the same key "orders"
```

A map or a sequence consumes this tag, or the tag is public, so each service
needs its own key. The key is the `id` attribute of the tag entry, or the
service ID when the entry has no `id`. Here the `id` of `users` is equal to the
ID of the service `orders`. Change one of the two keys. See
[Keys](configuration/tags.md#keys).

When one service declares the tag two times with the same `id`, the message
names that service one time:

```
generate: phase *ir.tagDesugarPhase apply: /path/to/myapp/gendi.yaml:8:13: tag "handler": service "users" has the same key "/users" in two entries
```

Give each entry its own `id`, or remove one of the two entries.

## attribute "id" must be a non-empty string

```
generate: phase *ir.servicePhase apply: /path/to/myapp/gendi.yaml:6:13: service "users": tag "handler": attribute "id" must be a non-empty string, got 404
```

The tag attribute `id` is reserved: it is the key of the service in a map or a
sequence. Its value must be a non-empty string. GenDI checks every tag entry,
also when no consumer uses the keys. Quote a number (`id: "404"`), or use
another attribute name for a value that is not a key. See
[Keys](configuration/tags.md#keys).

## sort_by cannot use the reserved attribute "id"

```
generate: phase *ir.tagPhase apply: /path/to/myapp/gendi.yaml:3:12: tag "handler" sort_by cannot use the reserved attribute "id"
```

`sort_by` needs an integer attribute, and `id` is the string key of a tag
entry. Put the order into another attribute, for example `priority`, and name
it in `sort_by`. See [Tag Sorting](configuration/tags.md#tag-sorting).

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

`--config`, `--out` and `--pkg` are all required — see [CLI](cli.md).

## no required module provides package github.com/gendi-org/gendi/stdlib

```
generate: /path/to/myapp/gendi.yaml:6:12: service "greeter" tag "greeters": tags need the gendi module: load package github.com/gendi-org/gendi/stdlib: no required module provides package github.com/gendi-org/gendi/stdlib; to add it:
	go get github.com/gendi-org/gendi/stdlib
```

A configuration that declares any tag needs the `github.com/gendi-org/gendi`
module to be resolvable *from the module being generated into*, because tagged
collections are desugared to `stdlib.MakeSeq2` during analysis. Installing
GenDI as a tool dependency satisfies this; running a prebuilt binary against a
module that does not require GenDI does not.

The position is the first place in the configuration that uses a tag.

## unknown pass "nope"

```
resolve passes: --enable-pass: unknown pass "nope"
```

Pass names are validated before generation starts. The stock binary ships
`slog` and `expose-all`; anything else has to be registered by a generator you
built yourself — see [Building a Custom Generator](embedding.md).
