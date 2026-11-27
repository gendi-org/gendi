package ir

import (
	"cmp"
	"slices"

	di "github.com/gendi-org/gendi"
	"github.com/gendi-org/gendi/xmaps"
)

// dependencyBuilderPhase builds service dependency graph.
// This phase must run after tagDesugarPhase, so every TaggedArg already
// refers to the sequence service of its tag.
type dependencyBuilderPhase struct{}

// Apply builds the dependency graph for all services
func (r *dependencyBuilderPhase) Apply(_ *di.Config, container *Container) error {
	for _, id := range xmaps.OrderedKeys(container.Services) {
		svc := container.Services[id]
		deps := make(map[string]*Service)
		for dependency := range svc.dependencyRefs() {
			deps[dependency.ID] = dependency
		}

		svc.Dependencies = make([]*Service, 0, len(deps))
		for _, dep := range deps {
			svc.Dependencies = append(svc.Dependencies, dep)
		}

		slices.SortFunc(svc.Dependencies, func(a, b *Service) int {
			return cmp.Compare(a.ID, b.ID)
		})
	}
	return nil
}
