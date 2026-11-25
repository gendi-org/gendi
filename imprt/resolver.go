package imprt

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/gendi-org/gendi/gomod"
)

// classify reports the addressing form of pattern, purely syntactically —
// no filesystem access. A pattern names a Go module only when it is
// multi-segment and its first segment contains a dot; everything else is a
// local path resolved against the importing file. Single-segment patterns
// (base.yaml, test_*.yaml, ".", "..") are always local — a module import
// must name a file inside the module, so it always contains a slash. Empty
// and absolute paths are errors.
func classify(pattern string) (kind, error) {
	if pattern == "" {
		return 0, fmt.Errorf("import path is empty")
	}
	if filepath.IsAbs(pattern) {
		return 0, fmt.Errorf("absolute paths are not allowed; use a path relative to the importing file or a Go module path")
	}
	if strings.HasPrefix(pattern, "./") || strings.HasPrefix(pattern, "../") {
		return kindLocal, nil
	}
	if strings.Contains(pattern, "/") && gomod.LooksLikeModulePath(pattern) {
		return kindModule, nil
	}
	return kindLocal, nil
}

// Candidate is a config file found by import resolution. Path preserves the
// addressed spelling, while Boundary is the root within which the YAML loader
// must confine the file before reading it.
type Candidate struct {
	Path     string
	Boundary string
}

// Resolver resolves import entries to config file candidates through one fixed
// pipeline: classify the import (local directory or Go module), compute its
// anchor and confinement boundary, find the files the import mask matches, and
// drop the ones matched by the exclusion masks. The YAML loader performs the
// final confinement check immediately before loading each candidate.
type Resolver struct {
	// boundary is the absolute path outside which loading is forbidden when
	// the importing file is not inside any Go module; within a module, that
	// module's root is the boundary instead.
	boundary string
	// moduleContext is the directory whose go.mod graph is used to resolve
	// module imports when the importing file lives outside any Go module.
	// It is deliberately independent from boundary: an external config must
	// remain confined to its own filesystem root while resolving modules in
	// the project that consumes it.
	moduleContext string
	// moduleDirs memoizes module resolution per (module root, modulePath),
	// failures included: each cold lookup can cost a `go list` subprocess,
	// and an unresolvable candidate prefix would otherwise re-run it for
	// every import entry sharing that prefix. Results depend only on the
	// context directory's go.mod graph, never on the process working
	// directory. Loading is single-threaded, so no locking is needed.
	moduleDirs map[string]moduleLookup
}

// moduleLookup is a memoized module resolution: the module directory when
// resolution succeeded, or ok=false when the candidate is not a module.
type moduleLookup struct {
	dir string
	ok  bool
}

// findResult describes a resolved module and the part of the import path that
// remains relative to its directory.
type findResult struct {
	moduleDir  string
	modulePath string
	remainder  string
}

// NewResolver creates a Resolver whose out-of-module confinement boundary is
// boundary. moduleContext supplies the go.mod graph used for module imports
// from configs outside any Go module; it may be empty when those configs use
// local imports only. An empty boundary is an error: it would silently degrade
// to the process working directory.
//
// A Resolver memoizes module lookups without locking and is not safe for
// concurrent use: to resolve imports in parallel, give each goroutine its
// own Resolver.
func NewResolver(boundary, moduleContext string) (*Resolver, error) {
	if boundary == "" {
		return nil, fmt.Errorf("boundary must not be empty")
	}
	abs, err := filepath.Abs(boundary)
	if err != nil {
		return nil, err
	}
	if moduleContext != "" {
		moduleContext, err = filepath.Abs(moduleContext)
		if err != nil {
			return nil, err
		}
	}
	return &Resolver{
		boundary:      abs,
		moduleContext: moduleContext,
		moduleDirs:    map[string]moduleLookup{},
	}, nil
}

// findModule locates a Go module by iterating through import path segments,
// resolving within the module context of the importing file: the module
// containing baseDir, or — when baseDir is outside any module — the module at
// the resolver's module context. Without either, module imports are impossible
// and the error asks for an explicit project root. Lookups are memoized in
// moduleDirs per (module root, modulePath), failures included.
func (r *Resolver) findModule(baseDir, importPath string) (findResult, error) {
	contextDir, _, found := gomod.FindModuleRoot(baseDir)
	if !found {
		if r.moduleContext == "" {
			return findResult{}, fmt.Errorf("module import %q requires a Go module: no go.mod found above %s and no module context was provided", importPath, baseDir)
		}
		contextDir, _, found = gomod.FindModuleRoot(r.moduleContext)
		if !found {
			return findResult{}, fmt.Errorf("module import %q requires a Go module: no go.mod found above %s or the module context %s", importPath, baseDir, r.moduleContext)
		}
	}

	locator := gomod.NewLocator(contextDir)
	parts := strings.Split(importPath, "/")
	for i := len(parts); i >= 1; i-- {
		candidate := strings.Join(parts[:i], "/")
		// Skip if candidate contains glob characters.
		if isGlobPattern(candidate) {
			continue
		}
		key := contextDir + "\x00" + candidate
		lookup, cached := r.moduleDirs[key]
		if !cached {
			dir, err := locator.FindModuleDir(candidate)
			lookup = moduleLookup{dir: dir, ok: err == nil}
			r.moduleDirs[key] = lookup
		}
		if !lookup.ok {
			continue
		}
		return findResult{
			moduleDir:  lookup.dir,
			modulePath: candidate,
			remainder:  strings.Join(parts[i:], "/"),
		}, nil
	}

	return findResult{}, fmt.Errorf("module %s not found", importPath)
}

// localBoundary returns the confinement boundary for a path resolved relative
// to an importing file in dir: the root of the Go module containing dir, or the
// resolver's out-of-module boundary when dir is inside no module.
func (r *Resolver) localBoundary(dir string) string {
	if root, _, found := gomod.FindModuleRoot(dir); found {
		return pathToAbs(root)
	}
	return pathToAbs(r.boundary)
}

// localMatch reports whether pattern names an existing file (or, for a glob, at
// least one match) relative to baseDir — the signal that a module-shaped
// spelling was really meant as a local path. Best-effort: a malformed glob is
// treated as no match, deferring the authoritative error to resolution.
func (r *Resolver) localMatch(baseDir, pattern string) bool {
	if isGlobPattern(pattern) {
		files, err := globMatches(baseDir, pattern)
		return err == nil && len(files) > 0
	}
	return fileExists(filepath.Join(baseDir, pattern))
}

// address classifies pattern and computes its resolution anchor. Local
// patterns resolve against the importing file's directory and are confined to
// its module; module patterns resolve against — and are confined to — the
// resolved module's directory.
func (r *Resolver) address(baseDir, pattern string) (target, error) {
	k, err := classify(pattern)
	if err != nil {
		return target{}, err
	}
	if k == kindLocal {
		return target{
			kind:      kindLocal,
			anchorDir: baseDir,
			boundary:  r.localBoundary(baseDir),
			pattern:   pattern,
		}, nil
	}

	result, err := r.findModule(baseDir, pattern)
	if err != nil {
		// Only suggest the explicit local spelling when the same spelling
		// exists locally; otherwise the hint would distract from the actual
		// failure (no go.mod, typo'd module).
		if r.localMatch(baseDir, pattern) {
			return target{}, fmt.Errorf("%w (for a local directory use %q)", err, "./"+pattern)
		}
		return target{}, err
	}
	if result.remainder == "" {
		return target{}, fmt.Errorf("module import %q must reference a file, e.g. %s/gendi.yaml", pattern, result.modulePath)
	}
	return target{
		kind:       kindModule,
		anchorDir:  pathToAbs(result.moduleDir),
		boundary:   pathToAbs(result.moduleDir),
		pattern:    result.remainder,
		modulePath: result.modulePath,
	}, nil
}

// ResolveImport resolves an import entry to config file candidates. Paths are
// absolute and preserve their addressed spelling, so every symlink alias keeps
// its own relative-import and $this context. A literal path must name an
// existing file; a glob that matches nothing resolves to no files. Files
// matched by any exclusion mask are dropped before candidates reach the YAML
// loader and its sandbox check. An error of one exclusion pattern is an
// *ExclusionError.
func (r *Resolver) ResolveImport(baseDir, importPath string, excludes []string) ([]Candidate, error) {
	// One invariant for the whole pipeline: baseDir is absolute past this
	// point. Anchoring, module context, and the confinement boundary must all
	// derive from the same directory, never from the process working
	// directory.
	baseDir = pathToAbs(baseDir)
	t, err := r.address(baseDir, importPath)
	if err != nil {
		return nil, err
	}
	masks, err := t.excludeMasks(excludes)
	if err != nil {
		return nil, err
	}

	var files []string
	if isGlobPattern(t.pattern) {
		files, err = globMatches(t.anchorDir, t.pattern)
		if err != nil {
			return nil, err
		}
	} else {
		full := filepath.Join(t.anchorDir, t.pattern)
		if !fileExists(full) {
			return nil, t.notFoundError(full)
		}
		files = []string{pathToAbs(full)}
	}

	kept := make([]string, 0, len(files))
	for _, file := range files {
		excluded, err := t.excludedBy(masks, file)
		if err != nil {
			return nil, err
		}
		if !excluded {
			kept = append(kept, file)
		}
	}

	candidates := make([]Candidate, 0, len(kept))
	for _, file := range kept {
		candidates = append(candidates, Candidate{
			Path:     file,
			Boundary: t.boundary,
		})
	}
	return candidates, nil
}
