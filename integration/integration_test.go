package integration

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"golang.org/x/tools/imports"

	"github.com/gendi-org/gendi/pipeline"
	"github.com/gendi-org/gendi/yaml"
)

func runEmbeddedTest(t *testing.T, testName string, expectedOutput string, wantCompileErr, wantRuntimeErr bool) {
	testDir := fmt.Sprintf("testdata/%s", testName)
	// Create a temporary directory for test
	tmpDir := prepareTestDir(t, testDir)
	configPath := filepath.Join(tmpDir, "gendi.yaml")

	// Load and parse config
	cfg, err := yaml.LoadConfig(configPath, tmpDir, tmpDir)
	if err != nil {
		t.Fatalf("failed to load config: %v", err)
	}

	// Emit container code
	opts := pipeline.Options{
		Out:        tmpDir,
		Package:    "main",
		ModulePath: "test",
		ModuleRoot: tmpDir,
	}
	if err := opts.Finalize(); err != nil {
		t.Fatalf("failed to finalize options: %v", err)
	}

	code, err := pipeline.Emit(cfg, opts)
	if err != nil {
		if wantCompileErr {
			return // Expected error
		}
		t.Fatalf("failed to generate code: %v", err)
	}

	// Format generated code and fix imports
	formatted, err := imports.Process("container_gen.go", code, nil)
	if err != nil {
		t.Fatalf("failed to format generated code: %v", err)
	}

	// Write generated container
	containerPath := filepath.Join(tmpDir, "container_gen.go")
	if err := os.WriteFile(containerPath, formatted, 0644); err != nil {
		t.Fatalf("failed to write container: %v", err)
	}

	// Write main.go AFTER container generation (so NewContainer is defined)
	if err := prepareMainGo(testDir, tmpDir); err != nil {
		t.Fatalf("failed to write main.go: %v", err)
	}

	// Compile the code
	compileCmd := exec.Command("go", "build", "-buildvcs=false", "-o", "gendi_fixture_bin")
	compileCmd.Dir = tmpDir
	compileOutput, err := compileCmd.CombinedOutput()
	if err != nil {
		if wantCompileErr {
			return // Expected compilation error
		}
		t.Fatalf("compilation failed: %v\nOutput:\n%s\n\nGenerated container:\n%s",
			err, compileOutput, string(formatted))
	}

	if wantCompileErr {
		t.Fatal("expected compilation error, but compilation succeeded")
	}

	// Run the binary
	runCmd := exec.Command(filepath.Join(tmpDir, "gendi_fixture_bin"))
	runCmd.Dir = tmpDir
	var stdout, stderr bytes.Buffer
	runCmd.Stdout = &stdout
	runCmd.Stderr = &stderr

	err = runCmd.Run()
	if err != nil {
		if wantRuntimeErr {
			return // Expected runtime error
		}
		t.Fatalf("runtime failed: %v\nStdout:\n%s\nStderr:\n%s",
			err, stdout.String(), stderr.String())
	}

	if wantRuntimeErr {
		t.Fatal("expected runtime error, but execution succeeded")
	}

	// Verify output
	actualOutput := stdout.String()
	if actualOutput != expectedOutput {
		t.Errorf("output mismatch:\nExpected:\n%q\nActual:\n%q",
			expectedOutput, actualOutput)
	}
}

// TestWorkflow tests the complete workflow using test files from testdata
func TestWorkflow(t *testing.T) {
	tests := []struct {
		name           string
		expectedOutput string
		wantCompileErr bool
		wantRuntimeErr bool
	}{
		// Basic tests
		{
			name:           "basic_service",
			expectedOutput: "Hello, World!\n",
		},
		{
			name:           "service_dependency",
			expectedOutput: "[LOG] Service running\n",
		},
		{
			name:           "shared_value_service",
			expectedOutput: "id1=1 id2=1 builds=1\n",
		},
		{
			name:           "tagged_injection",
			expectedOutput: "B\nA\n",
		},
		{
			name:           "decorator",
			expectedOutput: "decorated(base)\n",
		},
		{
			name:           "method_constructor",
			expectedOutput: "test-product\n",
		},
		{
			name:           "multi_file_imports",
			expectedOutput: "TestApp: 2 handlers\napi\nhome\n",
		},
		{
			name:           "import_with_exclusions",
			expectedOutput: "service banner is: prod\n",
		},
		{
			name:           "parameter_overrides",
			expectedOutput: "Hello from production\n",
		},
		{
			name:           "parameter_contextual",
			expectedOutput: "42 42\n",
		},
		{
			name:           "decorator_chain",
			expectedOutput: "cache(metrics(log(base)))\n",
		},
		{
			name:           "spread_variadic",
			expectedOutput: "B\nA\n",
		},
		{
			name: "map_argument",
			expectedOutput: "/ -> home\n" +
				"/api -> api\n" +
				"/fb -> fallback\n" +
				"/index -> home\n" +
				"eu = europe\n" +
				"us = america\n" +
				"read: 30s\n" +
				"write: 1.5s\n",
		},
		{
			name:           "complex_tagged_injection",
			expectedOutput: "Middleware chain (3):\n- auth\n- metrics\n- logging\nPublic getter returned 3 items\n",
		},
		{
			name:           "generic_channel",
			expectedOutput: "start\nprocess\nend\nProcessed 3 events\n",
		},
		{
			name:           "go_ref",
			expectedOutput: "hello from go ref\n",
		},
		{
			name:           "field_access",
			expectedOutput: "host=localhost dsn=postgres://localhost/mydb\n",
		},
		{
			name:           "field_access_go_ref",
			expectedOutput: "host=localhost dsn=postgres://localhost/defaults\n",
		},
		{
			name: "stdlib_services",
			expectedOutput: "http client timeout: 30s\n" +
				"pooled client timeout: 30s\n" +
				"pooled transport set: true\n" +
				"logger ready: true\n" +
				"json handler ready: true\n",
		},
		{
			name:           "docs_tour",
			expectedOutput: docsTourOutput,
		},
		{
			name:           "ident_collision",
			wantCompileErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			runEmbeddedTest(t, tt.name, tt.expectedOutput, tt.wantCompileErr, tt.wantRuntimeErr)
		})
	}
}
