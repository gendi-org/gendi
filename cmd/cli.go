package cmd

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"

	di "github.com/gendi-org/gendi"
	"github.com/gendi-org/gendi/pipeline"
	"github.com/gendi-org/gendi/srcloc"
	"github.com/gendi-org/gendi/stdlib"
	"github.com/gendi-org/gendi/yaml"
)

func BuiltinSelectablePasses() []di.Pass {
	return []di.Pass{
		&stdlib.SLogPass{},
		&di.ExposeAllPass{},
	}
}

// WriteTargetFile writes data to path atomically: a failed write leaves the
// previous file unchanged and never a truncated one.
//
// The function writes a temporary file in the directory of path and renames
// it. If path is a symlink, the rename replaces the link and does not write
// to its target. The name of the temporary file starts with a dot and does not
// end in ".go", so the go tool ignores a file that a killed process leaves.
func WriteTargetFile(path string, data []byte) (err error) {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("create output dir: %w", err)
	}

	tmp, err := os.CreateTemp(dir, "."+filepath.Base(path)+".*.tmp")
	if err != nil {
		return fmt.Errorf("create temp file: %w", err)
	}
	tmpPath := tmp.Name()
	defer func() {
		if err != nil {
			_ = tmp.Close()
			_ = os.Remove(tmpPath)
		}
	}()

	if _, err := tmp.Write(data); err != nil {
		return fmt.Errorf("write temp file: %w", err)
	}
	// CreateTemp creates the file with mode 0600. Generated source is not secret.
	if err := tmp.Chmod(0o644); err != nil {
		return fmt.Errorf("set temp file mode: %w", err)
	}
	if err := tmp.Sync(); err != nil {
		return fmt.Errorf("sync temp file: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("close temp file: %w", err)
	}
	if err := os.Rename(tmpPath, path); err != nil {
		return fmt.Errorf("replace target file: %w", err)
	}

	return nil
}

func Generate(cfg Config, passes []di.Pass) error {
	if err := cfg.Finalize(); err != nil {
		return fmt.Errorf("config finalize: %w", err)
	}

	boundary, err := yaml.DefaultBoundary(cfg.ConfigPath)
	if err != nil {
		return fmt.Errorf("derive load boundary: %w", err)
	}
	diCfg, err := yaml.LoadConfig(cfg.ConfigPath, boundary, cfg.Options.ModuleRoot)
	if err != nil {
		return srcloc.AddContext(err, "load config")
	}

	diCfg, err = di.ApplyPasses(diCfg, passes)
	if err != nil {
		return fmt.Errorf("apply passes: %w", err)
	}

	code, err := pipeline.Emit(diCfg, cfg.Options)
	if err != nil {
		return fmt.Errorf("generate: %w", err)
	}

	// After Finalize(), cfg.Options.Out is the full output file path
	if err := WriteTargetFile(cfg.Options.Out, code); err != nil {
		return fmt.Errorf("write target file: %w", err)
	}

	if cfg.Options.Verbose {
		_, _ = fmt.Fprintf(os.Stderr, "generated %s\n", cfg.Options.Out)
	}

	return nil
}

// Run executes the full GenDI workflow with compiler passes
func Run(flags *flag.FlagSet, passes, selectablePasses []di.Pass) error {
	var cfg Config
	cfg.RegisterFlags(flags)

	if err := flags.Parse(os.Args[1:]); err != nil {
		return fmt.Errorf("parse flags: %w", err)
	}

	resolvedPasses, err := cfg.resolvePasses(passes, selectablePasses)
	if err != nil {
		return fmt.Errorf("resolve passes: %w", err)
	}

	return Generate(cfg, resolvedPasses)
}

func PrintErrorAndExit(err error) {
	if err == nil {
		os.Exit(0)
	}

	errRenderer := srcloc.NewRenderer()
	_, _ = os.Stderr.WriteString(errRenderer.RenderError(err, 4))

	os.Exit(1)
}

func MustRun(flags *flag.FlagSet, passes, selectablePasses []di.Pass) {
	PrintErrorAndExit(Run(flags, passes, selectablePasses))
}
