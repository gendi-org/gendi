package typeres

import (
	"errors"
	"fmt"
	"go/token"
	"go/types"
	"os"
	"slices"
	"strings"

	"golang.org/x/tools/go/gcexportdata"
	"golang.org/x/tools/go/packages"
)

// Cache handles loading and caching of Go packages into a single shared type
// universe. Every package is decoded from compiler export data into one shared
// imports map, so types from packages loaded by separate packages.Load calls
// remain identical and comparable — a Cache can be reused across many loads and
// still yield one coherent universe.
type Cache struct {
	packages   map[string]*types.Package
	fset       *token.FileSet
	moduleRoot string
	buildTags  string
}

// NewCache creates a new package cache. buildTags is passed to the package
// loader as-is via -tags= when non-empty.
func NewCache(moduleRoot, buildTags string) *Cache {
	return &Cache{
		packages:   make(map[string]*types.Package),
		fset:       token.NewFileSet(),
		moduleRoot: moduleRoot,
		buildTags:  buildTags,
	}
}

// Get retrieves a package from the cache.
// Returns an error if the package has not been loaded.
func (c *Cache) Get(path string) (*types.Package, error) {
	if pkg, ok := c.packages[path]; ok {
		return pkg, nil
	}
	return nil, fmt.Errorf("package %q not loaded", path)
}

// Load loads packages by their import paths and caches their type information.
func (c *Cache) Load(paths []string) error {
	return c.LoadWithCandidates(paths, nil)
}

// key returns the import path of a loaded package.
func (c *Cache) key(pkg *packages.Package) string {
	if pkg.PkgPath != "" {
		return pkg.PkgPath
	}
	return pkg.ID
}

// LoadWithCandidates loads required paths together with candidate paths
// derived from ambiguous qualified names (e.g. field access on a Go symbol,
// where the package/symbol boundary is unknown). Candidates that do not
// resolve to a real package are skipped instead of failing the load.
//
// Packages are located and built via one packages.Load (NeedExportFile, so the
// go tool compiles them as needed and the export data is fresh), then decoded
// with gcexportdata.Read into the shared imports map. Because that map is the
// single type universe, every requested package is decoded from its own export
// file — a package that only appears as an incomplete transitive dependency of
// another is re-decoded when requested directly (see missing).
func (c *Cache) LoadWithCandidates(required, candidates []string) error {
	required = c.missing(required)
	candidates = c.missing(candidates)
	if len(required)+len(candidates) == 0 {
		return nil
	}

	cfg := &packages.Config{Mode: packages.NeedName | packages.NeedExportFile, Dir: c.moduleRoot}
	if c.buildTags != "" {
		cfg.BuildFlags = []string{"-tags=" + c.buildTags}
	}

	candidateSet := make(map[string]bool, len(candidates))
	for _, p := range candidates {
		candidateSet[p] = true
	}

	pkgs, err := packages.Load(cfg, slices.Concat(required, candidates)...)
	if err != nil {
		return fmt.Errorf("load packages: %w", err)
	}

	var errs []*PackageError
	for _, pkg := range pkgs {
		if err := c.decode(pkg); err != nil && !candidateSet[pkg.PkgPath] && !candidateSet[pkg.ID] {
			errs = append(errs, &PackageError{Path: c.key(pkg), Err: err})
		}
	}
	slices.SortFunc(errs, func(a, b *PackageError) int {
		return strings.Compare(a.Path, b.Path)
	})

	joined := make([]error, len(errs))
	for i, err := range errs {
		joined[i] = err
	}
	return errors.Join(joined...)
}

// missing returns the subset of paths that still need loading: those absent
// from the cache or present only as an incomplete package (a transitive
// dependency decoded on behalf of another package). A path requested directly
// must resolve to a complete package, so incomplete entries are re-decoded.
func (c *Cache) missing(paths []string) []string {
	out := paths[:0:0]
	for _, p := range paths {
		pkg, ok := c.packages[p]
		if !ok || pkg == nil || !pkg.Complete() {
			out = append(out, p)
		}
	}
	return out
}

// decode reads a package's export data into the shared imports map, returning a
// non-nil error if the package is unusable (load errors, missing export data,
// or an unreadable export file). Imports referenced by the export data are
// resolved through the same map, keeping the whole universe consistent.
func (c *Cache) decode(pkg *packages.Package) error {
	if len(pkg.Errors) > 0 {
		errs := make([]error, len(pkg.Errors))
		for i, e := range pkg.Errors {
			errs[i] = e
			// Without a position, packages.Error prints "-: msg".
			if e.Pos == "" {
				errs[i] = errors.New(e.Msg)
			}
		}
		return errors.Join(errs...)
	}

	key := c.key(pkg)

	if pkg.ExportFile == "" {
		return errors.New("no export data")
	}

	// Already-complete entries are left untouched to satisfy Read's precondition.
	if existing, ok := c.packages[key]; ok && existing != nil && existing.Complete() {
		return nil
	}

	f, err := os.Open(pkg.ExportFile)
	if err != nil {
		return fmt.Errorf("open export data: %w", err)
	}
	defer f.Close()

	r, err := gcexportdata.NewReader(f)
	if err != nil {
		return fmt.Errorf("read export data: %w", err)
	}
	if _, err := gcexportdata.Read(r, c.fset, c.packages, key); err != nil {
		return fmt.Errorf("decode export data: %w", err)
	}

	return nil
}
