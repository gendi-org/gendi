package ir

import (
	"cmp"
	"fmt"
	"go/types"
	"slices"

	di "github.com/gendi-org/gendi"
	"github.com/gendi-org/gendi/srcloc"
	"github.com/gendi-org/gendi/xmaps"
)

const (
	// tagServicePrefix is the ID prefix of the sequence service of a tag.
	tagServicePrefix = "__tagged_with."
	// StdlibPkgPath is the package of stdlib.MakeSeq2, which builds the
	// collection of a tag. The pipeline loads this package before the IR
	// phases, and the generator inlines the calls to MakeSeq2.
	StdlibPkgPath = "github.com/gendi-org/gendi/stdlib"
)

// tagDesugarPhase transforms each tag into one regular service: a sequence of
// key and service, built by stdlib.MakeSeq2. The generator converts the
// sequence to the shape of each consumer at the injection site.
// After this phase, container.tags is empty and every TaggedArg refers to
// the sequence service of its tag through Argument.Service.
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

	keyedTags := p.keyedTags(container)

	// Process tags in deterministic order
	for _, tagName := range xmaps.OrderedKeys(container.tags) {
		tag := container.tags[tagName]

		if tag.ElementType == nil {
			// Skip tags without ElementType - they have no usages
			continue
		}

		entries := p.sortEntries(tag)
		// A slice consumer does not see the keys, so a duplicate key matters
		// only for a map, a sequence or the public getter.
		if keyedTags[tagName] || tag.Public {
			if err := p.validateKeys(tag, entries); err != nil {
				return err
			}
		}

		svc, err := p.createTagService(tag, entries)
		if err != nil {
			return fmt.Errorf("tag %q: %w", tagName, err)
		}
		container.Services[svc.ID] = svc
	}

	if err := p.linkTaggedArgs(container); err != nil {
		return err
	}

	// Clear tags - they are now services
	container.tags = make(map[string]*Tag)

	return nil
}

// keyedTags returns the names of the tags that a constructor consumes as a
// map or a sequence.
func (p *tagDesugarPhase) keyedTags(container *Container) map[string]bool {
	result := make(map[string]bool)
	for _, svc := range container.Services {
		if svc.Constructor == nil {
			continue
		}
		for _, arg := range svc.Constructor.Args {
			if arg.Kind == SpreadArg && arg.Inner != nil {
				arg = arg.Inner
			}
			if arg.Kind == TaggedArg && arg.Tag != nil && arg.TagShape.Keyed() {
				result[arg.Tag.Name] = true
			}
		}
	}
	return result
}

// linkTaggedServices links the tag entries of the services to their tags and
// validates type compatibility.
func (p *tagDesugarPhase) linkTaggedServices(container *Container) error {
	for _, id := range xmaps.OrderedKeys(container.Services) {
		svc := container.Services[id]
		for _, st := range svc.Tags {
			// Validate service type is assignable to tag's ElementType (if known)
			if st.Tag.ElementType != nil && svc.Type != nil {
				if !types.AssignableTo(svc.Type, st.Tag.ElementType) {
					return srcloc.Errorf(st.SourceLoc, "service %q with tag %q: type %s is not assignable to %s",
						svc.ID, st.Tag.Name, svc.Type, st.Tag.ElementType)
				}
			}
			st.Tag.Entries = append(st.Tag.Entries, st)
		}
	}
	return nil
}

// validateKeys rejects a repeated key: a map would keep only one of the
// entries. The entries are in collection order.
func (p *tagDesugarPhase) validateKeys(tag *Tag, entries []*ServiceTag) error {
	first := make(map[string]*ServiceTag, len(entries))
	for _, entry := range entries {
		prev, ok := first[entry.Key]
		if !ok {
			first[entry.Key] = entry
			continue
		}
		// An autoconfigured entry has no location; the other entry has one,
		// because two service IDs cannot be equal.
		loc := entry.SourceLoc
		if loc == nil {
			loc = prev.SourceLoc
		}
		if prev.Service == entry.Service {
			return srcloc.Errorf(loc, "tag %q: service %q has the same key %q in two entries",
				tag.Name, entry.Service.ID, entry.Key)
		}
		return srcloc.Errorf(loc, "tag %q: services %q and %q have the same key %q",
			tag.Name, prev.Service.ID, entry.Service.ID, entry.Key)
	}
	return nil
}

func (p *tagDesugarPhase) createTagService(tag *Tag, entries []*ServiceTag) (*Service, error) {
	services := make([]*Service, len(entries))
	keys := make([]string, len(entries))
	for i, entry := range entries {
		services[i] = entry.Service
		keys[i] = entry.Key
	}
	constructor, err := p.createConstructor(tag.ElementType, services, keys)
	if err != nil {
		return nil, err
	}

	return &Service{
		ID:           tagServicePrefix + tag.Name,
		Type:         constructor.ResultType,
		Constructor:  constructor,
		Shared:       false, // Always create a fresh collection
		Public:       tag.Public,
		Tags:         nil, // Desugared services don't have tags
		Dependencies: slices.Clone(services),
	}, nil
}

// sortEntries returns the entries of the tag in collection order: descending
// by the sort_by attribute when the tag has one, then by service ID, then by
// key, which orders two entries of one service.
func (p *tagDesugarPhase) sortEntries(tag *Tag) []*ServiceTag {
	sorted := slices.Clone(tag.Entries)
	slices.SortFunc(sorted, func(a, b *ServiceTag) int {
		if tag.SortBy != "" && a.SortKey != b.SortKey {
			return cmp.Compare(b.SortKey, a.SortKey) // Descending order
		}
		return cmp.Or(cmp.Compare(a.Service.ID, b.Service.ID), cmp.Compare(a.Key, b.Key))
	})
	return sorted
}

// createConstructor creates a call of stdlib.MakeSeq2. The keys are its first
// argument, the services are the variadic values.
func (p *tagDesugarPhase) createConstructor(elemType types.Type, services []*Service, keys []string) (*Constructor, error) {
	fn, err := p.resolver.LookupFunc(StdlibPkgPath, "MakeSeq2")
	if err != nil {
		return nil, fmt.Errorf("lookup stdlib.MakeSeq2: %w", err)
	}

	// Format element type as string for instantiation
	elemTypeStr := types.TypeString(elemType, func(pkg *types.Package) string {
		return pkg.Path()
	})

	// Instantiate generic function
	sig, typeArgs, err := p.resolver.InstantiateFunc(fn, []string{elemTypeStr})
	if err != nil {
		return nil, fmt.Errorf("instantiate MakeSeq2[%s]: %w", elemTypeStr, err)
	}

	args := make([]*Argument, 0, len(services)+1)
	args = append(args, &Argument{
		Kind:    LiteralArg,
		Type:    types.NewSlice(types.Typ[types.String]),
		Literal: LiteralValue{Type: StringsLiteral, Value: keys},
	})
	for _, svc := range services {
		args = append(args, &Argument{
			Kind:    ServiceRefArg,
			Type:    svc.Type,
			Service: svc,
		})
	}

	params := make([]types.Type, sig.Params().Len())
	for i := range params {
		params[i] = sig.Params().At(i).Type() // the variadic param is the slice type
	}

	return &Constructor{
		Kind:         FuncConstructor,
		Func:         fn,
		TypeArgs:     typeArgs,
		Args:         args,
		Params:       params,
		ResultType:   sig.Results().At(0).Type(),
		ReturnsError: false,
		Variadic:     true,
	}, nil
}

// linkTaggedArgs points every TaggedArg, also inside a SpreadArg, at the
// sequence service of its tag. Dependencies are rebuilt later by
// dependencyBuilderPhase.
func (p *tagDesugarPhase) linkTaggedArgs(container *Container) error {
	for _, id := range xmaps.OrderedKeys(container.Services) {
		svc := container.Services[id]
		if svc.Constructor == nil {
			continue
		}

		for i, arg := range svc.Constructor.Args {
			if arg.Kind == SpreadArg && arg.Inner != nil {
				arg = arg.Inner
			}
			if arg.Kind != TaggedArg {
				continue
			}
			if arg.Tag == nil {
				return fmt.Errorf("service %q arg[%d]: TaggedArg has nil Tag", svc.ID, i)
			}
			tagSvc, ok := container.Services[tagServicePrefix+arg.Tag.Name]
			if !ok {
				return fmt.Errorf("service %q arg[%d]: sequence service of tag %q not found", svc.ID, i, arg.Tag.Name)
			}
			arg.Service = tagSvc
		}
	}

	return nil
}
