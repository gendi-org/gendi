package main

import (
	"iter"
	"maps"
	"slices"
)

type Handler interface {
	Name() string
}

type NamedHandler struct {
	name string
}

func (h *NamedHandler) Name() string { return h.name }

func NewUsersHandler() Handler  { return &NamedHandler{name: "users"} }
func NewOrdersHandler() Handler { return &NamedHandler{name: "orders"} }
func NewHealthHandler() Handler { return &NamedHandler{name: "health"} }

type LoggingHandler struct {
	inner Handler
}

func (h *LoggingHandler) Name() string { return "logged " + h.inner.Name() }

func NewLoggingHandler(inner Handler) Handler { return &LoggingHandler{inner: inner} }

// Registry is a named map type: a keyed tagged collection must match it.
type Registry map[string]Handler

type Router struct {
	routes Registry
	order  iter.Seq2[string, Handler]
}

func NewRouter(routes Registry, order iter.Seq2[string, Handler]) *Router {
	return &Router{routes: routes, order: order}
}

// Keys returns the keys of the map in sorted order: a map has no order.
func (r *Router) Keys() []string {
	return slices.Sorted(maps.Keys(r.routes))
}
