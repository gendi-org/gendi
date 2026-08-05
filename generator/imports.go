package generator

import (
	"fmt"
	"go/types"
	"sort"
	"strings"
)

type ImportManager struct {
	aliases       map[string]string
	reserved      map[string]string
	used          map[string]bool
	required      map[string]bool
	outputPkgPath string
}

func NewImportManager(outputPkgPath string, reservedAliases ...string) *ImportManager {
	m := &ImportManager{
		aliases:       map[string]string{},
		reserved:      map[string]string{},
		used:          map[string]bool{},
		required:      map[string]bool{},
		outputPkgPath: outputPkgPath,
	}
	m.ReserveAliases(reservedAliases...)
	return m
}

// Require marks a package as needed by emitted code so it is rendered in the
// import block without an alias.
func (m *ImportManager) Require(path string) {
	m.required[path] = true
}

// ReserveAliases marks aliases as used so they cannot be selected.
func (m *ImportManager) ReserveAliases(aliases ...string) {
	for _, alias := range aliases {
		if alias == "" {
			continue
		}
		m.used[alias] = true
	}
}

// ReserveImport pins the alias for a package path and marks it as used, so
// hardcoded references in emitted code (e.g. sync.Mutex, fmt.Errorf) and
// qualified references resolved through the type system share one import.
func (m *ImportManager) ReserveImport(path, alias string) {
	m.reserved[path] = alias
	m.used[alias] = true
}

func (m *ImportManager) qualifier(pkg *types.Package) string {
	if pkg.Path() == m.outputPkgPath {
		return ""
	}
	if alias, ok := m.aliases[pkg.Path()]; ok {
		return alias
	}
	if alias, ok := m.reserved[pkg.Path()]; ok {
		m.aliases[pkg.Path()] = alias
		return alias
	}
	base := pkg.Name()
	alias := base
	if alias == "" {
		alias = "pkg"
	}
	if m.aliasInUse(alias) {
		for i := 2; ; i++ {
			candidate := fmt.Sprintf("%s%d", alias, i)
			if !m.aliasInUse(candidate) {
				alias = candidate
				break
			}
		}
	}
	m.aliases[pkg.Path()] = alias
	m.used[alias] = true
	return alias
}

func (m *ImportManager) aliasInUse(alias string) bool {
	return m.used[alias]
}

func (m *ImportManager) typeString(t types.Type) string {
	return types.TypeString(t, m.qualifier)
}

// mapLiteralType returns the type to render for a map composite literal. A
// named map type is rendered by name when it is accessible from the
// generated package. When it is not — an unexported type from another
// package — the underlying, unnamed map[K]V type is rendered instead: an
// unnamed map literal is still assignable to the named type, so the
// constructor call compiles even though the generated package can never
// spell the named type itself.
func (m *ImportManager) mapLiteralType(t types.Type) types.Type {
	named, ok := types.Unalias(t).(*types.Named)
	if !ok {
		return t
	}
	obj := named.Obj()
	if obj.Pkg() == nil || obj.Pkg().Path() == m.outputPkgPath || obj.Exported() {
		return t
	}
	return named.Underlying()
}

func (m *ImportManager) funcName(fn *types.Func) string {
	pkg := fn.Pkg()
	if pkg == nil {
		return fn.Name()
	}
	alias := m.qualifier(pkg)
	if alias == "" {
		return fn.Name()
	}
	return alias + "." + fn.Name()
}

// funcNameWithTypeArgs returns the function name with type arguments for generic functions.
func (m *ImportManager) funcNameWithTypeArgs(fn *types.Func, typeArgs []types.Type) string {
	name := m.funcName(fn)
	if len(typeArgs) == 0 {
		return name
	}

	// Build type arguments string
	typeArgStrs := make([]string, len(typeArgs))
	for i, t := range typeArgs {
		typeArgStrs[i] = m.typeString(t)
	}

	return name + "[" + strings.Join(typeArgStrs, ", ") + "]"
}

func (m *ImportManager) renderImports() string {
	var imports []string
	for path, alias := range m.aliases {
		// Reserved imports bind to their package's own name, so the alias
		// is redundant in source.
		if alias == "" || m.reserved[path] == alias {
			imports = append(imports, fmt.Sprintf("\t\"%s\"\n", path))
			continue
		}
		imports = append(imports, fmt.Sprintf("\t%s \"%s\"\n", alias, path))
	}
	for path := range m.required {
		if _, aliased := m.aliases[path]; !aliased {
			imports = append(imports, fmt.Sprintf("\t\"%s\"\n", path))
		}
	}
	sort.Strings(imports)
	if len(imports) == 0 {
		return ""
	}
	return "import (\n" + strings.Join(imports, "") + ")\n\n"
}
