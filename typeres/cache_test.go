package typeres

import (
	"errors"
	"go/types"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// structFieldType looks up struct type structName in pkgPath and returns the
// type of the named field, failing the test if anything is missing.
func structFieldType(t *testing.T, c *Cache, pkgPath, structName, field string) types.Type {
	t.Helper()
	pkg, err := c.Get(pkgPath)
	if err != nil {
		t.Fatalf("get %s: %v", pkgPath, err)
	}
	obj := pkg.Scope().Lookup(structName)
	if obj == nil {
		t.Fatalf("%s.%s not found", pkgPath, structName)
	}
	st, ok := obj.Type().Underlying().(*types.Struct)
	if !ok {
		t.Fatalf("%s.%s is not a struct", pkgPath, structName)
	}
	for f := range st.Fields() {
		if f.Name() == field {
			return f.Type()
		}
	}
	t.Fatalf("field %s not found in %s.%s", field, pkgPath, structName)
	return nil
}

// TestSharedUniverseAcrossSeparateLoads is the regression guard for reusing one
// Cache across loads: two packages decoded by SEPARATE LoadWithCandidates calls
// (each doing its own packages.Load) must still land in one type universe, so a
// type they both reference is identical. This fails under a NeedTypes-per-call
// loader, where every packages.Load builds an isolated universe.
func TestSharedUniverseAcrossSeparateLoads(t *testing.T) {
	c := NewCache(".", "")

	if err := c.Load([]string{"net/http"}); err != nil {
		t.Fatalf("load net/http: %v", err)
	}
	if err := c.Load([]string{"archive/zip"}); err != nil {
		t.Fatalf("load archive/zip: %v", err)
	}

	httpTime := structFieldType(t, c, "net/http", "Cookie", "Expires")
	zipTime := structFieldType(t, c, "archive/zip", "FileHeader", "Modified")
	if !types.Identical(httpTime, zipTime) {
		t.Fatalf("time.Time from separate loads not identical: %s vs %s", httpTime, zipTime)
	}
}

// TestDirectLoadCompletesTransitiveStub guards the completeness-upgrade path:
// a package first pulled in as an incomplete transitive dependency must be
// re-decoded in place (same map entry) and become complete when requested
// directly, since the resolver queries arbitrary symbols in it.
func TestDirectLoadCompletesTransitiveStub(t *testing.T) {
	c := NewCache(".", "")
	if err := c.Load([]string{"net/http"}); err != nil {
		t.Fatalf("load net/http: %v", err)
	}

	stub := c.packages["time"]
	if stub == nil {
		t.Fatal("time not pulled in transitively by net/http")
	}

	if err := c.Load([]string{"time"}); err != nil {
		t.Fatalf("load time: %v", err)
	}
	got := c.packages["time"]
	if got != stub {
		t.Fatal("time entry replaced instead of upgraded in place")
	}
	if !got.Complete() {
		t.Fatal("time still incomplete after direct load")
	}
	if got.Scope().Lookup("Parse") == nil {
		t.Fatal("time.Parse not found after completion")
	}
}

// TestLoadSkipsCompleteCachedPaths keeps the incremental optimization: a
// complete package already in the cache is not re-decoded on a second load.
func TestLoadSkipsCompleteCachedPaths(t *testing.T) {
	c := NewCache(".", "")
	if err := c.Load([]string{"time"}); err != nil {
		t.Fatalf("load time: %v", err)
	}
	first := c.packages["time"]
	if first == nil || !first.Complete() {
		t.Fatalf("time not loaded complete on first Load")
	}

	if err := c.Load([]string{"time"}); err != nil {
		t.Fatalf("reload time: %v", err)
	}
	if c.packages["time"] != first {
		t.Fatal("complete cached package was re-decoded")
	}
}

func TestLoadReportsPackageErrors(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module example.com/app\n\ngo 1.26.0\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(dir, "broken"), 0o755); err != nil {
		t.Fatal(err)
	}
	broken := "package broken\n\nfunc X() int { return \"s\" }\n"
	if err := os.WriteFile(filepath.Join(dir, "broken", "b.go"), []byte(broken), 0o644); err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name      string
		paths     []string
		wantPaths []string
		wantMsg   []string
	}{
		{
			name:      "missing package",
			paths:     []string{"example.com/app/nope"},
			wantPaths: []string{"example.com/app/nope"},
			wantMsg:   []string{"load package example.com/app/nope: no required module provides package example.com/app/nope"},
		},
		{
			name:      "malformed path",
			paths:     []string{"$this"},
			wantPaths: []string{"$this"},
			wantMsg:   []string{`load package $this: malformed import path "$this"`},
		},
		{
			name:      "package does not compile",
			paths:     []string{"example.com/app/broken"},
			wantPaths: []string{"example.com/app/broken"},
			wantMsg:   []string{"load package example.com/app/broken: # example.com/app/broken\nbroken/b.go:3:23: cannot use"},
		},
		{
			name:      "errors sorted by path",
			paths:     []string{"example.com/app/zzz", "time", "example.com/app/aaa"},
			wantPaths: []string{"example.com/app/aaa", "example.com/app/zzz"},
			wantMsg:   []string{"load package example.com/app/aaa:", "load package example.com/app/zzz:"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := NewCache(dir, "").Load(tt.paths)
			if err == nil {
				t.Fatal("expected error, got nil")
			}
			joined, ok := err.(interface{ Unwrap() []error })
			if !ok {
				t.Fatalf("error %T does not join package errors", err)
			}
			errs := joined.Unwrap()
			if len(errs) != len(tt.wantPaths) {
				t.Fatalf("got %d errors, want %d: %v", len(errs), len(tt.wantPaths), err)
			}
			for i, e := range errs {
				var pkgErr *PackageError
				if !errors.As(e, &pkgErr) {
					t.Fatalf("error %d is %T, want *PackageError", i, e)
				}
				if pkgErr.Path != tt.wantPaths[i] {
					t.Errorf("error %d path = %q, want %q", i, pkgErr.Path, tt.wantPaths[i])
				}
				if !strings.HasPrefix(e.Error(), tt.wantMsg[i]) {
					t.Errorf("error %d = %q, want prefix %q", i, e.Error(), tt.wantMsg[i])
				}
			}
		})
	}
}
