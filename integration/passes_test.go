package integration

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"golang.org/x/tools/imports"

	di "github.com/gendi-org/gendi"
	"github.com/gendi-org/gendi/pipeline"
	"github.com/gendi-org/gendi/stdlib"
	"github.com/gendi-org/gendi/yaml"
)

// TestApplyPassesEndToEnd generates, compiles and runs a container built with
// the built-in SLogPass and ExposeAllPass applied, verifying both passes take
// effect at runtime.
func TestApplyPassesEndToEnd(t *testing.T) {
	const testDir = "testdata/passes_slog_expose"
	tmpDir := prepareTestDir(t, testDir)
	configPath := filepath.Join(tmpDir, "gendi.yaml")

	cfg, err := yaml.LoadConfig(configPath, tmpDir, tmpDir)
	if err != nil {
		t.Fatalf("failed to load config: %v", err)
	}

	cfg, err = di.ApplyPasses(cfg, []di.Pass{&stdlib.SLogPass{}, &di.ExposeAllPass{}})
	if err != nil {
		t.Fatalf("failed to apply passes: %v", err)
	}

	opts := pipeline.Options{Out: tmpDir, Package: "main", ModulePath: "test", ModuleRoot: tmpDir}
	if err := opts.Finalize(); err != nil {
		t.Fatalf("failed to finalize options: %v", err)
	}

	code, err := pipeline.Emit(cfg, opts)
	if err != nil {
		t.Fatalf("failed to generate code: %v", err)
	}

	formatted, err := imports.Process("container_gen.go", code, nil)
	if err != nil {
		t.Fatalf("failed to format generated code: %v", err)
	}

	if err := os.WriteFile(filepath.Join(tmpDir, "container_gen.go"), formatted, 0644); err != nil {
		t.Fatalf("failed to write container: %v", err)
	}
	if err := prepareMainGo(testDir, tmpDir); err != nil {
		t.Fatalf("failed to write main.go: %v", err)
	}

	compileCmd := exec.Command("go", "build", "-buildvcs=false", "-o", "gendi_fixture_bin")
	compileCmd.Dir = tmpDir
	if out, err := compileCmd.CombinedOutput(); err != nil {
		t.Fatalf("compilation failed: %v\nOutput:\n%s\n\nGenerated container:\n%s", err, out, formatted)
	}

	runCmd := exec.Command(filepath.Join(tmpDir, "gendi_fixture_bin"))
	runCmd.Dir = tmpDir
	var stdout, stderr bytes.Buffer
	runCmd.Stdout, runCmd.Stderr = &stdout, &stderr
	if err := runCmd.Run(); err != nil {
		t.Fatalf("runtime failed: %v\nStdout:\n%s\nStderr:\n%s", err, stdout.String(), stderr.String())
	}

	if got, want := stdout.String(), "worker\n"; got != want {
		t.Errorf("output mismatch:\nExpected: %q\nActual:   %q", want, got)
	}
}
