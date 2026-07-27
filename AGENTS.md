This file provides guidance to LLM tools when working with code in this repository.

It holds the rules and constraints that are **not** derivable from the code:
conventions, invariants, and prohibitions. The structure of the pipeline, the
role of each package, and the API surface are deliberately not documented
here — read them from the code, which is where they stay correct.

## Project Overview

**gendi** is a compile-time dependency injection container generator for Go. It reads YAML configuration files and generates type-safe, efficient container code with full compile-time validation—no runtime reflection.

Key characteristics:
- All dependencies are resolved and type-checked during code generation
- Generated code uses direct type assertions
- YAML-based declarative service definitions with imports and overrides
- Support for service lifecycle (shared/non-shared), decorators, tagged injection, and custom compiler passes

## Where Things Are Documented

- **YAML semantics** — services, parameters, tags, decorators, imports and
  their sandboxing, `$this`, argument syntax:
  [`site/content/docs/configuration/`](./site/content/docs/configuration/),
  one page per top-level key plus `arguments.md` for how a single argument is
  spelled. It is the canonical reference; do not restate it here or in code
  comments, and update it in the same commit as a behaviour change
- **The narrative entry point** — install, declare, generate, use, in that
  order: [`site/content/docs/_index.md`](./site/content/docs/_index.md). Every
  listing in it is pasted from a real run of the generator, so a change to the
  emitted code means regenerating the walkthrough, not editing it by hand.
  `TestDocsTour` enforces this against the `docs_tour` fixture — if it fails,
  the page is wrong, not the test
- **What a generation error means**:
  [`site/content/docs/troubleshooting.md`](./site/content/docs/troubleshooting.md),
  keyed by the verbatim message
- **Design rationale, generated container contract, error format**:
  [`site/content/docs/design.md`](./site/content/docs/design.md)
- **Writing compiler passes**: [`site/content/docs/passes.md`](./site/content/docs/passes.md), and
  building a generator around them: [`site/content/docs/embedding.md`](./site/content/docs/embedding.md)
- **Cheat sheet for agents consuming gendi** (kept intentionally
  self-contained, so it duplicates facts on purpose): [`doc/LLM.md`](./doc/LLM.md)
- **Reference wiring of the whole pipeline**: `cmd/cli.go` — load, apply
  passes, emit, write. Read it instead of a prose description
- **Phase order**: `pipeline/build.go` and `ir/builder.go` are ordered lists;
  read them rather than trusting a summary

## Keeping Documentation Consistent

Every topic has exactly one canonical document (the list above). A behaviour
change and the edit to its canonical document belong in the **same commit** —
documentation that lags is indistinguishable from documentation that lies.

Rules that keep it from drifting again:

- **Never copy a file's content into prose.** Link to it instead —
  `stdlib/gendi.yaml`, `config.go`, `literal.go` are read directly, not
  restated. A copy has no test keeping it honest and silently rots
- **Never restate the API.** Types, fields and signatures live in the source
  and on pkg.go.dev. Document *why* and *what is guaranteed*, not *what is
  declared*
- **Never describe package structure or phase order in prose.** `cmd/cli.go`,
  `pipeline/build.go` and `ir/builder.go` are the description
- **Never invent sample output.** Generated code, error messages and CLI help
  in docs must be pasted from a real run — run the generator or the failing
  test and copy what it printed
- **`doc/LLM.md` is the one intentional duplicate.** It is a self-contained
  cheat sheet for agents consuming gendi, so it repeats facts on purpose. It
  has no test guarding it: whenever the canonical document changes, check
  whether LLM.md states the same fact and update it in the same commit

### The documentation site

The pages under `site/content/` are the published site (Hugo + the Hextra
theme, deployed to GitHub Pages by `.github/workflows/docs.yml`). They are
still ordinary Markdown files reviewed in the same commit as the code — the
site is a rendering of them, never a second copy.

- **Every page needs YAML front matter** with `title` and `weight`; the title
  is rendered as the page heading, so the file must not start with its own
  `# Heading`
- **No hand-written tables of contents.** The theme renders one from the
  headings
- **Links between pages stay relative** (`./services.md`) — Hugo resolves
  them to page URLs, so the file reads correctly both on GitHub and on the
  site. This relies on `markup.goldmark.renderHooks.link.useEmbedded: always`
  in `site/hugo.yaml`; without it Hextra's own hook passes the `.md`
  destination through unchanged and the published link 404s
- **Links to files outside `site/content/` must be absolute GitHub URLs.** A
  relative path out of the content tree resolves on GitHub and breaks on the
  site
- `site/` is a **separate Go module** on purpose: the theme is a Hugo module,
  and it must never appear in the `go.mod` that consumers of gendi resolve

### What to touch for a given change

| Change | Also update |
|--------|-------------|
| New or changed argument form | `site/content/docs/configuration/arguments.md` (Argument Syntax table **and** Special Tokens), `gendi.schema.json`, `doc/LLM.md` |
| New or renamed CLI flag | README flag table (copy the description from `cmd/config.go`), `doc/LLM.md` |
| New YAML field or validation rule | the page for that top-level key under `site/content/docs/configuration/`, `gendi.schema.json` |
| Changed generated container API | `site/content/docs/design.md`, the pasted listings in `site/content/docs/_index.md`, `doc/LLM.md`, README if it appears in the quick start |
| Changed the text of a generation error | `site/content/docs/troubleshooting.md` — paste the new message from a failing run, not a paraphrase |
| New service or parameter in `stdlib/gendi.yaml` | `stdlib/README.md` (service section and parameter table — never the YAML itself) |
| Changed import resolution or sandboxing | `site/content/docs/configuration/imports.md`, `doc/LLM.md` if it touches `exclude` or import forms |
| New repository convention or prohibition | this file, not a page under `site/content/` |

### Checks

```bash
# Relative links in every tracked Markdown file resolve to an existing path
git ls-files '*.md' | while read -r f; do
  grep -oE '\]\((\.[^)]+)\)' "$f" | sed 's/^](//; s/)$//' | while read -r link; do
    target="$(dirname "$f")/${link%%#*}"
    [ -e "$target" ] || echo "broken: $f -> $link"
  done
done

# The JSON schema still accepts and rejects what the loader does
go test -run TestConfigSchema .

# The Getting Started listings still match what the generator emits
go test -run TestDocsTour ./integration/

# The site still builds — same command CI runs. `make -C site serve` previews it
make -C site build
```

## Essential Commands

### Building and Testing
```bash
# Run all tests
go test ./...

# Build the CLI generator
go build ./cmd/gendi

# Run the generator manually
go run ./cmd/gendi --config=path/to/gendi.yaml --out=path/to/internal/di --pkg=di
```

### Running a Single Test
```bash
# Run specific test
go test -run TestName ./path/to/package

# Run with verbose output
go test -v -run TestName ./path/to/package
```

### Demo Application
A full, realistic service demonstrating gendi end-to-end lives in the separate
repository `github.com/gendi-org/gendi-example-app`.

## Hard Rules

1. **No `recover()`.** There are no exceptions to this. A panic must crash
   loudly; swallowing it hides a generator bug behind broken output. Handle
   expected failures with errors, and let the unexpected ones kill the
   process.
2. **Deterministic output.** Never iterate a map where the order can reach
   the generated code, an error message, or a test assertion — sort the keys
   with `xmaps.OrderedKeys`. Two runs on the same config must produce
   byte-identical files.
3. **Config errors carry a source location.** Report them with
   `srcloc.Errorf`/`srcloc.WrapError` and the node's `SourceLoc`, so the
   renderer can print the offending YAML line with a caret. An error raised
   without a location silently degrades that output.
4. **Fail at generation time, not runtime.** Type errors, missing
   dependencies, circular references, and unconvertible parameter defaults
   must be caught while generating.
5. **No reflection in generated code.** Generated containers use direct calls
   and type assertions only.
6. **No autowiring.** Every dependency is explicitly configured; inference is
   limited to types, never to wiring.
7. **No `any` service types.** A constructor returning the empty interface is
   rejected — a service needs a type the container can check statically.

## Code Style

- Do not create package-level helper functions that are called from only one
  place. If the helper belongs to an object, make it a method on that object
  instead.
- Keep the public API surface of the runtime packages small: `Provider` and
  `Caster` are the extension points, and `Resolver` is deliberately a
  concrete facade rather than an interface.

## Testing Strategy

Tests are table-driven. There are no golden files — generated output is
asserted on, never diffed against a checked-in copy:
- `integration/integration_test.go`: end-to-end `TestWorkflow` — copies a
  `integration/testdata/<case>/` fixture to a temp dir, generates the
  container, compiles it together with the fixture's `main.go`, runs the
  binary and compares stdout (or asserts the expected generation/runtime
  failure)
- `integration/codegen_test.go`: builds a `di.Config` in Go, calls
  `pipeline.Emit`, and asserts on substrings that must (or must not) appear
  in the generated source, or on the generation error
- `integration/docs_tour_test.go`: `TestDocsTour` reads the Getting Started
  page and checks every Go listing on it against the `docs_tour` fixture —
  source listings against the fixture's files, generated listings against the
  containers the fixture's two configs emit. It is what keeps the walkthrough
  from becoming fiction, so do not weaken it into a substring smoke test; it
  also fails when the page loses its listings, to stop the guard from passing
  on nothing
- `generator/*_test.go`: unit tests for rendering helpers (import manager,
  identifiers, inliner, build-tag header)
- `ir/*_test.go`: IR phase validation and transformation tests
- `yaml/*_test.go`: config loading and import resolution tests

When updating generator behavior:
1. Run tests to see failures
2. Review generated output carefully
3. Adjust the asserted substrings, or add a `integration/testdata/` fixture
   when the change is worth compiling and running
4. Regenerate the demo app in its own repository

Each `integration/testdata/<case>/` directory commits a stub
`container_gen.go` so the fixture's `main.go` resolves in an IDE; the real
container is generated into a temp dir during the test and never overwrites
it.

The compiled fixture binary is named `gendi_fixture_bin` rather than something
obvious like `app`: `go build -o <name>` writes *into* a directory of that name
if one exists, so a fixture that declares a Go package directory colliding with
the binary name silently produces no executable and fails at exec time.

## Generated File Conventions

- Generated files follow `*_gen.go` naming
- All contain banner: `// Code generated by gendi; DO NOT EDIT.`
- Never edit generated files manually—modify YAML config or generator instead

## Commit Style

This project uses short, imperative, unscoped commit messages:
- ✅ "Fix circular dependency detection"
- ✅ "Add support for variadic constructors"
- ✅ "Regenerate examples"
- ❌ "feat(ir): add circular dependency detection"
