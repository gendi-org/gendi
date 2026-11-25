package yaml

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gendi-org/gendi/imprt"
	"github.com/gendi-org/gendi/srcloc"
)

// boundaryFor derives the load boundary a LoadConfig caller must supply for a
// root config, exactly as production callers do.
func boundaryFor(t *testing.T, path string) string {
	t.Helper()
	boundary, err := DefaultBoundary(path)
	if err != nil {
		t.Fatalf("boundary for %s: %v", path, err)
	}
	return boundary
}

// defaultLoader builds a loader with the production resolver, confined to the
// module (or directory) of rootPath.
func defaultLoader(t *testing.T, rootPath string) *ConfigLoaderYaml {
	t.Helper()
	boundary := boundaryFor(t, rootPath)
	resolver, err := imprt.NewResolver(boundary, boundary)
	if err != nil {
		t.Fatalf("new resolver: %v", err)
	}
	return NewConfigLoaderYaml(resolver, NewParser())
}

type stubResolver struct {
	paths map[string][]string
}

func (r stubResolver) ResolveImport(baseDir, importPath string, _ []string) ([]imprt.Candidate, error) {
	if paths, ok := r.paths[importPath]; ok {
		candidates := make([]imprt.Candidate, 0, len(paths))
		for _, path := range paths {
			candidates = append(candidates, imprt.Candidate{
				Path:     path,
				Boundary: baseDir,
			})
		}
		return candidates, nil
	}
	return nil, os.ErrNotExist
}

func writeFile(t *testing.T, dir, name, contents string) string {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte(contents), 0o644); err != nil {
		t.Fatalf("write %s: %v", name, err)
	}
	return path
}

// writeTree writes each relPath->contents entry under dir, creating parent
// directories as needed.
func writeTree(t *testing.T, dir string, files map[string]string) {
	t.Helper()
	for rel, contents := range files {
		full := filepath.Join(dir, rel)
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatalf("mkdir %s: %v", rel, err)
		}
		if err := os.WriteFile(full, []byte(contents), 0o644); err != nil {
			t.Fatalf("write %s: %v", rel, err)
		}
	}
}

func TestLoadDeduplicatesDiamondImports(t *testing.T) {
	dir := t.TempDir()

	rootPath := writeFile(t, dir, "root.yaml", `
imports:
  - path: b
  - path: c
parameters:
  a: "A"
`)
	bPath := writeFile(t, dir, "b.yaml", `
imports:
  - path: d
parameters:
  b: "B"
`)
	cPath := writeFile(t, dir, "c.yaml", `
imports:
  - path: d
parameters:
  c: "C"
`)
	dPath := writeFile(t, dir, "d.yaml", `
parameters:
  d: "D"
`)

	loader := NewConfigLoaderYaml(stubResolver{
		paths: map[string][]string{
			"b": {bPath},
			"c": {cPath},
			"d": {dPath},
		},
	}, NewParser())

	readCount := 0
	origRead := defaultOsReadFile
	defaultOsReadFile = func(path string) ([]byte, error) {
		readCount++
		return os.ReadFile(path)
	}
	defer func() { defaultOsReadFile = origRead }()

	cfg, err := loader.Load(rootPath, boundaryFor(t, rootPath))
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if got, want := len(cfg.Parameters), 4; got != want {
		t.Fatalf("expected %d parameters, got %d", want, got)
	}
	// d is reached through both b and c but resolves to one addressed path, so
	// it is loaded once: root, b, c, d — not d twice. A shared base reached
	// through many paths therefore costs one load, not one per path.
	if readCount != 4 {
		t.Fatalf("expected the shared import to be loaded once (4 reads), got %d", readCount)
	}
}

// Deduplication is keyed by the addressed path, so two symlink aliases of the
// same real file are still loaded independently — each keeps its own directory
// context for relative imports and $this.
func TestLoadDoesNotDeduplicateSymlinkAliases(t *testing.T) {
	dir := t.TempDir()

	realPath := writeFile(t, dir, "d.yaml", `
parameters:
  d: "D"
`)
	aliasPath := filepath.Join(dir, "d_alias.yaml")
	if err := os.Symlink(realPath, aliasPath); err != nil {
		t.Skipf("symlinks unsupported: %v", err)
	}

	rootPath := writeFile(t, dir, "root.yaml", `
imports:
  - path: d
  - path: d_alias
`)

	loader := NewConfigLoaderYaml(stubResolver{
		paths: map[string][]string{
			"d":       {realPath},
			"d_alias": {aliasPath},
		},
	}, NewParser())

	readCount := 0
	origRead := defaultOsReadFile
	defaultOsReadFile = func(path string) ([]byte, error) {
		readCount++
		return os.ReadFile(path)
	}
	defer func() { defaultOsReadFile = origRead }()

	if _, err := loader.Load(rootPath, boundaryFor(t, rootPath)); err != nil {
		t.Fatalf("Load: %v", err)
	}
	// root, d (real spelling), d_alias (symlink spelling): the two spellings of
	// the same real file are distinct addressed paths and both load.
	if readCount != 3 {
		t.Fatalf("expected distinct symlink spellings to load independently (3 reads), got %d", readCount)
	}
}

func TestExcludeBasicGlob(t *testing.T) {
	dir := t.TempDir()
	servicesDir := filepath.Join(dir, "services")
	if err := os.MkdirAll(servicesDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}

	writeFile(t, servicesDir, "app.yaml", `
parameters:
  app: "app"
`)
	writeFile(t, servicesDir, "db.yaml", `
parameters:
  db: "db"
`)
	writeFile(t, servicesDir, "test_helper.yaml", `
parameters:
  test_helper: "test"
`)

	rootPath := writeFile(t, dir, "gendi.yaml", `
imports:
  - path: ./services/*.yaml
    exclude:
      - ./services/test_*.yaml
`)

	loader := defaultLoader(t, rootPath)
	cfg, err := loader.Load(rootPath, boundaryFor(t, rootPath))
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	if _, ok := cfg.Parameters["app"]; !ok {
		t.Error("expected app parameter to be loaded")
	}
	if _, ok := cfg.Parameters["db"]; !ok {
		t.Error("expected db parameter to be loaded")
	}
	if _, ok := cfg.Parameters["test_helper"]; ok {
		t.Error("expected test_helper parameter to be excluded")
	}
}

func TestExcludeMultiplePatterns(t *testing.T) {
	dir := t.TempDir()
	servicesDir := filepath.Join(dir, "services")
	internalDir := filepath.Join(servicesDir, "internal")
	if err := os.MkdirAll(internalDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}

	writeFile(t, servicesDir, "app.yaml", `
parameters:
  app: "app"
`)
	writeFile(t, servicesDir, "test_app.yaml", `
parameters:
  test_app: "test"
`)
	writeFile(t, internalDir, "debug.yaml", `
parameters:
  debug: "debug"
`)

	rootPath := writeFile(t, dir, "gendi.yaml", `
imports:
  - path: ./services/**/*.yaml
    exclude:
      - ./services/test_*.yaml
      - ./services/internal/*.yaml
`)

	loader := defaultLoader(t, rootPath)
	cfg, err := loader.Load(rootPath, boundaryFor(t, rootPath))
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	if _, ok := cfg.Parameters["app"]; !ok {
		t.Error("expected app parameter to be loaded")
	}
	if _, ok := cfg.Parameters["test_app"]; ok {
		t.Error("expected test_app parameter to be excluded")
	}
	if _, ok := cfg.Parameters["debug"]; ok {
		t.Error("expected debug parameter to be excluded")
	}
}

func TestExcludeAllFiles(t *testing.T) {
	dir := t.TempDir()
	servicesDir := filepath.Join(dir, "services")
	if err := os.MkdirAll(servicesDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}

	writeFile(t, servicesDir, "test1.yaml", `
parameters:
  test1: "test1"
`)
	writeFile(t, servicesDir, "test2.yaml", `
parameters:
  test2: "test2"
`)

	rootPath := writeFile(t, dir, "gendi.yaml", `
imports:
  - path: ./services/*.yaml
    exclude:
      - ./services/*.yaml
`)

	loader := defaultLoader(t, rootPath)
	cfg, err := loader.Load(rootPath, boundaryFor(t, rootPath))
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	if len(cfg.Parameters) != 0 {
		t.Errorf("expected no parameters, got %d", len(cfg.Parameters))
	}
}

func TestExcludeNonGlobImport(t *testing.T) {
	dir := t.TempDir()

	writeFile(t, dir, "specific.yaml", `
parameters:
  specific: "specific"
`)

	rootPath := writeFile(t, dir, "gendi.yaml", `
imports:
  - path: ./specific.yaml
    exclude:
      - ./specific.yaml
`)

	loader := defaultLoader(t, rootPath)

	cfg, err := loader.Load(rootPath, boundaryFor(t, rootPath))
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	if len(cfg.Parameters) != 0 {
		t.Errorf("expected no parameters (file excluded), got %d", len(cfg.Parameters))
	}
}

func TestExcludeInvalidPattern(t *testing.T) {
	dir := t.TempDir()
	servicesDir := filepath.Join(dir, "services")
	if err := os.MkdirAll(servicesDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}

	writeFile(t, servicesDir, "app.yaml", `
parameters:
  app: "app"
`)

	rootPath := writeFile(t, dir, "gendi.yaml", `
imports:
  - path: ./services/*.yaml
    exclude:
      - "[invalid"
`)

	loader := defaultLoader(t, rootPath)
	_, err := loader.Load(rootPath, boundaryFor(t, rootPath))
	if err == nil {
		t.Fatal("expected error for invalid exclusion pattern")
	}
}

func TestExcludeBackwardCompatibility(t *testing.T) {
	dir := t.TempDir()
	servicesDir := filepath.Join(dir, "services")
	if err := os.MkdirAll(servicesDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}

	writeFile(t, servicesDir, "app.yaml", `
parameters:
  app: "app"
`)
	writeFile(t, servicesDir, "db.yaml", `
parameters:
  db: "db"
`)

	// Test both scalar form and mapping form without exclude
	rootPath := writeFile(t, dir, "gendi.yaml", `
imports:
  - ./services/app.yaml
  - path: ./services/db.yaml
`)

	loader := defaultLoader(t, rootPath)
	cfg, err := loader.Load(rootPath, boundaryFor(t, rootPath))
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	if _, ok := cfg.Parameters["app"]; !ok {
		t.Error("expected app parameter to be loaded")
	}
	if _, ok := cfg.Parameters["db"]; !ok {
		t.Error("expected db parameter to be loaded")
	}
}

func TestExcludeNestedGlob(t *testing.T) {
	dir := t.TempDir()
	configDir := filepath.Join(dir, "config")
	prodDir := filepath.Join(configDir, "prod")
	devDir := filepath.Join(configDir, "dev")
	if err := os.MkdirAll(prodDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.MkdirAll(devDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}

	writeFile(t, prodDir, "database.yaml", `
parameters:
  prod_db: "prod"
`)
	writeFile(t, devDir, "dev_database.yaml", `
parameters:
  dev_db: "dev"
`)
	writeFile(t, configDir, "base.yaml", `
parameters:
  base: "base"
`)

	rootPath := writeFile(t, dir, "gendi.yaml", `
imports:
  - path: ./config/**/*.yaml
    exclude:
      - ./config/**/dev_*.yaml
`)

	loader := defaultLoader(t, rootPath)
	cfg, err := loader.Load(rootPath, boundaryFor(t, rootPath))
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	if _, ok := cfg.Parameters["prod_db"]; !ok {
		t.Error("expected prod_db parameter to be loaded")
	}
	if _, ok := cfg.Parameters["base"]; !ok {
		t.Error("expected base parameter to be loaded")
	}
	if _, ok := cfg.Parameters["dev_db"]; ok {
		t.Error("expected dev_db parameter to be excluded")
	}
}

// Exclusions are addressed like import paths, so absolute filesystem paths
// are rejected in them too.
// An absolute exclusion pattern is rejected whether it points at a file or an
// existing directory — it never silently excludes the target.
func TestExcludeRejectsAbsolute(t *testing.T) {
	tests := []struct {
		name       string
		files      map[string]string
		importGlob string
		absTarget  string // path relative to the temp dir, made absolute in the exclude list
	}{
		{
			name: "file pattern",
			files: map[string]string{
				"services/app.yaml":  "parameters:\n  app: \"app\"\n",
				"services/test.yaml": "parameters:\n  test: \"test\"\n",
			},
			importGlob: "./services/*.yaml",
			absTarget:  "services/test.yaml",
		},
		{
			name: "directory",
			files: map[string]string{
				"services/app.yaml":           "parameters:\n  app: \"app\"\n",
				"services/internal/skip.yaml": "parameters:\n  skip_me: \"skip\"\n",
			},
			importGlob: "./services/**/*.yaml",
			absTarget:  "services/internal",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			writeTree(t, dir, tt.files)

			rootPath := writeFile(t, dir, "gendi.yaml", fmt.Sprintf(`
imports:
  - path: %s
    exclude:
      - %q
`, tt.importGlob, filepath.Join(dir, tt.absTarget)))

			loader := defaultLoader(t, rootPath)
			_, err := loader.Load(rootPath, boundaryFor(t, rootPath))
			if err == nil {
				t.Fatal("expected error for absolute exclusion")
			}
			if !strings.Contains(err.Error(), "absolute") {
				t.Fatalf("error should mention absolute paths, got: %v", err)
			}
		})
	}
}

// A module import is excluded with a module-form pattern — the exclusion
// mirrors the import path's addressing, so no importer-relative or resolved-
// base guessing is involved.
func TestExcludeModuleImport(t *testing.T) {
	moduleRoot := t.TempDir()
	writeFile(t, moduleRoot, "go.mod", "module example.com/testmod\n")

	servicesDir := filepath.Join(moduleRoot, "services")
	if err := os.MkdirAll(servicesDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	writeFile(t, servicesDir, "app.yaml", `
parameters:
  app: "app"
`)
	writeFile(t, servicesDir, "skip.yaml", `
parameters:
  skip_me: "skip"
`)

	// The importing file lives in a subdirectory of the module, so its
	// directory differs from the module root the resolved files live under —
	// mirroring makes the exclusion independent of that difference.
	appDir := filepath.Join(moduleRoot, "app")
	if err := os.MkdirAll(appDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	rootPath := writeFile(t, appDir, "gendi.yaml", `
imports:
  - path: example.com/testmod/services/*.yaml
    exclude:
      - example.com/testmod/services/skip.yaml
`)

	loader := defaultLoader(t, rootPath)
	cfg, err := loader.Load(rootPath, boundaryFor(t, rootPath))
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	if _, ok := cfg.Parameters["app"]; !ok {
		t.Error("expected app parameter to be loaded")
	}
	if _, ok := cfg.Parameters["skip_me"]; ok {
		t.Error("expected skip_me parameter to be excluded")
	}
}

// Absolute glob imports are rejected like any other absolute import path.
func TestLoadRejectsAbsoluteGlobImport(t *testing.T) {
	rootDir := t.TempDir()
	extDir := filepath.Join(rootDir, "ext")
	if err := os.MkdirAll(extDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	writeFile(t, extDir, "keep.yaml", `
parameters:
  keep: "keep"
`)

	rootPath := writeFile(t, rootDir, "gendi.yaml", fmt.Sprintf(`
imports:
  - path: %q
`, filepath.Join(extDir, "*.yaml")))

	loader := defaultLoader(t, rootPath)
	_, err := loader.Load(rootPath, boundaryFor(t, rootPath))
	if err == nil {
		t.Fatal("expected error for absolute glob import")
	}
	if !strings.Contains(err.Error(), "absolute") {
		t.Fatalf("error should mention absolute paths, got: %v", err)
	}
}

func TestLoad_UnmarshalError_HasLocation(t *testing.T) {
	tests := []struct {
		name     string
		yaml     string
		wantLine int
		wantMsg  string
	}{
		{
			name:     "import missing path",
			yaml:     "imports:\n  - exclude:\n      - foo",
			wantLine: 2,
			wantMsg:  "import path is required",
		},
		{
			name:     "service wrong type",
			yaml:     "services:\n  my_svc:\n    - item",
			wantLine: 3,
			wantMsg:  "service must be a mapping or alias",
		},
		{
			name:     "tag missing name",
			yaml:     "services:\n  my_svc:\n    constructor:\n      func: fmt.Println\n    tags:\n      - \"\"",
			wantLine: 6,
			wantMsg:  "tag name is required",
		},
		{
			name:     "service null body",
			yaml:     "services:\n  my_svc:",
			wantLine: 2,
			wantMsg:  "service must be a mapping or alias",
		},
		{
			name:     "default service null body",
			yaml:     "services:\n  _default:\n  my_svc:\n    constructor:\n      func: fmt.Println",
			wantLine: 2,
			wantMsg:  "service defaults must be a mapping",
		},
		{
			name:     "default service alias",
			yaml:     "services:\n  _default: \"@other\"\n",
			wantLine: 2,
			wantMsg:  "service defaults must be a mapping",
		},
		{
			name:     "null tag entry",
			yaml:     "services:\n  my_svc:\n    constructor:\n      func: fmt.Println\n    tags:\n      -",
			wantLine: 6,
			wantMsg:  "tag name is required",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			path := writeFile(t, dir, "gendi.yaml", tt.yaml)

			loader := NewConfigLoaderYaml(stubResolver{}, NewParser())
			_, err := loader.Load(path, boundaryFor(t, path))
			if err == nil {
				t.Fatal("expected error")
			}

			var locErr *srcloc.Error
			if !errors.As(err, &locErr) {
				t.Fatalf("expected srcloc.Error, got %T: %v", err, err)
			}
			if locErr.Loc == nil {
				t.Fatal("expected non-nil location")
			}
			if locErr.Loc.Line != tt.wantLine {
				t.Errorf("Line = %d, want %d", locErr.Loc.Line, tt.wantLine)
			}
			if locErr.Message != tt.wantMsg {
				t.Errorf("Message = %q, want %q", locErr.Message, tt.wantMsg)
			}
			if locErr.Loc.File == "" {
				t.Error("expected non-empty File in location")
			}
		})
	}
}

func TestLoad_UnknownField(t *testing.T) {
	tests := []struct {
		name   string
		yaml   string
		field  string
		line   int
		column int
	}{
		{
			name: "top level", yaml: "serivces: {}\nparamters: {}\n",
			field: "serivces", line: 1, column: 1,
		},
		{
			name: "import", yaml: "imports:\n  - paht: child.yaml\n",
			field: "paht", line: 2, column: 5,
		},
		{
			name: "tag definition", yaml: "tags:\n  handlers: {element_typ: string}\n",
			field: "element_typ", line: 2, column: 14,
		},
		{
			name: "service defaults", yaml: "services:\n  _default:\n    publc: true\n",
			field: "publc", line: 3, column: 5,
		},
		{
			name: "service field in service defaults", yaml: "services:\n  _default:\n    shared: true\n    constructor: {func: pkg.New}\n",
			field: "constructor", line: 4, column: 5,
		},
		{
			name: "service", yaml: "services:\n  server:\n    publc: true\n    shraed: false\n    constructor: {func: pkg.New}\n",
			field: "publc", line: 3, column: 5,
		},
		{
			name: "constructor", yaml: "services:\n  server:\n    constructor: {func: pkg.New, agrs: []}\n",
			field: "agrs", line: 3, column: 34,
		},
		{
			name: "merged field", yaml: "services:\n  server:\n    <<: {publc: true}\n",
			field: "publc", line: 3, column: 10,
		},
		{
			name: "merged alias", yaml: "services:\n  base:\n    tags:\n      - name: handlers\n        fields: &fields {publc: true}\ntags:\n  handlers: {<<: *fields}\n",
			field: "publc", line: 5, column: 26,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path := writeYAML(t, tt.yaml)
			boundary := boundaryFor(t, path)
			_, err := LoadConfig(path, boundary, boundary)
			var locErr *srcloc.Error
			if !errors.As(err, &locErr) || locErr.Loc == nil {
				t.Fatalf("expected located error, got %v", err)
			}
			wantLoc := *srcloc.NewLocation(path, tt.line, tt.column)
			if *locErr.Loc != wantLoc {
				t.Errorf("location = %v, want %v", locErr.Loc, &wantLoc)
			}
			if want := fmt.Sprintf("unknown field %q", tt.field); locErr.Message != want {
				t.Errorf("message = %q, want %q", locErr.Message, want)
			}
		})
	}
}

func TestLoad_KeyNotString(t *testing.T) {
	tests := []struct {
		name    string
		yaml    string
		wantMsg string
		line    int
		column  int
	}{
		{
			name: "integer service ID", yaml: "services:\n  123:\n    constructor: {func: pkg.New}\n",
			wantMsg: "service ID 123 is not a string, quote it", line: 2, column: 3,
		},
		{
			name: "hex service ID", yaml: "services:\n  0x10:\n    constructor: {func: pkg.New}\n",
			wantMsg: "service ID 0x10 is not a string, quote it", line: 2, column: 3,
		},
		{
			name: "null service ID", yaml: "services:\n  ~:\n    constructor: {func: pkg.New}\n",
			wantMsg: "service ID ~ is not a string, quote it", line: 2, column: 3,
		},
		{
			name: "bool service ID", yaml: "services:\n  True:\n    constructor: {func: pkg.New}\n",
			wantMsg: "service ID True is not a string, quote it", line: 2, column: 3,
		},
		{
			name: "explicit str tag", yaml: "services:\n  !!str 5:\n    constructor: {func: pkg.New}\n",
			wantMsg: "service ID !!str 5 is not a string, quote it", line: 2, column: 3,
		},
		{
			name: "parameter name", yaml: "parameters:\n  name: x\n  1: hello\n",
			wantMsg: "parameter name 1 is not a string, quote it", line: 3, column: 3,
		},
		{
			name: "tag name", yaml: "tags:\n  .inf: {}\n",
			wantMsg: "tag name .inf is not a string, quote it", line: 2, column: 3,
		},
		{
			name: "tag attribute", yaml: "services:\n  svc:\n    constructor: {func: pkg.New}\n    tags:\n      - {name: handlers, 1.5: x}\n",
			wantMsg: "tag attribute 1.5 is not a string, quote it", line: 5, column: 26,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path := writeYAML(t, tt.yaml)
			boundary := boundaryFor(t, path)
			_, err := LoadConfig(path, boundary, boundary)
			var locErr *srcloc.Error
			if !errors.As(err, &locErr) || locErr.Loc == nil {
				t.Fatalf("expected located error, got %v", err)
			}
			wantLoc := *srcloc.NewLocation(path, tt.line, tt.column)
			if *locErr.Loc != wantLoc {
				t.Errorf("location = %v, want %v", locErr.Loc, &wantLoc)
			}
			if locErr.Message != tt.wantMsg {
				t.Errorf("message = %q, want %q", locErr.Message, tt.wantMsg)
			}
		})
	}
}

// Keys that YAML reads as strings keep their text: quoted numbers, YAML 1.1
// booleans, and keys with an anchor.
func TestLoad_StringKeys(t *testing.T) {
	path := writeYAML(t, `
parameters:
  "1": hello
  yes: on
tags:
  "123": {}
services:
  &svc 0x10x:
    constructor: {func: pkg.New}
    tags: [{name: "123", "1": x}]
  "0x10":
    constructor: {func: pkg.New}
`)
	boundary := boundaryFor(t, path)
	cfg, err := LoadConfig(path, boundary, boundary)
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"1", "yes"} {
		if _, ok := cfg.Parameters[name]; !ok {
			t.Errorf("missing parameter %q", name)
		}
	}
	if _, ok := cfg.Tags["123"]; !ok {
		t.Error(`missing tag "123"`)
	}
	for _, id := range []string{"0x10x", "0x10"} {
		if _, ok := cfg.Services[id]; !ok {
			t.Errorf("missing service %q", id)
		}
	}
	if tags := cfg.Services["0x10x"].Tags; len(tags) != 1 || tags[0].Attributes["1"] != "x" {
		t.Errorf("tag attribute \"1\" = %v, want x", tags)
	}
}

func TestLoad_UnknownFieldInImport(t *testing.T) {
	dir := t.TempDir()
	root := writeFile(t, dir, "gendi.yaml", "imports: [child.yaml]\n")
	child := writeFile(t, dir, "child.yaml", "serivces: {}\n")
	boundary := boundaryFor(t, root)
	_, err := LoadConfig(root, boundary, boundary)
	var locErr *srcloc.Error
	if !errors.As(err, &locErr) || locErr.Loc == nil {
		t.Fatalf("expected located error, got %v", err)
	}
	if locErr.Loc.File != child || locErr.Message != `unknown field "serivces"` {
		t.Errorf("expected unknown field error in %s, got %v", child, err)
	}
}

// An error of an import entry points at that entry in the importing config.
func TestLoad_ImportErrorHasLocation(t *testing.T) {
	tests := []struct {
		name    string
		files   map[string]string
		symlink string // app/link.yaml points at this path, relative to app/
		file    string // the config that holds the import entry
		line    int
		column  int
		want    string
	}{
		{
			name:  "missing file",
			files: map[string]string{"app/gendi.yaml": "imports:\n  - ./missing.yaml\n"},
			file:  "app/gendi.yaml", line: 2, column: 5,
			want: `resolve import "./missing.yaml": not found at`,
		},
		{
			name:  "missing file in mapping form",
			files: map[string]string{"app/gendi.yaml": "imports:\n  - path: ./missing.yaml\n    exclude: []\n"},
			file:  "app/gendi.yaml", line: 2, column: 11,
			want: `resolve import "./missing.yaml": not found at`,
		},
		{
			name:  "empty path",
			files: map[string]string{"app/gendi.yaml": "imports:\n  - \"\"\n"},
			file:  "app/gendi.yaml", line: 2, column: 5,
			want: `resolve import "": import path is empty`,
		},
		{
			name:  "null entry",
			files: map[string]string{"app/gendi.yaml": "imports:\n  - ./child.yaml\n  - ~\n", "app/child.yaml": "{}\n"},
			file:  "app/gendi.yaml", line: 3, column: 5,
			want: "import must be a string or mapping",
		},
		{
			name:  "null entry in tagged sequence",
			files: map[string]string{"app/gendi.yaml": "imports: !!seq\n  - ./child.yaml\n  - ~\n", "app/child.yaml": "{}\n"},
			file:  "app/gendi.yaml", line: 3, column: 5,
			want: "import must be a string or mapping",
		},
		{
			name:  "exclusion",
			files: map[string]string{"app/gendi.yaml": "imports:\n  - path: ./*.yaml\n    exclude:\n      - ./a.yaml\n      - /abs.yaml\n"},
			file:  "app/gendi.yaml", line: 5, column: 9,
			want: `exclusion "/abs.yaml": absolute paths are not allowed`,
		},
		{
			name:  "exclusion in tagged sequence",
			files: map[string]string{"app/gendi.yaml": "imports:\n  - path: ./*.yaml\n    exclude: !!seq [./a.yaml, \"[bad\"]\n"},
			file:  "app/gendi.yaml", line: 3, column: 30,
			want: `invalid exclusion pattern "[bad"`,
		},
		{
			name: "cycle",
			files: map[string]string{
				"app/gendi.yaml": "imports:\n  - ./child.yaml\n",
				"app/child.yaml": "parameters: {}\nimports:\n  - ./gendi.yaml\n",
			},
			file: "app/child.yaml", line: 3, column: 5,
			want: "cyclic import detected at",
		},
		{
			name: "symlink outside boundary",
			files: map[string]string{
				"app/gendi.yaml": "imports: [./link.yaml]\n",
				"outside.yaml":   "{}\n",
			},
			symlink: "../outside.yaml",
			file:    "app/gendi.yaml", line: 1, column: 11,
			want: "import: config",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			writeTree(t, dir, tt.files)
			if tt.symlink != "" {
				if err := os.Symlink(tt.symlink, filepath.Join(dir, "app", "link.yaml")); err != nil {
					t.Fatalf("symlink: %v", err)
				}
			}
			root := filepath.Join(dir, "app", "gendi.yaml")
			boundary := boundaryFor(t, root)
			_, err := LoadConfig(root, boundary, boundary)
			var locErr *srcloc.Error
			if !errors.As(err, &locErr) || locErr.Loc == nil {
				t.Fatalf("expected located error, got %v", err)
			}
			wantLoc := *srcloc.NewLocation(filepath.Join(dir, tt.file), tt.line, tt.column)
			if *locErr.Loc != wantLoc {
				t.Errorf("location = %v, want %v", locErr.Loc, &wantLoc)
			}
			if !strings.Contains(err.Error(), tt.want) {
				t.Errorf("error = %v, want it to contain %q", err, tt.want)
			}
		})
	}
}

func TestLoad_OpenMappings(t *testing.T) {
	path := writeYAML(t, `
parameters:
  publc: true
tags:
  publc: &tag {sort_by: priority}
  other: {<<: *tag}
services:
  _default:
    public: true
  publc:
    constructor:
      func: pkg.New
      args: [value]
    tags: [{name: publc, priority: 10}]
`)
	boundary := boundaryFor(t, path)
	cfg, err := LoadConfig(path, boundary, boundary)
	if err != nil {
		t.Fatal(err)
	}
	service := cfg.Services["publc"]
	if !service.Public || cfg.Tags["other"].SortBy != "priority" {
		t.Fatal("expected the service default and merged tag fields")
	}
	if _, ok := cfg.Parameters["publc"]; !ok {
		t.Error("missing parameter")
	}
	if len(service.Tags) != 1 || service.Tags[0].Attributes["priority"] == nil {
		t.Error("missing tag attribute")
	}
	args := service.Constructor.Args
	if len(args) != 1 || args[0].Literal.String() != "value" {
		t.Error("missing constructor argument")
	}
}

// A directory exclusion — spelled exactly or via a glob that matches the
// directory — excludes that directory's whole subtree.
func TestExcludeDirectorySubtree(t *testing.T) {
	tests := []struct {
		name         string
		files        map[string]string
		exclude      string
		wantLoaded   []string
		wantExcluded []string
	}{
		{
			name: "exact directory path",
			files: map[string]string{
				"services/app.yaml":             "parameters:\n  app: \"app\"\n",
				"services/internal/secret.yaml": "parameters:\n  secret: \"secret\"\n",
				"services/internal/debug.yaml":  "parameters:\n  debug: \"debug\"\n",
			},
			exclude:      "./services/internal",
			wantLoaded:   []string{"app"},
			wantExcluded: []string{"secret", "debug"},
		},
		{
			name: "glob matching directory",
			files: map[string]string{
				"services/app.yaml":           "parameters:\n  app: \"app\"\n",
				"services/internal/skip.yaml": "parameters:\n  skip_me: \"skip\"\n",
			},
			exclude:      "./services/int*",
			wantLoaded:   []string{"app"},
			wantExcluded: []string{"skip_me"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			writeTree(t, dir, tt.files)

			rootPath := writeFile(t, dir, "gendi.yaml", fmt.Sprintf(`
imports:
  - path: ./services/**/*.yaml
    exclude:
      - %s
`, tt.exclude))

			loader := defaultLoader(t, rootPath)
			cfg, err := loader.Load(rootPath, boundaryFor(t, rootPath))
			if err != nil {
				t.Fatalf("Load: %v", err)
			}
			for _, p := range tt.wantLoaded {
				if _, ok := cfg.Parameters[p]; !ok {
					t.Errorf("expected %q parameter to be loaded", p)
				}
			}
			for _, p := range tt.wantExcluded {
				if _, ok := cfg.Parameters[p]; ok {
					t.Errorf("expected %q parameter to be excluded", p)
				}
			}
		})
	}
}

// An exclusion is a mask over what the import found: a mask pointing outside
// the import (an existing sibling directory, an ancestor) matches none of the
// found files and is a silent no-op — never an error, never a way to blank
// the import.
func TestExcludeOutsideMaskIsNoOp(t *testing.T) {
	outer := t.TempDir()
	shared := filepath.Join(outer, "shared")
	if err := os.MkdirAll(shared, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	moduleRoot := filepath.Join(outer, "module")
	servicesDir := filepath.Join(moduleRoot, "services")
	if err := os.MkdirAll(servicesDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	writeFile(t, moduleRoot, "go.mod", "module example.com/app\n")
	writeFile(t, servicesDir, "app.yaml", `
parameters:
  app: "app"
`)

	rootPath := writeFile(t, moduleRoot, "gendi.yaml", `
imports:
  - path: ./services/*.yaml
    exclude:
      - ../shared
      - ../..
      - ./services/missing.yaml
`)

	loader := defaultLoader(t, rootPath)
	cfg, err := loader.Load(rootPath, boundaryFor(t, rootPath))
	if err != nil {
		t.Fatalf("non-matching masks must be no-ops: %v", err)
	}
	if _, ok := cfg.Parameters["app"]; !ok {
		t.Error("expected app parameter to be loaded")
	}
}

// An exclusion glob whose base directory does not exist is a silent no-op,
// like any other glob that matches nothing.
func TestExcludeGlobMissingBaseDirIsSilent(t *testing.T) {
	dir := t.TempDir()
	servicesDir := filepath.Join(dir, "services")
	if err := os.MkdirAll(servicesDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	writeFile(t, servicesDir, "app.yaml", `
parameters:
  app: "app"
`)

	rootPath := writeFile(t, dir, "gendi.yaml", `
imports:
  - path: ./services/*.yaml
    exclude:
      - ./optional/dev_*.yaml
`)

	loader := defaultLoader(t, rootPath)
	cfg, err := loader.Load(rootPath, boundaryFor(t, rootPath))
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if _, ok := cfg.Parameters["app"]; !ok {
		t.Error("expected app parameter to be loaded")
	}
}

// An exclusion must be addressed the same way as its import: a local exclude
// on a module import (or a module exclude on a local import) is an error.
func TestExcludeFormMustMatchImportForm(t *testing.T) {
	moduleRoot := t.TempDir()
	writeFile(t, moduleRoot, "go.mod", "module example.com/testmod\n")
	servicesDir := filepath.Join(moduleRoot, "services")
	if err := os.MkdirAll(servicesDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	writeFile(t, servicesDir, "app.yaml", `
parameters:
  app: "app"
`)
	writeFile(t, servicesDir, "skip.yaml", `
parameters:
  skip_me: "skip"
`)

	// Module import + local exclude.
	rootPath := writeFile(t, moduleRoot, "gendi.yaml", `
imports:
  - path: example.com/testmod/services/*.yaml
    exclude:
      - ./services/skip.yaml
`)
	loader := defaultLoader(t, rootPath)
	if _, err := loader.Load(rootPath, boundaryFor(t, rootPath)); err == nil || !strings.Contains(err.Error(), "does not match the addressing") {
		t.Fatalf("expected form-mismatch error for local exclude on module import, got %v", err)
	}

	// Local import + module exclude.
	rootPath = writeFile(t, moduleRoot, "gendi.yaml", `
imports:
  - path: ./services/*.yaml
    exclude:
      - example.com/testmod/services/skip.yaml
`)
	loader = defaultLoader(t, rootPath)
	if _, err := loader.Load(rootPath, boundaryFor(t, rootPath)); err == nil || !strings.Contains(err.Error(), "does not match the addressing") {
		t.Fatalf("expected form-mismatch error for module exclude on local import, got %v", err)
	}
}

// An empty boundary is rejected loudly instead of silently degrading to the
// process working directory.
func TestLoadConfigRejectsEmptyBoundary(t *testing.T) {
	dir := t.TempDir()
	rootPath := writeFile(t, dir, "gendi.yaml", `
parameters:
  app: "app"
`)

	if _, err := LoadConfig(rootPath, "", ""); err == nil || !strings.Contains(err.Error(), "boundary") {
		t.Fatalf("expected empty-boundary error, got %v", err)
	}
}
