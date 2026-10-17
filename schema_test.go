package di

import (
	"bytes"
	"os"
	"testing"

	jsonschema "github.com/santhosh-tekuri/jsonschema/v6"
)

func compileConfigSchema(t *testing.T) *jsonschema.Schema {
	t.Helper()
	raw, err := os.ReadFile("gendi.schema.json")
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

func TestConfigSchemaMapArguments(t *testing.T) {
	schema := compileConfigSchema(t)

	withArg := func(arg any) map[string]any {
		return map[string]any{
			"services": map[string]any{
				"router": map[string]any{
					"constructor": map[string]any{
						"func": "app.NewRouter",
						"args": []any{arg},
					},
				},
			},
		}
	}

	valid := []struct {
		name string
		arg  any
	}{
		{"scalar values", map[string]any{"/": "@handler.home", "n": 1.0, "ok": true}},
		{"empty mapping", map[string]any{}},
		{"plain string argument still valid", "@handler.home"},
		{"nil value", map[string]any{"/": nil}},
		{"int value", map[string]any{"/": 5}},
	}
	for _, tt := range valid {
		t.Run("valid/"+tt.name, func(t *testing.T) {
			if err := schema.Validate(withArg(tt.arg)); err != nil {
				t.Fatalf("expected valid, got %v", err)
			}
		})
	}

	invalid := []struct {
		name string
		arg  any
	}{
		{"nested mapping", map[string]any{"a": map[string]any{"b": 1.0}}},
		{"nested sequence", map[string]any{"a": []any{1.0, 2.0}}},
	}
	for _, tt := range invalid {
		t.Run("invalid/"+tt.name, func(t *testing.T) {
			if err := schema.Validate(withArg(tt.arg)); err == nil {
				t.Fatal("expected the schema to reject a nested collection")
			}
		})
	}
}
