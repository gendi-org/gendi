package schematest

import (
	"bytes"
	"io/fs"
	"os"
	"path/filepath"
	"testing"

	goyaml "github.com/goccy/go-yaml"
	jsonschema "github.com/santhosh-tekuri/jsonschema/v6"

	"github.com/gendi-org/gendi/yaml"
)

// repoRoot is the root of the gendi module, relative to this module.
const repoRoot = ".."

// findConfigFiles returns every YAML file of the gendi module. The site
// module and this module hold no configs, and the walk skips them.
func findConfigFiles(t *testing.T) []string {
	t.Helper()
	var files []string
	err := filepath.WalkDir(repoRoot, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			switch path {
			case filepath.Join(repoRoot, ".git"), filepath.Join(repoRoot, "site"), filepath.Join(repoRoot, "schematest"):
				return filepath.SkipDir
			}
			return nil
		}
		if filepath.Ext(path) == ".yaml" {
			files = append(files, path)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("find config files: %v", err)
	}
	return files
}

// readAsJSON reads a YAML file and decodes it in the form that the
// validator expects.
func readAsJSON(t *testing.T, path string) any {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	raw, err := goyaml.YAMLToJSON(data)
	if err != nil {
		t.Fatalf("convert to JSON: %v", err)
	}
	doc, err := jsonschema.UnmarshalJSON(bytes.NewReader(raw))
	if err != nil {
		t.Fatalf("decode JSON: %v", err)
	}
	return doc
}

// TestConfigSchemaAcceptsLoadedConfigs checks that the schema accepts each
// config file that the loader accepts, except the files of rejectedBySchema.
// The opposite direction is not checked: the schema does not express all the
// semantic rules of the generator.
func TestConfigSchemaAcceptsLoadedConfigs(t *testing.T) {
	// rejectedBySchema lists the config files that the loader accepts, but the
	// generator rejects in a later phase. The schema expresses the same rule, so
	// it must reject these files.
	rejectedBySchema := map[string]string{
		"pipeline/testdata/errors/alias_with_constructor/gendi.yaml": "a service has both alias and constructor",
		"pipeline/testdata/errors/both_func_and_method/gendi.yaml":   "a constructor has both func and method",
		"pipeline/testdata/errors/missing_constructor/gendi.yaml":    "a service has neither constructor nor alias",
	}

	schema := compileConfigSchema(t)
	files := findConfigFiles(t)

	validated := 0
	seenRejected := make(map[string]bool)
	for _, path := range files {
		rel, err := filepath.Rel(repoRoot, path)
		if err != nil {
			t.Fatalf("relative path of %s: %v", path, err)
		}
		name := filepath.ToSlash(rel)
		t.Run(name, func(t *testing.T) {
			boundary, err := yaml.DefaultBoundary(path)
			if err != nil {
				t.Fatalf("boundary: %v", err)
			}
			if _, err := yaml.LoadConfig(path, boundary, ""); err != nil {
				t.Skipf("the loader rejects the file: %v", err)
			}
			validated++

			err = schema.Validate(readAsJSON(t, path))
			if reason, ok := rejectedBySchema[name]; ok {
				seenRejected[name] = true
				if err == nil {
					t.Errorf("the schema accepts the file, but %s", reason)
				}
				return
			}
			if err != nil {
				t.Errorf("the loader accepts the file, but the schema rejects it: %#v", err)
			}
		})
	}

	for name := range rejectedBySchema {
		if !seenRejected[name] {
			t.Errorf("rejectedBySchema has %s, but the loader does not accept it or the file does not exist", name)
		}
	}
	// The loader also rejects a file when the environment is wrong. Without
	// this guard, a broken environment skips every file and the test passes.
	if validated*2 < len(files) {
		t.Fatalf("the loader accepted %d of %d config files; want at least half", validated, len(files))
	}
}
