package integration

import (
	"bytes"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// TestCLI builds the gendi binary and runs it the way a user does: from the
// module directory, with relative paths. It covers what the tests of the cmd
// package cannot reach: flag parsing, the built-in selectable passes, the exit
// status and the error output on stderr.
func TestCLI(t *testing.T) {
	bin := filepath.Join(t.TempDir(), "gendi_cli_bin")
	build := exec.Command("go", "build", "-buildvcs=false", "-o", bin, "github.com/gendi-org/gendi/cmd/gendi")
	build.Dir = getModuleRoot()
	if out, err := build.CombinedOutput(); err != nil {
		t.Fatalf("build gendi: %v\n%s", err, out)
	}

	const (
		publicConfig = `services:
  replacer:
    constructor:
      func: strings.NewReplacer
    public: true
`
		privateConfig = `services:
  replacer:
    constructor:
      func: strings.NewReplacer
`
		oldContainer = "// old container\n"
		getter       = "func (c *Container) GetReplacer() (*strings.Replacer, error) {"
	)

	cases := []struct {
		name     string
		config   string
		args     []string
		wantExit int
		// wantStderr is the exact stderr, with the module directory
		// replaced by <root>. If both wantStderr and wantStderrContains are
		// empty, stderr must be empty.
		wantStderr string
		// wantStderrContains is a substring of stderr, for output that the
		// test does not own, such as the usage text of the flag package.
		wantStderrContains string
		// wantFile is a substring of the output file after the run.
		wantFile string
	}{
		{
			name:       "generates the container",
			config:     publicConfig,
			args:       []string{"--config=gendi.yaml", "--out=internal/di", "--pkg=di", "--verbose"},
			wantStderr: "generated internal/di/container_gen.go\n",
			wantFile:   getter,
		},
		{
			name:     "selectable pass makes the service public",
			config:   privateConfig,
			args:     []string{"--config=gendi.yaml", "--out=internal/di", "--pkg=di", "--enable-pass=expose-all"},
			wantFile: getter,
		},
		{
			name: "config error prints the snippet",
			config: `services:
  replacer:
    constructor:
      func: strings.NewReplacer
    bogus: 1
`,
			args:     []string{"--config=gendi.yaml", "--out=internal/di", "--pkg=di"},
			wantExit: 1,
			wantStderr: `<root>/gendi.yaml:5:5: load config: unknown field "bogus"
1 | services:
2 |   replacer:
3 |     constructor:
4 |       func: strings.NewReplacer
5 |     bogus: 1
        ^
`,
			wantFile: oldContainer,
		},
		{
			name:       "error without a location ends with a newline",
			config:     publicConfig,
			args:       []string{"--out=internal/di", "--pkg=di"},
			wantExit:   1,
			wantStderr: "config finalize: config is required\n",
			wantFile:   oldContainer,
		},
		{
			name:       "unknown selectable pass",
			config:     publicConfig,
			args:       []string{"--config=gendi.yaml", "--out=internal/di", "--pkg=di", "--enable-pass=nope"},
			wantExit:   1,
			wantStderr: "resolve passes: --enable-pass: unknown pass \"nope\"\n",
			wantFile:   oldContainer,
		},
		{
			// flag.CommandLine exits with status 2 on a parse error.
			name:               "unknown flag",
			config:             publicConfig,
			args:               []string{"--bogus"},
			wantExit:           2,
			wantStderrContains: "flag provided but not defined: -bogus",
			wantFile:           oldContainer,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			root, err := filepath.EvalSymlinks(t.TempDir())
			if err != nil {
				t.Fatal(err)
			}
			if err := createGoMod(filepath.Join(root, "go.mod")); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(root, "gendi.yaml"), []byte(tc.config), 0o644); err != nil {
				t.Fatal(err)
			}
			out := filepath.Join(root, "internal", "di", "container_gen.go")
			if err := os.MkdirAll(filepath.Dir(out), 0o755); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(out, []byte(oldContainer), 0o644); err != nil {
				t.Fatal(err)
			}

			run := exec.Command(bin, tc.args...)
			run.Dir = root
			var stdout, stderr bytes.Buffer
			run.Stdout, run.Stderr = &stdout, &stderr
			err = run.Run()

			exitCode := 0
			if exitErr, ok := errors.AsType[*exec.ExitError](err); ok {
				exitCode = exitErr.ExitCode()
			} else if err != nil {
				t.Fatalf("run gendi: %v", err)
			}
			if exitCode != tc.wantExit {
				t.Errorf("exit status = %d, want %d\nstderr:\n%s", exitCode, tc.wantExit, stderr.String())
			}

			if stdout.Len() != 0 {
				t.Errorf("stdout = %q, want it empty", stdout.String())
			}
			gotStderr := strings.ReplaceAll(stderr.String(), root, "<root>")
			if tc.wantStderr != "" && gotStderr != tc.wantStderr {
				t.Errorf("stderr mismatch:\ngot:\n%s\nwant:\n%s", gotStderr, tc.wantStderr)
			}
			if tc.wantStderr == "" && tc.wantStderrContains == "" && gotStderr != "" {
				t.Errorf("stderr = %q, want it empty", gotStderr)
			}
			if !strings.Contains(gotStderr, tc.wantStderrContains) {
				t.Errorf("stderr does not contain %q:\n%s", tc.wantStderrContains, gotStderr)
			}

			got, err := os.ReadFile(out)
			if err != nil {
				t.Fatal(err)
			}
			if !strings.Contains(string(got), tc.wantFile) {
				t.Errorf("output file does not contain %q:\n%s", tc.wantFile, got)
			}
		})
	}
}
