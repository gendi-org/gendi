---
title: Configuration Reference
weight: 2
description: "The YAML surface of GenDI: schema validation, the four top-level keys, and a page per concept."
---

Complete reference for GenDI YAML configuration files.

## Schema Validation

Add this line at the top of your YAML files for editor autocomplete and validation:

```yaml
# yaml-language-server: $schema=https://raw.githubusercontent.com/gendi-org/gendi/master/gendi.schema.json
```

Supported editors:
- **VS Code**: Install [YAML extension](https://marketplace.visualstudio.com/items?itemName=redhat.vscode-yaml)
- **IntelliJ IDEA**: Built-in YAML support
- **Vim/Neovim**: Use [yaml-language-server](https://github.com/redhat-developer/yaml-language-server)

For local schema validation:
```yaml
# yaml-language-server: $schema=./gendi.schema.json
```

## Root Structure

A config file has four optional top-level keys and nothing else:

```yaml
imports: []      # other configs to load and merge before this file
parameters: {}   # scalar defaults, injected as %name%
tags: {}         # tag declarations
services: {}     # service definitions
```


Each of those keys has its own page: [Parameters](parameters.md),
[Services](services.md), [Tags](tags.md) and [Imports](imports.md). How an
individual argument is spelled is covered in [Arguments](arguments.md).

## Anchors and Merge Keys

YAML anchors, aliases, and merge keys (`<<`) work at all levels of a file:

```yaml
services:
  base: &handler
    constructor:
      func: example.com/app/http.NewHandler
    public: true
  admin:
    <<: *handler
    public: false   # an explicit key overrides a merged key
```

An alias refers to the last anchor with that name before it. When a merge
key has a sequence of mappings, the first mapping wins for a key that two
mappings share. An anchor is visible only in its own file: an imported file
and the file that imports it cannot refer to the anchors of each other.

## See Also

- [Design](../design.md)
- [Custom Compiler Passes](../passes.md)
- [Standard Library Services](https://github.com/gendi-org/gendi/blob/master/stdlib/README.md)
- [Example App](https://github.com/gendi-org/gendi-example-app)
