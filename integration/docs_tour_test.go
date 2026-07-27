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

// docsTourOutput is what the docs_tour fixture prints, and what the Getting
// Started page shows as the program's output.
const docsTourOutput = "Hello, world!\nHola, world!\n"

const docsTourPage = "../site/content/docs/_index.md"

// docsTourSources maps the path in a listing's leading comment on the page to
// the fixture file that listing must equal.
//
// The page's main.go is deliberately absent: it imports the container from a
// separate `di` package, while the fixture keeps the container in package main
// because that is what the integration harness generates. Its behaviour is
// covered instead by TestWorkflow asserting docsTourOutput.
var docsTourSources = map[string]string{
	"greet/greet.go": "testdata/docs_tour/greet/greet.go",
}

var goBlockRe = regexp.MustCompile("(?s)```go\n(.*?)```")

// TestDocsTour keeps the Getting Started walkthrough honest. Every Go listing on
// that page is either a file of the docs_tour fixture or a fragment of a
// container that fixture generates, so a change to the emitted code fails here
// instead of quietly turning the page into fiction.
func TestDocsTour(t *testing.T) {
	page, err := os.ReadFile(docsTourPage)
	if err != nil {
		t.Fatalf("read %s: %v", docsTourPage, err)
	}

	if !strings.Contains(string(page), docsTourOutput) {
		t.Errorf("%s does not show the output the fixture prints: %q",
			docsTourPage, docsTourOutput)
	}

	// Both configurations the page walks through, so listings from either step
	// can be checked.
	containers := []string{
		emitDocsTour(t, "first-service.yaml"),
		emitDocsTour(t, "gendi.yaml"),
	}

	var sources, generated, skipped int

	for _, match := range goBlockRe.FindAllStringSubmatch(string(page), -1) {
		block := strings.Trim(match[1], "\n")

		if path, ok := docsTourSourcePath(block); ok {
			want, err := os.ReadFile(docsTourSources[path])
			if err != nil {
				t.Fatalf("read fixture %s: %v", docsTourSources[path], err)
			}

			if block != strings.Trim(string(want), "\n") {
				t.Errorf("listing for %s on %s differs from the fixture file %s",
					path, docsTourPage, docsTourSources[path])
			}

			sources++

			continue
		}

		if !isGeneratedListing(block) {
			skipped++

			continue
		}

		if !containsAny(containers, block) {
			t.Errorf("this listing on %s is not in any container the docs_tour "+
				"fixture generates:\n%s", docsTourPage, block)
		}

		generated++
	}

	t.Logf("checked %d source listing(s) and %d generated listing(s), skipped %d",
		sources, generated, skipped)

	// Guard the guard: if the page loses its listings, this test must not
	// silently start passing on nothing.
	if sources < 1 || generated < 3 {
		t.Errorf("too few listings checked (%d source, %d generated) — has the "+
			"walkthrough been rewritten without updating this test?",
			sources, generated)
	}
}

// docsTourSourcePath reports the fixture-relative path a listing claims to be,
// taken from its leading `// path` comment.
func docsTourSourcePath(block string) (string, bool) {
	first, _, _ := strings.Cut(block, "\n")

	path := strings.TrimSpace(strings.TrimPrefix(first, "//"))
	if _, ok := docsTourSources[path]; !ok {
		return "", false
	}

	return path, true
}

// isGeneratedListing reports whether a listing claims to be generated code.
func isGeneratedListing(block string) bool {
	return strings.Contains(block, "func (c *Container)") ||
		strings.Contains(block, "var DefaultContainerParameters")
}

func containsAny(haystacks []string, needle string) bool {
	for _, h := range haystacks {
		if strings.Contains(h, needle) {
			return true
		}
	}

	return false
}

// emitDocsTour generates the container for one of the fixture's configurations
// and returns the formatted source.
func emitDocsTour(t *testing.T, configName string) string {
	t.Helper()

	tmpDir := prepareTestDir(t, "testdata/docs_tour")

	cfg, err := yaml.LoadConfig(filepath.Join(tmpDir, configName), tmpDir, tmpDir)
	if err != nil {
		t.Fatalf("load %s: %v", configName, err)
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
		t.Fatalf("emit %s: %v", configName, err)
	}

	formatted, err := imports.Process("container_gen.go", code, nil)
	if err != nil {
		t.Fatalf("format %s: %v", configName, err)
	}

	return string(formatted)
}
