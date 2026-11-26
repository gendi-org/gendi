package schematest

import (
	"bytes"
	"os"
	"testing"

	jsonschema "github.com/santhosh-tekuri/jsonschema/v6"
)

func compileConfigSchema(t *testing.T) *jsonschema.Schema {
	t.Helper()
	raw, err := os.ReadFile("../gendi.schema.json")
	if err != nil {
		t.Fatalf("read schema: %v", err)
	}
	doc, err := jsonschema.UnmarshalJSON(bytes.NewReader(raw))
	if err != nil {
		t.Fatalf("parse schema: %v", err)
	}
	compiler := jsonschema.NewCompiler()
	if err := compiler.AddResource("gendi.schema.json", doc); err != nil {
		t.Fatalf("add schema resource: %v", err)
	}
	schema, err := compiler.Compile("gendi.schema.json")
	if err != nil {
		t.Fatalf("compile schema: %v", err)
	}
	return schema
}

func TestConfigSchemaParameters(t *testing.T) {
	schema := compileConfigSchema(t)

	withParam := func(value any) map[string]any {
		return map[string]any{
			"parameters": map[string]any{"p": value},
		}
	}

	valid := []struct {
		name  string
		value any
	}{
		{"string", "localhost"},
		{"int", 8080.0}, // JSON numbers decode as float64
		{"float", 1.5},
		{"bool", true},
	}
	for _, tt := range valid {
		t.Run("valid/"+tt.name, func(t *testing.T) {
			if err := schema.Validate(withParam(tt.value)); err != nil {
				t.Fatalf("expected valid, got %v", err)
			}
		})
	}

	invalid := []struct {
		name  string
		value any
	}{
		{"null", nil},
		{"array", []any{1, 2}},
		// The removed {type, value} form: the loader rejects every mapping
		// value, so the schema must reject it too instead of validating a
		// config the generator refuses.
		{"removed typed form", map[string]any{"type": "int", "value": 8080.0}},
		{"removed typed form without type", map[string]any{"value": "x"}},
		{"any other mapping", map[string]any{"typo": "x"}},
	}
	for _, tt := range invalid {
		t.Run("invalid/"+tt.name, func(t *testing.T) {
			if err := schema.Validate(withParam(tt.value)); err == nil {
				t.Fatalf("expected schema violation for %v", tt.value)
			}
		})
	}
}

func TestConfigSchemaAlias(t *testing.T) {
	schema := compileConfigSchema(t)

	tests := []struct {
		name  string
		alias map[string]any
		valid bool
	}{
		{"plain alias", map[string]any{"alias": "@target"}, true},
		{"public alias", map[string]any{"alias": "@target", "public": true}, true},
		{"shared true", map[string]any{"alias": "@target", "shared": true}, false},
		{"shared false", map[string]any{"alias": "@target", "shared": false}, false},
		{"autoconfigure true", map[string]any{"alias": "@target", "autoconfigure": true}, false},
		{"autoconfigure false", map[string]any{"alias": "@target", "autoconfigure": false}, false},
		{"tags", map[string]any{"alias": "@target", "tags": []any{"handler"}}, false},
		{"empty tags", map[string]any{"alias": "@target", "tags": []any{}}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := map[string]any{
				"services": map[string]any{
					"target": map[string]any{
						"constructor": map[string]any{"func": "example.com/app.New"},
					},
					"alias": tt.alias,
				},
			}
			err := schema.Validate(cfg)
			if tt.valid && err != nil {
				t.Fatalf("expected valid, got %v", err)
			}
			if !tt.valid && err == nil {
				t.Fatalf("expected schema violation for %v", tt.alias)
			}
		})
	}
}

func TestConfigSchemaTagKeys(t *testing.T) {
	schema := compileConfigSchema(t)

	withTagEntry := func(entry map[string]any, sortBy string) map[string]any {
		tag := map[string]any{}
		if sortBy != "" {
			tag["sort_by"] = sortBy
		}
		return map[string]any{
			"tags": map[string]any{"handler": tag},
			"services": map[string]any{
				"users": map[string]any{
					"constructor": map[string]any{"func": "example.com/app.New"},
					"tags":        []any{entry},
				},
			},
		}
	}

	tests := []struct {
		name  string
		cfg   map[string]any
		valid bool
	}{
		{"id string", withTagEntry(map[string]any{"name": "handler", "id": "/users"}, ""), true},
		{"no id", withTagEntry(map[string]any{"name": "handler", "priority": 1.0}, ""), true},
		{"id empty", withTagEntry(map[string]any{"name": "handler", "id": ""}, ""), false},
		{"id number", withTagEntry(map[string]any{"name": "handler", "id": 42.0}, ""), false},
		{"sort_by other attribute", withTagEntry(map[string]any{"name": "handler"}, "priority"), true},
		{"sort_by id", withTagEntry(map[string]any{"name": "handler"}, "id"), false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := schema.Validate(tt.cfg)
			if tt.valid && err != nil {
				t.Fatalf("expected valid, got %v", err)
			}
			if !tt.valid && err == nil {
				t.Fatal("expected schema violation")
			}
		})
	}
}
