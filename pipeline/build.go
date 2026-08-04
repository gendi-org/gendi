package pipeline

import (
	di "github.com/gendi-org/gendi"
	"github.com/gendi-org/gendi/ir"
	"github.com/gendi-org/gendi/typeres"
	"github.com/gendi-org/gendi/xmaps"
)

// Output is the compiled result of the config pipeline, ready for code generation.
type Output struct {
	Config       *di.Config
	IR           *ir.Container
	TypeResolver *typeres.Resolver
}

// Build compiles a DI config through internal passes, type loading, and IR building.
// The caller's config is not modified; passes operate on a clone.
// Options must be finalized before calling Build (via Options.Finalize()).
func Build(cfg *di.Config, opts Options) (*Output, error) {
	cfg, err := di.ApplyInternalPasses(cfg.Clone())
	if err != nil {
		return nil, err
	}

	// Re-populate Packages fields after passes may have added/modified services.
	refreshPackages(cfg)

	paths, candidatePaths := collectPackagePaths(cfg)
	typeResolver := typeres.NewResolver(opts.ModuleRoot, opts.BuildTags)
	if err := typeResolver.LoadPackagesWithCandidates(paths, candidatePaths); err != nil {
		return nil, err
	}

	irBuilder := ir.NewBuilder(typeResolver)
	irContainer, err := irBuilder.Build(cfg)
	if err != nil {
		return nil, err
	}

	return &Output{
		Config:       cfg,
		IR:           irContainer,
		TypeResolver: typeResolver,
	}, nil
}

// refreshPackages re-populates the Packages field on all config structs.
// This is needed after compiler passes that may add or modify services.
func refreshPackages(cfg *di.Config) {
	for name, tag := range cfg.Tags {
		tag.Packages = typeres.CollectTypePackages(tag.ElementType)
		cfg.Tags[name] = tag
	}
	for name, svc := range cfg.Services {
		svc.Packages = typeres.CollectTypePackages(svc.Type)
		svc.Constructor.Packages = typeres.CollectFuncPackages(svc.Constructor.Func)
		for i := range svc.Constructor.Args {
			refreshArgPackages(&svc.Constructor.Args[i])
		}
		cfg.Services[name] = svc
	}
}

// refreshArgPackages re-collects the packages an argument references,
// descending into map argument entries.
func refreshArgPackages(arg *di.Argument) {
	switch arg.Kind {
	case di.ArgGoRef:
		arg.Packages = typeres.CollectGoRefPackages(arg.Value)
	case di.ArgFieldAccessGo:
		arg.Packages = typeres.CollectFieldAccessGoPackages(arg.Value)
	}
	for _, child := range arg.Children() {
		refreshArgPackages(child)
	}
}

// collectPackagePaths returns the package paths a config requires. The second
// list holds candidate paths from field access on Go symbols, where the
// package/symbol boundary is ambiguous; they must be loaded leniently.
func collectPackagePaths(cfg *di.Config) (required, candidates []string) {
	seen := map[string]bool{}
	addAll := func(pkgs []string) {
		for _, p := range pkgs {
			if p != "" {
				seen[p] = true
			}
		}
	}
	candidateSet := map[string]bool{}

	for _, svc := range cfg.Services {
		addAll(svc.Packages)
		addAll(svc.Constructor.Packages)
		var addArg func(arg *di.Argument)
		addArg = func(arg *di.Argument) {
			if arg.Kind == di.ArgFieldAccessGo {
				for _, p := range arg.Packages {
					if p != "" {
						candidateSet[p] = true
					}
				}
			} else {
				addAll(arg.Packages)
			}
			for _, child := range arg.Children() {
				addArg(child)
			}
		}
		for i := range svc.Constructor.Args {
			addArg(&svc.Constructor.Args[i])
		}
	}
	for _, tag := range cfg.Tags {
		addAll(tag.Packages)
	}

	if hasTagsOrTaggedArgs(cfg) {
		seen["github.com/gendi-org/gendi/stdlib"] = true
	}

	for p := range candidateSet {
		if seen[p] {
			delete(candidateSet, p)
		}
	}

	return xmaps.OrderedKeys(seen), xmaps.OrderedKeys(candidateSet)
}

func hasTagsOrTaggedArgs(cfg *di.Config) bool {
	if len(cfg.Tags) > 0 {
		return true
	}
	for _, svc := range cfg.Services {
		if len(svc.Tags) > 0 {
			return true
		}
		for _, arg := range svc.Constructor.Args {
			if arg.Kind == di.ArgTagged {
				return true
			}
		}
	}
	return false
}
