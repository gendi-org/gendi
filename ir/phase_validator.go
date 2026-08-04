package ir

import (
	"errors"
	"fmt"
	"go/types"
	"strings"

	di "github.com/gendi-org/gendi"
	"github.com/gendi-org/gendi/xmaps"
)

// validatorPhase validates the IR for correctness
type validatorPhase struct{}

// Apply runs all validation checks
func (v *validatorPhase) Apply(_ *di.Config, container *Container) error {
	if err := v.validatePublicServices(container); err != nil {
		return err
	}
	if err := v.validateArgumentTypes(container); err != nil {
		return err
	}
	if err := v.detectCycles(container); err != nil {
		return err
	}
	return nil
}

// validateArgumentTypes ensures referenced services fit their parameter
// types. Argument resolution stamps each argument with the parameter type but
// does not compare it against the referenced service's actual type, which may
// not be resolved yet at that point.
func (v *validatorPhase) validateArgumentTypes(container *Container) error {
	for _, id := range xmaps.OrderedKeys(container.Services) {
		svc := container.Services[id]
		if svc.Constructor == nil {
			continue
		}
		for i, arg := range svc.Constructor.Args {
			if err := v.validateArgumentType(svc, i, arg); err != nil {
				return err
			}
		}
	}
	return nil
}

func (v *validatorPhase) validateArgumentType(svc *Service, idx int, arg *Argument) error {
	if arg.Kind == ServiceRefArg {
		dep := arg.Service
		if dep != nil && dep.Type != nil && arg.Type != nil &&
			!types.AssignableTo(dep.Type, arg.Type) &&
			// Slices with assignable element types are converted elementwise by
			// the generator (desugared tagged collections rely on this).
			!v.slicesConvertible(dep.Type, arg.Type) {
			return fmt.Errorf("service %q arg[%d]: service %q type %s is not assignable to %s",
				svc.ID, idx, dep.ID, dep.Type, arg.Type)
		}
	}
	for _, child := range arg.children() {
		if err := v.validateArgumentType(svc, idx, child); err != nil {
			return err
		}
	}
	return nil
}

func (v *validatorPhase) slicesConvertible(from, to types.Type) bool {
	fromSlice, fromOk := from.Underlying().(*types.Slice)
	toSlice, toOk := to.Underlying().(*types.Slice)
	return fromOk && toOk && types.AssignableTo(fromSlice.Elem(), toSlice.Elem())
}

// validatePublicServices ensures at least one public service exists.
// Note: After tag desugaring, public tags become public services with !tagged: prefix.
func (v *validatorPhase) validatePublicServices(container *Container) error {
	for _, svc := range container.Services {
		if svc.Public {
			return nil
		}
	}
	return errors.New("at least one public service or tag is required")
}

// detectCyclesDFS performs DFS-based cycle detection on a service graph.
// It accepts a neighbor function to traverse different types of relationships.
func (v *validatorPhase) detectCyclesDFS(
	services map[string]*Service,
	getNeighbors func(*Service) []*Service,
	errorPrefix string,
) error {
	visited := make(map[string]bool)
	stack := make(map[string]bool)

	var dfs func(svc *Service, path []string) error
	dfs = func(svc *Service, path []string) error {
		if svc == nil {
			return nil
		}
		if stack[svc.ID] {
			cycle := append(path, svc.ID)
			return fmt.Errorf("%s: %s", errorPrefix, strings.Join(cycle, " -> "))
		}
		if visited[svc.ID] {
			return nil
		}
		visited[svc.ID] = true
		stack[svc.ID] = true

		for _, neighbor := range getNeighbors(svc) {
			if err := dfs(neighbor, append(path, svc.ID)); err != nil {
				return err
			}
		}

		stack[svc.ID] = false
		return nil
	}

	for _, svc := range services {
		if err := dfs(svc, nil); err != nil {
			return err
		}
	}
	return nil
}

// detectCycles detects circular dependencies using DFS
func (v *validatorPhase) detectCycles(container *Container) error {
	return v.detectCyclesDFS(
		container.Services,
		func(svc *Service) []*Service { return svc.Dependencies },
		"circular dependency",
	)
}
