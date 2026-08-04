package ir

import (
	di "github.com/gendi-org/gendi"
)

type unusedParamPrunePhase struct{}

// Apply removes parameters not referenced by any remaining service
// from both the IR container and the source config.
func (p *unusedParamPrunePhase) Apply(cfg *di.Config, container *Container) error {
	used := make(map[string]bool)

	var markUsed func(arg *Argument)
	markUsed = func(arg *Argument) {
		if arg == nil {
			return
		}
		if arg.Kind == ParamRefArg && arg.Parameter != nil {
			used[arg.Parameter.Name] = true
		}
		for _, child := range arg.children() {
			markUsed(child)
		}
	}

	for _, svc := range container.Services {
		if svc.Constructor == nil {
			continue
		}
		for _, arg := range svc.Constructor.Args {
			markUsed(arg)
		}
	}

	for name := range container.Parameters {
		if !used[name] {
			delete(container.Parameters, name)
		}
	}

	for name := range cfg.Parameters {
		if !used[name] {
			delete(cfg.Parameters, name)
		}
	}

	return nil
}
