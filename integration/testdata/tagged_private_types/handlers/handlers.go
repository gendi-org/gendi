// Package handlers is in another package than the container, and its
// constructor takes private named types and private aliases. The container
// must not name them.
package handlers

import (
	"fmt"
	"maps"
	"slices"
	"strings"
)

type Handler interface {
	Name() string
}

type namedHandler struct {
	name string
}

func (h *namedHandler) Name() string { return h.name }

func NewUsers() Handler  { return &namedHandler{name: "users"} }
func NewOrders() Handler { return &namedHandler{name: "orders"} }

// handler is a private alias of the element type of the tag.
type handler = Handler

type handlerList []handler

// callback is a private alias of the callback of iter.Seq2.
type callback = func(string, Handler) bool

type routes func(yield callback)

type Registry struct {
	list   handlerList
	routes routes
	byKey  map[string]Handler
}

func NewRegistry(list handlerList, routes routes, byKey map[string]Handler) *Registry {
	return &Registry{list: list, routes: routes, byKey: byKey}
}

func (r *Registry) Describe() string {
	var b strings.Builder
	names := make([]string, len(r.list))
	for i, h := range r.list {
		names[i] = h.Name()
	}
	fmt.Fprintf(&b, "list: %s\n", strings.Join(names, ", "))
	b.WriteString("routes:\n")
	for key, h := range r.routes {
		fmt.Fprintf(&b, "- %s: %s\n", key, h.Name())
	}
	b.WriteString("map:\n")
	for _, key := range slices.Sorted(maps.Keys(r.byKey)) {
		fmt.Fprintf(&b, "- %s: %s\n", key, r.byKey[key].Name())
	}
	return b.String()
}
