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
		return nil, newPackageRefs(cfg).locate(err)
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
		for i, arg := range svc.Constructor.Args {
			switch arg.Kind {
			case di.ArgGoRef:
				arg.Packages = typeres.CollectGoRefPackages(arg.Value)
			case di.ArgFieldAccessGo:
				arg.Packages = typeres.CollectFieldAccessGoPackages(arg.Value)
			}
			svc.Constructor.Args[i] = arg
		}
		cfg.Services[name] = svc
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
		for _, arg := range svc.Constructor.Args {
			if arg.Kind == di.ArgFieldAccessGo {
				for _, p := range arg.Packages {
					if p != "" {
						candidateSet[p] = true
					}
				}
				continue
			}
			addAll(arg.Packages)
		}
	}
	for _, tag := range cfg.Tags {
		addAll(tag.Packages)
	}

	if _, ok := firstTagUse(cfg); ok {
		seen[ir.StdlibPkgPath] = true
	}

	for p := range candidateSet {
		if seen[p] {
			delete(candidateSet, p)
		}
	}

	return xmaps.OrderedKeys(seen), xmaps.OrderedKeys(candidateSet)
}
