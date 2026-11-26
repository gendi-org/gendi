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
//	  base: { constructor: ..., tags: [a, b] }
//	  dec: { decorates: "base", args: ["@.inner"], tags: [{name: b, x: 1}] }
//	Output:
//	  dec.inner: { constructor: <original base constructor> }
//	  dec: { args: ["@dec.inner"], tags: [a, {name: b, x: 1}], DecoratedID: "base" }
//	  base: { alias: "dec" }
//
// Tags describe a service, not a name. The decorator replaces the base, so
// the tags of the base move to the decorator, as in Symfony. The entries of
// the decorator replace all entries of the base with the same tag name. In a
// chain, the tags of the base move to the outermost decorator, and each
// decorator keeps its own tags: an inner decorator is still a service under
// its own ID. Symfony also moves the tags of an inner decorator; GenDI does
// not. When the base is an alias that the user declared, no tags move: the
// decorator replaces only that name, and the target stays a service under
// its own ID. An alias cannot have tags: the tags of such an alias stay on
// it, and the IR rejects them.
//
// The outermost decorator gets the base ID in DecoratedID, also when the base
// is an alias. An inner decorator does not get it: it keeps its own tags under
// its own ID. With the base ID, a tag of the same name on an inner and on the
// outermost decorator gives the same key twice.
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
	if err := p.moveToOutermost(cfg, state); err != nil {
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

	// The pass applies the decorators in this order, so the first one is the
	// innermost. A higher priority comes last, so it is the outer decorator.
	// Symfony uses the reverse order. Symfony breaks a tie by the order of
	// definition; GenDI has no such order, so the lower ID comes first.
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

// mergeTags returns the tags of a decorator after it takes the tags of the
// service that it decorates. An entry of the decorator replaces all moved
// entries with the same name, as array_merge does in Symfony.
func (p *DecoratorPass) mergeTags(moved, own []ServiceTag) []ServiceTag {
	if len(moved) == 0 {
		return own
	}

	ownNames := make(map[string]struct{}, len(own))
	for _, tag := range own {
		ownNames[tag.Name] = struct{}{}
	}

	result := make([]ServiceTag, 0, len(moved)+len(own))
	for _, tag := range moved {
		if _, ok := ownNames[tag.Name]; !ok {
			result = append(result, tag)
		}
	}
	return append(result, own...)
}

// moveToOutermost gives the outermost decorator of each base the base ID in
// DecoratedID. When the base is not an alias, the tags of the base also move to
// that decorator. The decorators of a base are in the order of application, so
// the outermost decorator is the last one.
//
// An entry of the decorator can replace an entry of the base, and then the IR
// does not see the entry of the base. So the pass checks the id attribute of
// each entry of the base first: an invalid entry must not disappear without an
// error.
func (p *DecoratorPass) moveToOutermost(cfg *Config, state *decoratorPassState) error {
	for _, baseID := range xmaps.OrderedKeys(state.baseToDecorators) {
		decorators := state.baseToDecorators[baseID]
		outerID := decorators[len(decorators)-1]
		outer := cfg.Services[outerID]
		outer.DecoratedID = baseID

		base := cfg.Services[baseID]
		if base.Alias == "" && len(base.Tags) > 0 {
			for _, tag := range base.Tags {
				if _, _, err := tag.IDAttribute(baseID); err != nil {
					return err
				}
			}
			outer.Tags = p.mergeTags(base.Tags, outer.Tags)
			base.Tags = nil
			cfg.Services[baseID] = base
		}
		cfg.Services[outerID] = outer
	}
	return nil
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
	if baseSvc.Alias != "" {
		// An alias cannot have tags. Keep the tags of a user alias, so that
		// the service phase of the IR rejects them. If the pass drops them,
		// the error is lost.
		aliasService.Tags = baseSvc.Tags
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
		// The tags of the base moved to the outermost decorator, so the
		// inner service gets none. See moveToOutermost.
		Autoconfigure: false,
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
