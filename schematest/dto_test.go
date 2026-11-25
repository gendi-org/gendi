package schematest

import (
	"encoding/json"
	"os"
	"reflect"
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/gendi-org/gendi/yaml"
)

// yamlKeys returns the YAML keys of the fields of a struct type.
func yamlKeys(typ reflect.Type) []string {
	var keys []string
	for field := range typ.Fields() {
		name, _, _ := strings.Cut(field.Tag.Get("yaml"), ",")
		if name == "" || name == "-" {
			continue
		}
		keys = append(keys, name)
	}
	slices.Sort(keys)
	return keys
}

// schemaObject returns the schema object at a JSON pointer.
func schemaObject(t *testing.T, root any, pointer string) map[string]any {
	t.Helper()
	node := root
	for _, token := range strings.Split(strings.TrimPrefix(pointer, "/"), "/") {
		if token == "" {
			continue
		}
		switch n := node.(type) {
		case map[string]any:
			node = n[token]
		case []any:
			i, err := strconv.Atoi(token)
			if err != nil || i < 0 || i >= len(n) {
				t.Fatalf("%s: index %q is not valid", pointer, token)
			}
			node = n[i]
		default:
			t.Fatalf("%s: cannot go into %T at %q", pointer, node, token)
		}
	}
	obj, ok := node.(map[string]any)
	if !ok {
		t.Fatalf("%s: got %T, want an object", pointer, node)
	}
	return obj
}

// TestConfigSchemaPropertiesMatchDTO checks that each object of the schema
// has the same keys as the struct that the loader decodes it into. Each of
// these objects must also reject unknown keys, as the loader does.
func TestConfigSchemaPropertiesMatchDTO(t *testing.T) {
	raw, err := os.ReadFile("../gendi.schema.json")
	if err != nil {
		t.Fatalf("read schema: %v", err)
	}
	var root any
	if err := json.Unmarshal(raw, &root); err != nil {
		t.Fatalf("parse schema: %v", err)
	}

	tests := []struct {
		pointer string
		dto     reflect.Type
	}{
		{"", reflect.TypeFor[yaml.RawConfig]()},
		{"/properties/imports/items/oneOf/1", reflect.TypeFor[yaml.RawImport]()},
		{"/properties/tags/additionalProperties", reflect.TypeFor[yaml.RawTag]()},
		{"/properties/services/properties/_default", reflect.TypeFor[yaml.ServiceDefaults]()},
		{"/properties/services/additionalProperties/oneOf/1", reflect.TypeFor[yaml.RawService]()},
		{"/properties/services/additionalProperties/oneOf/1/properties/constructor", reflect.TypeFor[yaml.RawConstructor]()},
	}
	for _, tt := range tests {
		t.Run(tt.dto.Name(), func(t *testing.T) {
			obj := schemaObject(t, root, tt.pointer)

			props, ok := obj["properties"].(map[string]any)
			if !ok {
				t.Fatalf("%q has no properties", tt.pointer)
			}
			var got []string
			for key := range props {
				got = append(got, key)
			}
			slices.Sort(got)

			if want := yamlKeys(tt.dto); !slices.Equal(got, want) {
				t.Errorf("schema properties at %q = %v, %s YAML keys = %v", tt.pointer, got, tt.dto.Name(), want)
			}
			if obj["additionalProperties"] != false {
				t.Errorf("%q: additionalProperties = %v, want false", tt.pointer, obj["additionalProperties"])
			}
		})
	}
}
