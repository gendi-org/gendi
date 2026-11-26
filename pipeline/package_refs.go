package pipeline

import (
	"errors"
	"fmt"
	"slices"

	di "github.com/gendi-org/gendi"
	"github.com/gendi-org/gendi/ir"
	"github.com/gendi-org/gendi/srcloc"
	"github.com/gendi-org/gendi/typeres"
	"github.com/gendi-org/gendi/xmaps"
)

// packageRef is a place in a config that references a package.
type packageRef struct {
	loc *srcloc.Location
	// what names the config element in the form of the IR errors, for
	// example `service "x" constructor.func`.
	what string
}

// firstTagUse returns the first place in cfg that uses tags, in a
// deterministic order. Tagged collections need the stdlib package, but no
// config element names it.
func firstTagUse(cfg *di.Config) (packageRef, bool) {
	const need = "tags need the gendi module"
	if names := xmaps.OrderedKeys(cfg.Tags); len(names) > 0 {
		name := names[0]
		return packageRef{cfg.Tags[name].SourceLoc, fmt.Sprintf("tag %q: %s", name, need)}, true
	}
	for _, id := range xmaps.OrderedKeys(cfg.Services) {
		svc := cfg.Services[id]
		// An alias cannot have tags, and the IR rejects them. Such tags do
		// not need stdlib: if they do, the error about the module hides
		// the error about the alias.
		if len(svc.Tags) > 0 && svc.Alias == "" {
			st := svc.Tags[0]
			return packageRef{st.SourceLoc, fmt.Sprintf("service %q tag %q: %s", id, st.Name, need)}, true
		}
		for i, arg := range svc.Constructor.Args {
			if arg.Kind == di.ArgTagged {
				return packageRef{arg.SourceLoc, fmt.Sprintf("service %q arg[%d]: %s", id, i, need)}, true
			}
		}
	}
	return packageRef{}, false
}

// packageRefs maps a package path to the first place in a config that
// references it. The package loader knows only the path of a package that
// fails to load; packageRefs gives its error a location in the config.
type packageRefs map[string]packageRef

func newPackageRefs(cfg *di.Config) packageRefs {
	refs := packageRefs{}
	for _, id := range xmaps.OrderedKeys(cfg.Services) {
		svc := cfg.Services[id]
		refs.add(svc.Packages, packageRef{svc.SourceLoc, fmt.Sprintf("service %q type", id)})
		refs.add(svc.Constructor.Packages, packageRef{svc.Constructor.SourceLoc, fmt.Sprintf("service %q constructor.func", id)})
		for i, arg := range svc.Constructor.Args {
			refs.add(arg.Packages, packageRef{arg.SourceLoc, fmt.Sprintf("service %q arg[%d]", id, i)})
		}
	}
	for _, name := range xmaps.OrderedKeys(cfg.Tags) {
		tag := cfg.Tags[name]
		refs.add(tag.Packages, packageRef{tag.SourceLoc, fmt.Sprintf("tag %q element_type", name)})
	}
	if ref, ok := firstTagUse(cfg); ok {
		refs.add([]string{ir.StdlibPkgPath}, ref)
	}
	return refs
}

// add records ref for each path that has no reference yet.
func (r packageRefs) add(paths []string, ref packageRef) {
	for _, p := range paths {
		if _, ok := r[p]; !ok && p != "" {
			r[p] = ref
		}
	}
}

// locate gives each package load error in err the location of the first
// reference to its package. It keeps all other errors as they are.
func (r packageRefs) locate(err error) error {
	errs := []error{err}
	if joined, ok := err.(interface{ Unwrap() []error }); ok {
		errs = slices.Clone(joined.Unwrap())
	}
	for i, e := range errs {
		var pkgErr *typeres.PackageError
		if !errors.As(e, &pkgErr) {
			continue
		}
		if ref, ok := r[pkgErr.Path]; ok {
			errs[i] = srcloc.WrapError(ref.loc, ref.what, e)
		}
	}
	if len(errs) == 1 {
		return errs[0]
	}
	return errors.Join(errs...)
}
