package cmd

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"golang.org/x/mod/modfile"
)

func TestWriteTargetFile(t *testing.T) {
	const data = "package di\n"

	cases := []struct {
		name string
		// setup prepares root and returns the path to write.
		setup   func(t *testing.T, root string) string
		wantErr string
	}{
		{
			name: "creates missing directories",
			setup: func(t *testing.T, root string) string {
				return filepath.Join(root, "internal", "di", "container_gen.go")
			},
		},
		{
			name: "replaces a longer existing file",
			setup: func(t *testing.T, root string) string {
				path := filepath.Join(root, "container_gen.go")
				writeFile(t, path, strings.Repeat("old content\n", 100))
				return path
			},
		},
		{
			name: "parent path is a regular file",
			setup: func(t *testing.T, root string) string {
				writeFile(t, filepath.Join(root, "di"), "not a directory")
				return filepath.Join(root, "di", "container_gen.go")
			},
			wantErr: "create output dir",
		},
		{
			name: "target is a non-empty directory",
			setup: func(t *testing.T, root string) string {
				path := filepath.Join(root, "container_gen.go")
				writeFile(t, filepath.Join(path, "keep.go"), "package keep\n")
				return path
			},
			wantErr: "replace target file",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			root := t.TempDir()
			path := tc.setup(t, root)

			err := WriteTargetFile(path, []byte(data))
			assertNoTempFiles(t, root)

			if tc.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
					t.Fatalf("WriteTargetFile() error = %v, want it to contain %q", err, tc.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("WriteTargetFile() error = %v", err)
			}

			got, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			if string(got) != data {
				t.Errorf("file content = %q, want %q", got, data)
			}

			info, err := os.Stat(path)
			if err != nil {
				t.Fatal(err)
			}
			if runtime.GOOS != "windows" && info.Mode().Perm() != 0o644 {
				t.Errorf("file mode = %v, want %v", info.Mode().Perm(), os.FileMode(0o644))
			}
		})
	}
}

func TestGenerate(t *testing.T) {
	const oldContainer = "// old container\n"
	goVersion := moduleGoVersion(t)

	cases := []struct {
		name       string
		config     string
		configPath string // relative to the module root; "" means gendi.yaml
		wantErr    string
		// wantFile is a substring of the output file after the call. On an
		// error, the old container must stay.
		wantFile string
	}{
		{
			name: "writes the container",
			config: `services:
  replacer:
    constructor:
      func: strings.NewReplacer
    public: true
`,
			wantFile: "func (c *Container) GetReplacer() (*strings.Replacer, error) {",
		},
		{
			name: "config error keeps the old file",
			config: `services:
  replacer:
    constructor:
      func: strings.NewReplacer
    bogus: 1
`,
			wantErr:  `unknown field "bogus"`,
			wantFile: oldContainer,
		},
		{
			name: "generation error keeps the old file",
			config: `services:
  replacer:
    constructor:
      func: strings.NewReplacer
`,
			wantErr:  "at least one public service or tag is required",
			wantFile: oldContainer,
		},
		{
			name:       "missing config file keeps the old file",
			configPath: "missing.yaml",
			wantErr:    "missing.yaml",
			wantFile:   oldContainer,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			root := t.TempDir()
			writeFile(t, filepath.Join(root, "go.mod"), "module test\n\ngo "+goVersion+"\n")
			writeFile(t, filepath.Join(root, "gendi.yaml"), tc.config)
			out := filepath.Join(root, "internal", "di", "container_gen.go")
			writeFile(t, out, oldContainer)

			configPath := tc.configPath
			if configPath == "" {
				configPath = "gendi.yaml"
			}
			cfg := Config{ConfigPath: filepath.Join(root, configPath)}
			cfg.Options.Out = out
			cfg.Options.Package = "di"

			err := Generate(cfg, nil)
			if tc.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
					t.Fatalf("Generate() error = %v, want it to contain %q", err, tc.wantErr)
				}
			} else if err != nil {
				t.Fatalf("Generate() error = %v", err)
			}

			got, err := os.ReadFile(out)
			if err != nil {
				t.Fatal(err)
			}
			if !strings.Contains(string(got), tc.wantFile) {
				t.Errorf("output file does not contain %q:\n%s", tc.wantFile, got)
			}
			assertNoTempFiles(t, root)
		})
	}
}

func TestGenerate_RequiresConfig(t *testing.T) {
	var cfg Config
	cfg.Options.Out = filepath.Join(t.TempDir(), "container_gen.go")
	cfg.Options.Package = "di"

	err := Generate(cfg, nil)
	if err == nil || !strings.Contains(err.Error(), "config is required") {
		t.Fatalf("Generate() error = %v, want it to contain %q", err, "config is required")
	}
}

// The fixture module uses the go version of the gendi module, so the test has
// no version literal that becomes old.
func moduleGoVersion(t *testing.T) string {
	t.Helper()
	data, err := os.ReadFile("../go.mod")
	if err != nil {
		t.Fatal(err)
	}
	mod, err := modfile.ParseLax("go.mod", data, nil)
	if err != nil {
		t.Fatal(err)
	}
	return mod.Go.Version
}

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func assertNoTempFiles(t *testing.T, root string) {
	t.Helper()
	err := filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if strings.HasSuffix(d.Name(), ".tmp") {
			t.Errorf("temporary file left behind: %s", path)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}
