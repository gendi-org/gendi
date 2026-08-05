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
// named type or alias is rendered by its own name when that name is
// accessible from the generated package. When it is not — an unexported
// name from another package — the walk steps past it to whatever it names
// next (an alias's target, or a named type's underlying map[K]V), because an
// unnamed map literal is still assignable through any number of alias and
// named layers, so the constructor call compiles even though the generated
// package can never spell the inaccessible name.
func (m *ImportManager) mapLiteralType(t types.Type) types.Type {
	for {
		switch typ := t.(type) {
		case *types.Alias:
			if m.typeNameAccessible(typ.Obj()) {
				return t
			}
			t = types.Unalias(typ)
		case *types.Named:
			if m.typeNameAccessible(typ.Obj()) {
				return t
			}
			return typ.Underlying()
		default:
			return t
		}
	}
}

// typeNameAccessible reports whether obj's name can be spelled from the
// generated package: it's a predeclared name, declared in the generated
// package itself, or exported.
func (m *ImportManager) typeNameAccessible(obj *types.TypeName) bool {
	return obj.Pkg() == nil || obj.Pkg().Path() == m.outputPkgPath || obj.Exported()
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
