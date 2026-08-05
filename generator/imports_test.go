package generator

import (
	"go/token"
	"go/types"
	"strings"
	"testing"
)

func TestRenderImportsReservedPathIsNotDuplicated(t *testing.T) {
	m := NewImportManager("example.com/output")
	m.ReserveImport("sync", "sync")
	m.Require("sync")

	// A constructor signature referencing stdlib sync resolves it through
	// the qualifier; the reserved alias must be reused, not sync2.
	syncPkg := types.NewPackage("sync", "sync")
	if got := m.qualifier(syncPkg); got != "sync" {
		t.Fatalf("qualifier(sync) = %q, want %q", got, "sync")
	}

	rendered := m.renderImports()
	if got := strings.Count(rendered, "\"sync\""); got != 1 {
		t.Fatalf("import path \"sync\" rendered %d times, want 1:\n%s", got, rendered)
	}
	if strings.Contains(rendered, "sync2") {
		t.Fatalf("reserved package must not get a numbered alias:\n%s", rendered)
	}
	if strings.Contains(rendered, "sync \"sync\"") {
		t.Fatalf("reserved import must render without redundant alias:\n%s", rendered)
	}
}

func TestRenderImportsUserPackageNamedSyncGetsNumberedAlias(t *testing.T) {
	m := NewImportManager("example.com/output")
	m.ReserveImport("sync", "sync")
	m.Require("sync")

	userSync := types.NewPackage("example.com/app/sync", "sync")
	if got := m.qualifier(userSync); got != "sync2" {
		t.Fatalf("qualifier(user sync) = %q, want %q", got, "sync2")
	}

	rendered := m.renderImports()
	if !strings.Contains(rendered, "\t\"sync\"\n") {
		t.Fatalf("required stdlib sync import missing:\n%s", rendered)
	}
	if !strings.Contains(rendered, "sync2 \"example.com/app/sync\"") {
		t.Fatalf("user package must keep its numbered alias:\n%s", rendered)
	}
}

func TestRenderImportsRequiredSkippedWhenAlreadyReservedAndQualified(t *testing.T) {
	m := NewImportManager("example.com/output")
	m.ReserveImport("fmt", "fmt")

	fmtPkg := types.NewPackage("fmt", "fmt")
	if got := m.qualifier(fmtPkg); got != "fmt" {
		t.Fatalf("qualifier(fmt) = %q, want %q", got, "fmt")
	}
	m.Require("fmt")

	rendered := m.renderImports()
	if got := strings.Count(rendered, "\"fmt\""); got != 1 {
		t.Fatalf("import path \"fmt\" rendered %d times, want 1:\n%s", got, rendered)
	}
}

// TestMapLiteralType covers the accessibility rule mapBuilder relies on to
// render a map argument's composite literal type: a named type or alias is
// rendered by its own name only when that name can actually be spelled from
// the generated package, otherwise the walk steps past it. Two of these
// cases are regressions a naive "check the target's accessibility" version
// would get wrong: an unexported alias of an exported named type (the target
// is accessible, but that's not what gets printed — the alias's own name
// is), and an unexported alias of an unnamed type (there's no *types.Named
// at all for such a version to find).
func TestMapLiteralType(t *testing.T) {
	otherPkg := types.NewPackage("example.com/other", "other")
	mapType := types.NewMap(types.Typ[types.String], types.Typ[types.Int])

	namedExported := types.NewNamed(types.NewTypeName(token.NoPos, otherPkg, "Routes", nil), mapType, nil)
	namedUnexported := types.NewNamed(types.NewTypeName(token.NoPos, otherPkg, "routes", nil), mapType, nil)

	aliasOfUnnamed := types.NewAlias(types.NewTypeName(token.NoPos, otherPkg, "routes", nil), mapType)
	aliasOfNamedExported := types.NewAlias(types.NewTypeName(token.NoPos, otherPkg, "routes", nil), namedExported)
	aliasExported := types.NewAlias(types.NewTypeName(token.NoPos, otherPkg, "Routes", nil), mapType)

	m := NewImportManager("example.com/generated")

	for _, tt := range []struct {
		name string
		in   types.Type
		want types.Type
	}{
		{"exported named type stays named", namedExported, namedExported},
		{"unexported named type falls back to its underlying map", namedUnexported, mapType},
		{
			"unexported alias of an unnamed map falls back to that map",
			aliasOfUnnamed, mapType,
		},
		{
			// The naive fix reviewed here checked the *target's*
			// accessibility (Routes is exported) and returned the alias
			// unchanged — but the alias's own name, routes, is what's
			// printed, and it's unexported. The walk must step past the
			// alias to the named type it targets.
			"unexported alias of an exported named type renders the named type, not the alias",
			aliasOfNamedExported, namedExported,
		},
		{"exported alias stays an alias", aliasExported, aliasExported},
	} {
		t.Run(tt.name, func(t *testing.T) {
			got := m.mapLiteralType(tt.in)
			if got != tt.want {
				t.Fatalf("mapLiteralType(%s) = %s (%T), want %s (%T)", tt.in, got, got, tt.want, tt.want)
			}
		})
	}
}
