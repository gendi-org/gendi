---
title: Configuration Reference
weight: 2
---

Complete reference for gendi YAML configuration files.

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

## See Also

- [Design](../design.md)
- [Custom Compiler Passes](../custom-passes.md)
- [Standard Library Services](https://github.com/gendi-org/gendi/blob/master/stdlib/README.md)
- [Example App](https://github.com/gendi-org/gendi-example-app)
