package testmodule

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestWriteBuildsGendiPackages(t *testing.T) {
	tests := []struct {
		name   string
		source string
	}{
		{
			// The packages that a generated container imports.
			name: "container imports",
			source: `package main

import (
	_ "github.com/gendi-org/gendi/parameters"
	_ "github.com/gendi-org/gendi/stdlib"
)

func main() {}
`,
		},
		{
			// pipeline imports goccy/go-yaml, doublestar and x/tools. Without
			// the requirements and go.sum of gendi, the build fails with
			// "missing go.sum entry".
			name: "external dependencies",
			source: `package main

import _ "github.com/gendi-org/gendi/pipeline"

func main() {}
`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			dir := t.TempDir()
			if err := Write(dir); err != nil {
				t.Fatalf("Write: %v", err)
			}
			if err := os.WriteFile(filepath.Join(dir, "main.go"), []byte(tt.source), 0o644); err != nil {
				t.Fatal(err)
			}

			cmd := exec.Command("go", "build", "-o", os.DevNull, ".")
			cmd.Dir = dir
			if out, err := cmd.CombinedOutput(); err != nil {
				t.Fatalf("go build: %v\n%s", err, out)
			}
		})
	}
}
