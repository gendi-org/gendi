// Package routes provides constructors whose map parameter is typed through
// a name the generated package (main, in this fixture) cannot spell: an
// unexported named map type, and an unexported alias of an unnamed map
// type. Both exist to prove the generator falls back to rendering the
// underlying map[K]V type in the composite literal it emits — an unnamed
// literal is still assignable through either — so the generated code
// actually compiles, not just looks right in a substring check.
package routes

import (
	"fmt"
	"sort"
)

// Handler is the map argument's value type.
type Handler struct {
	Name string
}

// NewHandler is a trivial constructor for Handler service entries.
func NewHandler(name string) Handler {
	return Handler{Name: name}
}

// namedRoutes is an unexported named map type.
type namedRoutes map[string]Handler

// NamedRouter holds routes built through namedRoutes.
type NamedRouter struct {
	routes namedRoutes
}

// NewNamedRouter is exported, but its parameter type is not.
func NewNamedRouter(routes namedRoutes) *NamedRouter {
	return &NamedRouter{routes: routes}
}

// Run prints routes to stdout in a stable order.
func (r *NamedRouter) Run() {
	for _, path := range sortedPaths(r.routes) {
		fmt.Printf("named %s -> %s\n", path, r.routes[path].Name)
	}
}

// aliasRoutes is an unexported alias of an unnamed map[string]Handler: there
// is no *types.Named anywhere in it, only a name of its own.
type aliasRoutes = map[string]Handler

// AliasRouter holds routes built through aliasRoutes.
type AliasRouter struct {
	routes aliasRoutes
}

// NewAliasRouter is exported, but its parameter type is not.
func NewAliasRouter(routes aliasRoutes) *AliasRouter {
	return &AliasRouter{routes: routes}
}

// Run prints routes to stdout in a stable order.
func (r *AliasRouter) Run() {
	for _, path := range sortedPaths(r.routes) {
		fmt.Printf("alias %s -> %s\n", path, r.routes[path].Name)
	}
}

func sortedPaths(m map[string]Handler) []string {
	paths := make([]string, 0, len(m))
	for path := range m {
		paths = append(paths, path)
	}
	sort.Strings(paths)
	return paths
}
