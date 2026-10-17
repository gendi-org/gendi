package ir

import (
	"fmt"
	"go/types"

	di "github.com/gendi-org/gendi"
	"github.com/gendi-org/gendi/parameters"
	"github.com/gendi-org/gendi/srcloc"
	"github.com/gendi-org/gendi/xmaps"
)

// paramDefaultValidatorPhase checks every declared parameter default against
// the target type of each injection site by executing the standard caster,
// so conversion failures surface at generation time instead of runtime.
// Runtime-provided values remain checked at construction time.
type paramDefaultValidatorPhase struct{}

func (p *paramDefaultValidatorPhase) Apply(cfg *di.Config, container *Container) error {
	for _, id := range xmaps.OrderedKeys(container.Services) {
		svc := container.Services[id]
		if svc.Constructor == nil {
			continue
		}
		for i, arg := range svc.Constructor.Args {
			if err := p.validateArg(cfg, svc.ID, i, arg); err != nil {
				return err
			}
		}
	}
	return nil
}

func (p *paramDefaultValidatorPhase) validateArg(cfg *di.Config, svcID string, idx int, arg *Argument) error {
	if arg == nil {
		return nil
	}
	for _, child := range arg.Children() {
		if err := p.validateArg(cfg, svcID, idx, child); err != nil {
			return err
		}
	}
	if arg.Kind != ParamRefArg || arg.Parameter == nil {
		return nil
	}
	decl, ok := cfg.Parameters[arg.Parameter.Name]
	if !ok {
		return nil // runtime-only parameter; the value is checked at construction time
	}
	if err := castDefault(decl.Value, arg.Type); err != nil {
		return srcloc.Errorf(decl.SourceLoc, "service %q arg[%d]: parameter %q default: %v",
			svcID, idx, arg.Parameter.Name, err)
	}
	return nil
}

// castDefault runs the standard caster on a declared default exactly as the
// generated container will at runtime: the raw value fed here matches the
// representation rendered into the generated defaults map (int literals are
// emitted as int64) on every target architecture.
func castDefault(lit di.Literal, target types.Type) error {
	var raw any
	switch lit.Kind {
	case di.LiteralString:
		raw = lit.String()
	case di.LiteralInt:
		raw = lit.Int()
	case di.LiteralFloat:
		raw = lit.Float()
	case di.LiteralBool:
		raw = lit.Bool()
	default:
		return fmt.Errorf("unsupported default literal kind %d", lit.Kind)
	}
	kind, _, err := ParamScalarKind(target)
	if err != nil {
		return err
	}
	_, err = kind.Cast(parameters.StandardCaster{}, raw)
	return err
}
