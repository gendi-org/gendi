package pipeline_test

import (
	"errors"
	"os"
	"path/filepath"
	"regexp"
	"testing"

	"github.com/gendi-org/gendi/internal/testmodule"
	"github.com/gendi-org/gendi/pipeline"
	"github.com/gendi-org/gendi/yaml"
)

// TestErrorCases tests that invalid configurations are properly rejected during the
// YAML loading and code generation pipeline (not including compilation/runtime).
func TestErrorCases(t *testing.T) {
	tests := []struct {
		name        string
		expectError string
		phase       string // "load" or "generate"
	}{
		{
			name:        "circular_dependency",
			expectError: "circular",
			phase:       "generate",
		},
		{
			name:        "missing_service",
			expectError: "not found",
			phase:       "generate",
		},
		{
			name:        "missing_parameter",
			expectError: "not found",
			phase:       "generate",
		},
		{
			name:        "invalid_parameter_form",
			expectError: "value must be a plain scalar, got a mapping",
			phase:       "load",
		},
		{
			name:        "missing_constructor",
			expectError: "constructor",
			phase:       "generate",
		},
		{
			name:        "both_func_and_method",
			expectError: "both func and method",
			phase:       "generate",
		},
		{
			name:        "decorator_missing_inner",
			expectError: "inner",
			phase:       "generate",
		},
		{
			name:        "spread_not_last",
			expectError: "spread.*last",
			phase:       "generate",
		},
		{
			name:        "autoconfigure_with_sortby",
			expectError: "autoconfigure.*sort_by",
			phase:       "generate",
		},
		{
			name:        "alias_with_autoconfigure",
			expectError: `gendi\.yaml:8:10: .*service "alias": alias cannot define autoconfigure; autoconfigure applies to target "dummy"`,
			phase:       "load",
		},
		{
			name:        "alias_with_empty_tags",
			expectError: `gendi\.yaml:10:5: .*service "alias": alias cannot define tags; tag the target "target" instead`,
			phase:       "load",
		},
		{
			// The tag moved from users to its decorator. The error must
			// point at the tag entry on users.
			name:        "decorated_tag_not_assignable",
			expectError: `gendi\.yaml:11:9: service "users\.logging" with tag "handler": type test\.Service is not assignable to test\.Handler`,
			phase:       "generate",
		},
		{
			name:        "invalid_alias",
			expectError: "alias.*not found",
			phase:       "generate",
		},
		{
			name:        "alias_with_constructor",
			expectError: "alias.*constructor",
			phase:       "generate",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			srcDir := filepath.Join("testdata", "errors", tt.name)
			tmpDir := prepareErrorTestDir(t, srcDir)

			configPath := filepath.Join(tmpDir, "gendi.yaml")
			cfg, err := yaml.LoadConfig(configPath, tmpDir, tmpDir)

			if tt.phase == "load" {
				if err == nil {
					t.Fatal("expected error during load, got none")
				}
				matched, _ := regexp.MatchString("(?i)"+tt.expectError, err.Error())
				if !matched {
					t.Errorf("expected error matching %q, got: %v", tt.expectError, err)
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected load error: %v", err)
			}

			opts := pipeline.Options{
				Package:    "main",
				ModuleRoot: tmpDir,
				Out:        tmpDir,
				ModulePath: "test",
			}
			if err := opts.Finalize(); err != nil {
				t.Fatalf("finalize options: %v", err)
			}

			_, err = pipeline.Emit(cfg, opts)

			if tt.phase == "generate" {
				if err == nil {
					t.Fatal("expected error during generation, got none")
				}
				matched, _ := regexp.MatchString("(?i)"+tt.expectError, err.Error())
				if !matched {
					t.Errorf("expected error matching %q, got: %v", tt.expectError, err)
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected generation error: %v", err)
			}
		})
	}
}

func prepareErrorTestDir(t *testing.T, srcDir string) string {
	t.Helper()

	tmpDir := t.TempDir()
	if err := os.CopyFS(tmpDir, os.DirFS(srcDir)); err != nil {
		t.Fatal(err)
	}
	if err := testmodule.Write(tmpDir); err != nil {
		t.Fatal(err)
	}

	mainGoPath := filepath.Join(tmpDir, "main.go")
	if _, err := os.Stat(mainGoPath); errors.Is(err, os.ErrNotExist) {
		if err := os.WriteFile(mainGoPath, []byte("package main\nfunc main() {}\n"), 0644); err != nil {
			t.Fatal(err)
		}
	}

	return tmpDir
}
