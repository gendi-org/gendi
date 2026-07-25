package ir

import (
	di "github.com/gendi-org/gendi"
)

type unreachablePrunePhase struct{}

// Apply removes services not reachable from public services.
// Note: After tag desugaring, public tags are public services carrying the
// TagServicePrefix, so we only need to check Services, not tags.
func (p *unreachablePrunePhase) Apply(_ *di.Config, container *Container) error {
	reachable := map[string]bool{}
	var queue []*Service

	// Start from all public services (including desugared public tags)
	for _, svc := range container.Services {
		if svc != nil && svc.Public {
			if !reachable[svc.ID] {
				reachable[svc.ID] = true
				queue = append(queue, svc)
			}
		}
	}

	// BFS to find all reachable services
	for len(queue) > 0 {
		svc := queue[0]
		queue = queue[1:]
		if svc == nil {
			continue
		}
		for _, dep := range svc.Dependencies {
			if dep == nil || reachable[dep.ID] {
				continue
			}
			reachable[dep.ID] = true
			queue = append(queue, dep)
		}
	}

	// Remove unreachable services
	for id := range container.Services {
		if !reachable[id] {
			delete(container.Services, id)
		}
	}

	return nil
}
