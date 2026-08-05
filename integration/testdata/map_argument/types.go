package main

import (
	"fmt"
	"sort"
	"time"
)

type Region string

type Handler struct{ name string }

func NewHandler(name string) Handler { return Handler{name: name} }

var FallbackHandler = Handler{name: "fallback"}

type Router struct {
	routes   map[string]Handler
	labels   map[Region]string
	timeouts map[string]time.Duration
}

func NewRouter(routes map[string]Handler, labels map[Region]string, timeouts map[string]time.Duration) *Router {
	return &Router{routes: routes, labels: labels, timeouts: timeouts}
}

func (r *Router) Run() {
	for _, path := range sortedKeys(r.routes) {
		fmt.Printf("%s -> %s\n", path, r.routes[path].name)
	}
	labels := make(map[string]string, len(r.labels))
	for region, label := range r.labels {
		labels[string(region)] = label
	}
	for _, region := range sortedKeys(labels) {
		fmt.Printf("%s = %s\n", region, labels[region])
	}
	for _, name := range sortedKeys(r.timeouts) {
		fmt.Printf("%s: %s\n", name, r.timeouts[name])
	}
}

func sortedKeys[V any](m map[string]V) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}
