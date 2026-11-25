package ir

import (
	"cmp"
	"fmt"
	"go/types"
	"slices"

	di "github.com/gendi-org/gendi"
	"github.com/gendi-org/gendi/xmaps"
)

const (
	// TagServicePrefix is the prefix for desugared tag service IDs
	TagServicePrefix = "__tagged_with."
	// StdlibPkgPath is the package of stdlib.MakeSlice, which builds the
	// collection of a tag. The pipeline loads this package before the IR
	// phases, and the generator inlines the calls to MakeSlice.
	StdlibPkgPath = "github.com/gendi-org/gendi/stdlib"
)

// tagDesugarPhase transforms tags into regular services with MakeSlice constructors.
// After this phase, container.tags will be empty and all TaggedArg arguments
// will be converted to ServiceRefArg pointing to the desugared tag services.
type tagDesugarPhase struct {
	resolver TypeResolver
}

// Apply transforms all tags into services
func (p *tagDesugarPhase) Apply(_ *di.Config, container *Container) error {
	if len(container.tags) == 0 {
		return nil
	}

	// First, link services to their tags and validate type compatibility
	if err := p.linkTaggedServices(container); err != nil {
		return err
	}

	// Process tags in deterministic order
	for _, tagName := range xmaps.OrderedKeys(container.tags) {
		tag := container.tags[tagName]

		if tag.ElementType == nil {
			// Skip tags without ElementType - they have no usages
			continue
		}

		// Create synthetic service for this tag
		svc, err := p.createTagService(tagName, tag)
		if err != nil {
			return fmt.Errorf("tag %q: %w", tagName, err)
		}

		container.Services[svc.ID] = svc
	}

	// Rewrite TaggedArg -> ServiceRefArg
	if err := p.rewriteTaggedArgs(container); err != nil {
		return err
	}

	// Clear tags - they are now services
	container.tags = make(map[string]*Tag)

	return nil
}

// linkTaggedServices links services to their tags and validates type compatibility
func (p *tagDesugarPhase) linkTaggedServices(container *Container) error {
	for _, id := range xmaps.OrderedKeys(container.Services) {
		svc := container.Services[id]
		for _, st := range svc.Tags {
			// Validate service type is assignable to tag's ElementType (if known)
			if st.Tag.ElementType != nil && svc.Type != nil {
				if !types.AssignableTo(svc.Type, st.Tag.ElementType) {
					return fmt.Errorf("service %q with tag %q: type %s is not assignable to %s",
						svc.ID, st.Tag.Name, svc.Type, st.Tag.ElementType)
				}
			}
			st.Tag.Services = append(st.Tag.Services, svc)
		}
	}
	return nil
}

// createTagService creates a synthetic service for a tag
func (p *tagDesugarPhase) createTagService(tagName string, tag *Tag) (*Service, error) {
	serviceID := TagServicePrefix + tagName

	// Sort services by priority if needed
	services := p.sortTagServices(tag)

	// Create MakeSlice constructor
	constructor, err := p.createMakeSliceConstructor(tag.ElementType, services)
	if err != nil {
		return nil, err
	}

	sliceType := types.NewSlice(tag.ElementType)

	// Build dependencies from constructor args
	deps := make([]*Service, len(services))
	copy(deps, services)

	return &Service{
		ID:           serviceID,
		Type:         sliceType,
		Constructor:  constructor,
		Shared:       false, // Always create fresh slice
		Public:       tag.Public,
		Tags:         nil, // Desugared services don't have tags
		Dependencies: deps,
	}, nil
}

// sortTagServices sorts services according to tag's SortBy attribute
func (p *tagDesugarPhase) sortTagServices(tag *Tag) []*Service {
	if len(tag.Services) == 0 {
		return nil
	}

	sorted := slices.Clone(tag.Services)

	if tag.SortBy != "" {
		slices.SortFunc(sorted, func(a, b *Service) int {
			va := getServiceTagSortKey(a, tag.Name)
			vb := getServiceTagSortKey(b, tag.Name)
			if va != vb {
				return cmp.Compare(vb, va) // Descending order
			}
			return cmp.Compare(a.ID, b.ID)
		})
	} else {
		slices.SortFunc(sorted, func(a, b *Service) int {
			return cmp.Compare(a.ID, b.ID)
		})
	}

	return sorted
}

// getServiceTagSortKey returns the sort key of a service's tag.
func getServiceTagSortKey(svc *Service, tagName string) int {
	for _, st := range svc.Tags {
		if st.Tag.Name == tagName {
			return st.SortKey
		}
	}
	return 0
}

// createMakeSliceConstructor creates a constructor using stdlib.MakeSlice
func (p *tagDesugarPhase) createMakeSliceConstructor(elemType types.Type, services []*Service) (*Constructor, error) {
	// Lookup stdlib.MakeSlice function
	fn, err := p.resolver.LookupFunc(StdlibPkgPath, "MakeSlice")
	if err != nil {
		return nil, fmt.Errorf("lookup stdlib.MakeSlice: %w", err)
	}

	// Format element type as string for instantiation
	elemTypeStr := types.TypeString(elemType, func(pkg *types.Package) string {
		return pkg.Path()
	})

	// Instantiate generic function
	sig, typeArgs, err := p.resolver.InstantiateFunc(fn, []string{elemTypeStr})
	if err != nil {
		return nil, fmt.Errorf("instantiate MakeSlice[%s]: %w", elemTypeStr, err)
	}

	// Build arguments - one ServiceRefArg per tagged service
	args := make([]*Argument, len(services))
	for i, svc := range services {
		args[i] = &Argument{
			Kind:    ServiceRefArg,
			Type:    svc.Type,
			Service: svc,
		}
	}

	// Get result type from signature
	resultType := sig.Results().At(0).Type()

	return &Constructor{
		Kind:         FuncConstructor,
		Func:         fn,
		TypeArgs:     typeArgs,
		Args:         args,
		Params:       []types.Type{resultType}, // variadic param is slice type
		ResultType:   resultType,
		ReturnsError: false,
		Variadic:     true,
	}, nil
}

// rewriteArgument recursively rewrites TaggedArg to ServiceRefArg, including within SpreadArg.
// Returns true if any rewriting was done.
func (p *tagDesugarPhase) rewriteArgument(container *Container, svcID string, argIndex int, arg *Argument) (bool, error) {
	switch arg.Kind {
	case TaggedArg:
		if arg.Tag == nil {
			return false, fmt.Errorf("service %q arg[%d]: TaggedArg has nil Tag", svcID, argIndex)
		}

		tagServiceID := TagServicePrefix + arg.Tag.Name
		tagSvc, ok := container.Services[tagServiceID]
		if !ok {
			return false, fmt.Errorf("service %q arg[%d]: desugared tag service %q not found", svcID, argIndex, tagServiceID)
		}

		// Rewrite to ServiceRefArg
		arg.Kind = ServiceRefArg
		arg.Service = tagSvc
		arg.Tag = nil
		return true, nil

	case SpreadArg:
		// Recursively rewrite inner argument
		if arg.Inner != nil {
			return p.rewriteArgument(container, svcID, argIndex, arg.Inner)
		}
		return false, nil

	default:
		return false, nil
	}
}

// rewriteTaggedArgs rewrites all TaggedArg arguments to ServiceRefArg.
// Dependencies will be rebuilt later by dependencyBuilderPhase.
func (p *tagDesugarPhase) rewriteTaggedArgs(container *Container) error {
	for _, svc := range container.Services {
		if svc.Constructor == nil {
			continue
		}

		for i, arg := range svc.Constructor.Args {
			_, err := p.rewriteArgument(container, svc.ID, i, arg)
			if err != nil {
				return err
			}
		}
	}

	return nil
}
