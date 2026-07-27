---
title: Service Defaults
weight: 9
---

The reserved `_default` entry sets the default `shared`, `public`, and
`autoconfigure` flags for the services declared in the same file:

```yaml
services:
  _default:
    shared: true
    public: false
    autoconfigure: false

  logger:
    constructor:
      func: "$this.NewLogger"
    public: true      # explicit value wins over the default
```

- Applies per file — an imported config keeps its own defaults, and inherits
  none from the importing file
- Only `shared`, `public`, and `autoconfigure` are allowed; `type`,
  `constructor`, `alias`, `decorates`, `decoration_priority`, and `tags` in
  `_default` are configuration errors
- Without `_default`, the built-in defaults are `shared: true`,
  `public: false`, `autoconfigure: true`
- `_default` is not a service and gets no getter
