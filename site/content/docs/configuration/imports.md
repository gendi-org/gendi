---
title: Imports
weight: 10
---

Configuration files can import and override other configurations.

## Import Syntax

An entry is either a string or a mapping with `path` (and optionally
`exclude`); the two forms can be mixed in one list:

```yaml
imports:
  - ./base.yaml                     # Relative path
  - ./services/*.yaml               # Glob pattern
  - ./**/gendi.yaml                 # Recursive glob
  - github.com/pkg/stdlib/gendi.yaml # Module import (file or glob, never a bare module path)
  - path: ./config/*.yaml           # Mapping form
    exclude: [./config/dev_*.yaml]
```

## Import Resolution

- Entries are processed in declaration order, and a glob's matches are
  expanded in lexicographic order — both matter, because later definitions
  win
- Importing recursively is fine; an import cycle is a generation-time error
- An import is classified by its form: a multi-segment path whose first
  segment contains a dot (`example.com/...`) names a Go module; everything
  else — including single-segment names like `base.yaml` — is a local path.
  For a local directory whose name contains a dot, use the `./` spelling
  (`./assets.d/*.yaml`). A bare module-shaped spelling always selects the
  module when it exists, regardless of a same-spelled local path; use `./` to
  select the local path explicitly
- Absolute filesystem paths are not allowed
- Relative paths resolved from importing file's directory
- Glob patterns expanded using doublestar matching; a glob over an existing
  directory that matches nothing is a silent no-op, but a glob whose base
  directory does not exist is a generation-time error
- Every config, including the root, is confined immediately before loading.
  Imported candidates use the module of the importing file (or the named
  module) as their boundary: after exclusions are applied, each candidate is
  resolved through symlinks and checked against the boundary — a file whose
  real path is outside is a generation-time error (exclude unwanted
  symlinked matches to keep a broad glob loadable). A candidate whose real
  path belongs to a nested Go module is also rejected unless that module was
  selected through a module-path import. Every import occurrence is loaded
  independently and keeps its addressed path, so a config imported through a
  symlink anchors its own relative imports and `$this` at the symlink's
  directory. Cycle detection identifies only active imports by real path
- Imports are merged depth-first in declaration order: each imported file's
  imports are merged before that file, and the importing file is merged after
  all of its imports
- Later definitions override earlier ones
- Services with same ID are replaced completely
- Every occurrence in the import graph participates in the merge. With diamond
  imports where A imports B and then C, and both import D, the merge order is
  D, B, D, C, A. The second occurrence of D therefore re-introduces definitions
  that B overrode. Put final overrides in A, or in a file imported after every
  branch that can re-introduce the original definition

## Import Exclusions

Exclude specific files from glob pattern imports:

```yaml
imports:
  # Load all services except test files and internal files
  - path: ./services/*.yaml
    exclude:
      - ./services/test_*.yaml
      - ./services/internal/*.yaml

  # Glob in subdirectories with exclusions
  - path: ./config/**/*.yaml
    exclude:
      - ./config/**/dev_*.yaml
```

**Exclusion Features:**
- Exclusions are masks over the files the import found — they never touch
  the filesystem themselves
- Supports full glob syntax (`*`, `?`, `[]`, `**`)
- Addressed like the import and must use the same form: a local import takes
  local masks, a module import takes masks inside the same module
  (`example.com/mod/services/skip.yaml`)
- A mask matching a directory on a file's path excludes the whole subtree
- A mask that matches nothing is a silent no-op; only a malformed pattern is
  an error
- Applied before the sandbox check, so unwanted symlinked matches can be
  excluded explicitly

## Import Merging

When merging configurations:

1. **Parameters**: Later values override earlier ones
2. **Tags**: Later definitions override earlier ones
3. **Services**: Later services completely replace earlier ones with same ID

## Module Imports

Import from Go modules:

```yaml
imports:
  - github.com/gendi-org/gendi/stdlib/gendi.yaml
```

The module is located through the `go.mod` graph (including `replace`
directives) and the named file is loaded from it. A module import must name a
file or glob explicitly — a bare module path is an error.

Module lookup uses the module containing the importing config. When the root
config is outside every Go module, the CLI uses the module containing the
generated output as the lookup context while keeping the root config confined
to its own directory. Library callers provide these independently as
`yaml.LoadConfig(path, boundary, moduleContext)`. Resolution never depends on
the process working directory.

## Best Practices

**Recommended structure:**

```
project/
├── gendi.yaml                 # Root (imports only)
├── services/
│   ├── database.yaml
│   ├── http.yaml
│   └── logging.yaml
└── config/
    ├── dev.yaml
    └── prod.yaml
```

**Root file:**
```yaml
imports:
  - ./services/*.yaml
  - ./config/dev.yaml
```

**Benefits:**
- Small, focused configuration files
- Easy to navigate and maintain
- Clear separation of concerns
- Environment-specific overrides
