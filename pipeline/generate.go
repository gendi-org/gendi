package pipeline

import (
	di "github.com/gendi-org/gendi"
	"github.com/gendi-org/gendi/generator"
)

// Emit compiles a DI config and emits generated Go source code.
// Options must be finalized before calling Emit (via Options.Finalize()).
func Emit(cfg *di.Config, opts Options) ([]byte, error) {
	compiled, err := Build(cfg, opts)
	if err != nil {
		return nil, err
	}

	irConverter := generator.NewIRConverter(compiled.TypeResolver)
	genCtx, err := irConverter.Convert(compiled.IR)
	if err != nil {
		return nil, err
	}

	identGenerator := generator.NewIdentGenerator()

	return generator.NewGenerator(identGenerator).
		Generate(
			generator.Options{
				Package:       opts.Package,
				Container:     opts.Container,
				BuildTags:     opts.BuildTags,
				OutputPkgPath: opts.OutputPkgPath,
			},
			compiled.Config, genCtx,
		)
}
