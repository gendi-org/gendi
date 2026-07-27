package integration

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"golang.org/x/tools/imports"

	"github.com/gendi-org/gendi/pipeline"
	"github.com/gendi-org/gendi/yaml"
)

const docsReadme = "../README.md"

var (
	yamlBlockRe = regexp.MustCompile("(?s)```yaml\n(.*?)```")
	getterRe    = regexp.MustCompile(`\b(?:Get|Must)[A-Z]\w*`)
)

// TestDocsReadme checks the README quick start against the real loader and the
// real generator: its YAML is generated from as written, and every getter its
// Go snippet calls has to exist in the result.
//
// Nothing here is a copy of the README to keep in step — the README block is
// the input. Only the module path is rewritten, because the integration
// harness builds everything as module "test" while the README shows a
// plausible one.
func TestDocsReadme(t *testing.T) {
	readme, err := os.ReadFile(docsReadme)
	if err != nil {
		t.Fatalf("read %s: %v", docsReadme, err)
	}

	config := quickStartConfig(t, string(readme))
	container := emitDocsReadme(t, config)

	getters := getterRe.FindAllString(strings.Join(goBlocks(string(readme)), "\n"), -1)
	if len(getters) == 0 {
		t.Fatalf("no getter call found in the README Go listings — has the quick "+
			"start been rewritten without updating this test? (%s)", docsReadme)
	}

	for _, getter := range getters {
		if !strings.Contains(container, "func (c *Container) "+getter+"(") {
			t.Errorf("the README calls %s, which the container generated from its "+
				"own quick-start configuration does not have", getter)
		}
	}
}

// quickStartConfig returns the README's service configuration, addressed at the
// module the integration harness creates.
func quickStartConfig(t *testing.T, readme string) string {
	t.Helper()

	for _, m := range yamlBlockRe.FindAllStringSubmatch(readme, -1) {
		block := m[1]
		if strings.Contains(block, "services:") {
			return strings.ReplaceAll(block, "example.com/myapp", "test")
		}
	}

	t.Fatalf("no quick-start configuration found in %s", docsReadme)

	return ""
}

func goBlocks(text string) []string {
	var out []string
	for _, m := range goBlockRe.FindAllStringSubmatch(text, -1) {
		out = append(out, m[1])
	}

	return out
}

// emitDocsReadme generates a container from config against the docs_tour
// fixture's packages and returns the formatted source.
func emitDocsReadme(t *testing.T, config string) string {
	t.Helper()

	tmpDir := prepareTestDir(t, "testdata/docs_tour")

	path := filepath.Join(tmpDir, "readme.yaml")
	if err := os.WriteFile(path, []byte(config), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	cfg, err := yaml.LoadConfig(path, tmpDir, tmpDir)
	if err != nil {
		t.Fatalf("the README quick-start configuration does not load: %v", err)
	}

	opts := pipeline.Options{
		Out:        tmpDir,
		Package:    "main",
		ModulePath: "test",
		ModuleRoot: tmpDir,
	}
	if err := opts.Finalize(); err != nil {
		t.Fatalf("finalize options: %v", err)
	}

	code, err := pipeline.Emit(cfg, opts)
	if err != nil {
		t.Fatalf("the README quick-start configuration does not generate: %v", err)
	}

	formatted, err := imports.Process("container_gen.go", code, nil)
	if err != nil {
		t.Fatalf("format: %v", err)
	}

	return string(formatted)
}
