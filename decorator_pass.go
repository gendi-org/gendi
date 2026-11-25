package di

import (
	"sort"
	"strings"

	"github.com/gendi-org/gendi/srcloc"
	"github.com/gendi-org/gendi/xmaps"
)

// DecoratorPass transforms decorator services into plain services and aliases.
// This is an internal mandatory pass applied during the pipeline build after a user passes.
//
// Transformation example:
//
//	Input:
//	  base: { constructor: ... }
//	  dec: { decorates: "base", args: ["@.inner"] }
//	Output:
//	  dec.inner: { constructor: <original base constructor> }
//	  dec: { args: ["@dec.inner"] }
//	  base: { alias: "dec" }
type DecoratorPass struct{}

func (p *DecoratorPass) Name() string {
	return "decorator"
}

type decoratorPassState struct {
	baseToDecorators    map[string][]string
	decoratorToPriority map[string]int
}

func (s *decoratorPassState) popNext() (baseID string, decoratorID string, ok bool) {
	for _, base := range xmaps.OrderedKeys(s.baseToDecorators) {
		decorators := s.baseToDecorators[base]
		decoratorID = decorators[0]

		decorators = decorators[1:]
		s.baseToDecorators[base] = decorators
		if len(decorators) == 0 {
			delete(s.baseToDecorators, base)
		}

		return base, decoratorID, true
	}

	return "", "", false
}

func (p *DecoratorPass) Process(cfg *Config) (*Config, error) {
	state, err := p.buildState(cfg)
	if err != nil {
		return nil, err
	}

	for {
		baseID, decoratorID, ok := state.popNext()
		if !ok {
			break
		}

		if err := p.expandOne(cfg, baseID, decoratorID); err != nil {
			return nil, err
		}
	}

	return cfg, nil
}

func (p *DecoratorPass) buildState(cfg *Config) (*decoratorPassState, error) {
	state := &decoratorPassState{
		baseToDecorators:    make(map[string][]string),
		decoratorToPriority: make(map[string]int),
	}

	for _, id := range xmaps.OrderedKeys(cfg.Services) {
		svc := cfg.Services[id]
		if strings.HasSuffix(id, ".inner") {
			return nil, srcloc.Errorf(svc.SourceLoc, "service ID %q cannot use reserved .inner suffix", id)
		}
		if svc.Decorates == "" {
			continue
		}

		base, ok := cfg.Services[svc.Decorates]
		if !ok {
			return nil, srcloc.Errorf(svc.SourceLoc, "decorator %q decorates unknown service %q", id, svc.Decorates)
		}
		if base.Decorates != "" {
			return nil, srcloc.Errorf(svc.SourceLoc, "decorator %q cannot be decorated by %q", svc.Decorates, id)
		}
		if err := p.validateDecoratorArgs(id, svc); err != nil {
			return nil, err
		}

		state.baseToDecorators[svc.Decorates] = append(state.baseToDecorators[svc.Decorates], id)
		state.decoratorToPriority[id] = svc.DecorationPriority
	}

	// Sort decorators by priority, then by ID for deterministic tie-breaking.
	for baseID, decs := range state.baseToDecorators {
		if len(decs) <= 1 {
			continue
		}
		sort.Slice(decs, func(i, j int) bool {
			pi := state.decoratorToPriority[decs[i]]
			pj := state.decoratorToPriority[decs[j]]
			if pi != pj {
				return pi < pj
			}
			return decs[i] < decs[j]
		})
		state.baseToDecorators[baseID] = decs
	}

	return state, nil
}

func (p *DecoratorPass) validateDecoratorArgs(decoratorID string, svc Service) error {
	for i, arg := range svc.Constructor.Args {
		var form string
		switch {
		case arg.Kind == ArgSpread && arg.Value == "@.inner":
			form = "!spread:@.inner"
		case arg.Kind == ArgFieldAccessService && (arg.Value == ".inner" || strings.HasPrefix(arg.Value, ".inner.")):
			form = "!field:@" + arg.Value
		default:
			continue
		}
		loc := arg.SourceLoc
		if loc == nil {
			loc = svc.SourceLoc
		}
		return srcloc.Errorf(loc, "decorator %q arg[%d]: %s is not supported; use @.inner directly", decoratorID, i, form)
	}

	return nil
}

func (p *DecoratorPass) expandOne(cfg *Config, baseID, decoratorID string) error {
	baseSvc := cfg.Services[baseID]
	decoratorSvc := cfg.Services[decoratorID]

	// Determine inner service (reuse existing alias or create new)
	innerID, innerSvc, err := p.resolveInnerService(cfg, baseID, baseSvc, decoratorID)
	if err != nil {
		return err
	}

	// Transform base into alias to current decorator. The declared type and
	// source location are preserved for validation and error reporting.
	aliasService := Service{
		Alias:         decoratorID,
		Type:          baseSvc.Type,
		Public:        baseSvc.Public, // Preserve public flag
		Autoconfigure: false,
		SourceLoc:     baseSvc.SourceLoc,
	}

	// Rewrite @.inner args in decorator
	decoratorSvc.Constructor.Args = p.rewriteInnerArgs(decoratorSvc.Constructor.Args, innerID)
	decoratorSvc.Decorates = "" // Clear decoration marker

	// Update config (need to write back all modified services)
	cfg.Services[innerID] = innerSvc
	cfg.Services[decoratorID] = decoratorSvc
	cfg.Services[baseID] = aliasService

	return nil
}

func (p *DecoratorPass) resolveInnerService(cfg *Config, baseID string, baseSvc Service, decoratorID string) (string, Service, error) {
	// If base is already an alias, reuse the target instead of creating new inner
	if baseSvc.Alias != "" {
		innerID := baseSvc.Alias
		innerSvc, ok := cfg.Services[innerID]
		if !ok {
			// The same error as for an alias without a decorator. Without
			// this check, the pass adds an empty service with the ID of the
			// target, and the generator reports that service instead.
			return "", Service{}, srcloc.Errorf(baseSvc.SourceLoc, "service %q alias target %q not found", baseID, innerID)
		}
		innerSvc.Autoconfigure = false
		cfg.Services[innerID] = innerSvc
		return innerID, innerSvc, nil
	}

	// Create new inner service (clone of base)
	innerID := decoratorID + ".inner"
	innerSvc := Service{
		Type:        baseSvc.Type,
		Constructor: baseSvc.Constructor,
		Shared:      baseSvc.Shared,
		Public:      false, // Inner services are never public
		// Explicit base tags stay on the inner (undecorated) definition,
		// mirroring Symfony's decoration semantics.
		Autoconfigure: false,
		Tags:          baseSvc.Tags,
		SourceLoc:     baseSvc.SourceLoc,
	}
	// Store the new inner service
	cfg.Services[innerID] = innerSvc
	return innerID, innerSvc, nil
}

func (p *DecoratorPass) rewriteInnerArgs(args []Argument, innerServiceID string) []Argument {
	result := make([]Argument, len(args))
	for i, arg := range args {
		if arg.Kind != ArgInner {
			result[i] = arg
			continue
		}
		result[i] = Argument{
			Kind:  ArgServiceRef,
			Value: innerServiceID,
		}
	}
	return result
}
