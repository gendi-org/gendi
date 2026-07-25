package yaml

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	yamllib "github.com/goccy/go-yaml"
	"github.com/goccy/go-yaml/ast"
	"github.com/goccy/go-yaml/parser"

	di "github.com/gendi-org/gendi"
	"github.com/gendi-org/gendi/srcloc"
)

// mustParseNode parses a YAML snippet via goccy and returns the
// document root node. Used by tests that previously hand-built
// *yaml.Node literals.
func mustParseNode(t *testing.T, src string) ast.Node {
	t.Helper()
	f, err := parser.ParseBytes([]byte(src), 0)
	if err != nil {
		t.Fatalf("parse helper for %q: %v", src, err)
	}
	if len(f.Docs) == 0 {
		t.Fatalf("no docs in %q", src)
	}
	return f.Docs[0].Body
}

func TestParseServiceAlias(t *testing.T) {
	for _, alias := range []string{"@foo", "foo"} {
		t.Run(alias, func(t *testing.T) {
			svc, err := NewParser().convertServiceWithPackageAndFile(
				&RawService{Alias: alias},
				nil,
				"",
				"",
			)
			if err != nil {
				t.Fatalf("convertServiceWithPackageAndFile failed: %v", err)
			}
			if svc.Alias != "foo" {
				t.Errorf("alias = %q, want %q", svc.Alias, "foo")
			}
			if svc.Shared {
				t.Error("alias must not define shared")
			}
		})
	}
}

func TestParseServiceAliasRejectsExplicitShared(t *testing.T) {
	for _, alias := range []string{"@target", "target"} {
		for _, shared := range []bool{true, false} {
			t.Run(fmt.Sprintf("alias=%s/shared=%t", alias, shared), func(t *testing.T) {
				raw := &RawConfig{
					Services: map[string]*RawService{
						"alias": {Alias: alias, Shared: &shared},
					},
				}

				_, err := NewParser().ConvertConfigWithDirAndFile(raw, "", "gendi.yaml")
				if err == nil {
					t.Fatal("expected explicit alias shared setting to fail")
				}
				if !strings.Contains(err.Error(), `service "alias": alias cannot define shared; lifecycle is inherited from target "target"`) {
					t.Fatalf("unexpected error: %v", err)
				}
			})
		}
	}
}

func TestParseServiceAliasDoesNotInheritSharedDefault(t *testing.T) {
	raw := &RawService{Alias: "@target"}
	svc, err := NewParser().convertServiceWithPackageAndFile(
		raw,
		&ServiceDefaults{Shared: boolPtr(true)},
		"",
		"",
	)
	if err != nil {
		t.Fatalf("convertServiceWithPackageAndFile failed: %v", err)
	}
	if svc.Shared {
		t.Error("alias must not inherit the default shared setting")
	}
}

func TestParseArgument(t *testing.T) {
	tests := []struct {
		name     string
		raw      *RawArgument
		wantKind di.ArgumentKind
		check    func(t *testing.T, arg di.Argument)
	}{
		{
			name:     "service reference",
			raw:      &RawArgument{Value: strPtr("@myService")},
			wantKind: di.ArgServiceRef,
			check: func(t *testing.T, arg di.Argument) {
				if arg.Value != "myService" {
					t.Errorf("expected value 'myService', got '%s'", arg.Value)
				}
			},
		},
		{
			name:     "literal string",
			raw:      &RawArgument{Value: strPtr("just a string")},
			wantKind: di.ArgLiteral,
			check: func(t *testing.T, arg di.Argument) {
				if arg.Literal.String() != "just a string" {
					t.Errorf("expected literal value 'just a string', got '%s'", arg.Literal.String())
				}
			},
		},
		{
			name:     "literal node",
			raw:      &RawArgument{Node: mustParseNode(t, "42")},
			wantKind: di.ArgLiteral,
			check: func(t *testing.T, arg di.Argument) {
				if arg.Literal.Int() != 42 {
					t.Errorf("expected literal 42, got %d", arg.Literal.Int())
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			arg, err := NewParser().convertArgumentWithFile(tt.raw, "")
			if err != nil {
				t.Fatalf("convertArgumentWithFile failed: %v", err)
			}
			if arg.Kind != tt.wantKind {
				t.Errorf("expected kind %v, got %v", tt.wantKind, arg.Kind)
			}
			tt.check(t, arg)
		})
	}
}

func TestServiceDefaults(t *testing.T) {
	tests := []struct {
		name                  string
		defaults              *ServiceDefaults
		service               *RawService
		expectedShared        *bool
		expectedPublic        bool
		expectedAutoconfigure bool
	}{
		{
			name:     "no defaults",
			defaults: nil,
			service: &RawService{
				Type: "string",
			},
			expectedShared:        nil,
			expectedPublic:        false,
			expectedAutoconfigure: true,
		},
		{
			name: "inherit shared from defaults",
			defaults: &ServiceDefaults{
				Shared: boolPtr(true),
			},
			service: &RawService{
				Type: "string",
			},
			expectedShared:        boolPtr(true),
			expectedPublic:        false,
			expectedAutoconfigure: true,
		},
		{
			name: "inherit public from defaults",
			defaults: &ServiceDefaults{
				Public: boolPtr(true),
			},
			service: &RawService{
				Type: "string",
			},
			expectedShared:        nil,
			expectedPublic:        true,
			expectedAutoconfigure: true,
		},
		{
			name: "override shared",
			defaults: &ServiceDefaults{
				Shared: boolPtr(true),
			},
			service: &RawService{
				Type:   "string",
				Shared: boolPtr(false),
			},
			expectedShared:        boolPtr(false),
			expectedPublic:        false,
			expectedAutoconfigure: true,
		},
		{
			name: "override public",
			defaults: &ServiceDefaults{
				Public: boolPtr(true),
			},
			service: &RawService{
				Type:   "string",
				Public: boolPtr(false),
			},
			expectedShared:        nil,
			expectedPublic:        false,
			expectedAutoconfigure: true,
		},
		{
			name: "inherit both from defaults",
			defaults: &ServiceDefaults{
				Shared: boolPtr(true),
				Public: boolPtr(true),
			},
			service: &RawService{
				Type: "string",
			},
			expectedShared:        boolPtr(true),
			expectedPublic:        true,
			expectedAutoconfigure: true,
		},
		{
			name: "inherit autoconfigure from defaults",
			defaults: &ServiceDefaults{
				Autoconfigure: boolPtr(false),
			},
			service: &RawService{
				Type: "string",
			},
			expectedShared:        nil,
			expectedPublic:        false,
			expectedAutoconfigure: false,
		},
		{
			name: "override autoconfigure",
			defaults: &ServiceDefaults{
				Autoconfigure: boolPtr(false),
			},
			service: &RawService{
				Type:          "string",
				Autoconfigure: boolPtr(true),
			},
			expectedShared:        nil,
			expectedPublic:        false,
			expectedAutoconfigure: true,
		},
	}

	p := NewParser()
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc, err := p.convertServiceWithPackageAndFile(tt.service, tt.defaults, "", "")
			if err != nil {
				t.Fatalf("convertServiceWithPackageAndFile failed: %v", err)
			}

			if svc.Shared != resolveBoolPtr(tt.expectedShared) {
				t.Errorf("expected shared=%v, got %v", resolveBoolPtr(tt.expectedShared), svc.Shared)
			}

			if svc.Public != tt.expectedPublic {
				t.Errorf("expected public=%v, got %v", tt.expectedPublic, svc.Public)
			}

			if svc.Autoconfigure != tt.expectedAutoconfigure {
				t.Errorf("expected autoconfigure=%v, got %v", tt.expectedAutoconfigure, svc.Autoconfigure)
			}
		})
	}
}

func TestServiceTagFlattened(t *testing.T) {
	// Test new syntax where attributes are at the same level as name
	raw := &RawService{
		Type: "string",
		Tags: []RawServiceTag{
			{
				Name: "test.tag",
				Attributes: map[string]any{
					"priority": 10,
					"enabled":  true,
				},
			},
		},
	}

	p := NewParser()
	svc, err := p.convertServiceWithPackageAndFile(raw, nil, "", "")
	if err != nil {
		t.Fatalf("convertServiceWithPackageAndFile failed: %v", err)
	}

	if len(svc.Tags) != 1 {
		t.Fatalf("expected 1 tag, got %d", len(svc.Tags))
	}

	tag := svc.Tags[0]
	if tag.Name != "test.tag" {
		t.Errorf("expected tag name 'test.tag', got '%s'", tag.Name)
	}

	if priority, ok := tag.Attributes["priority"].(int); !ok || priority != 10 {
		t.Errorf("expected priority=10, got %v", tag.Attributes["priority"])
	}

	if enabled, ok := tag.Attributes["enabled"].(bool); !ok || !enabled {
		t.Errorf("expected enabled=true, got %v", tag.Attributes["enabled"])
	}
}

func TestServiceTagOnlyName(t *testing.T) {
	// Test tag with only name (no attributes)
	raw := &RawService{
		Type: "string",
		Tags: []RawServiceTag{
			{
				Name:       "marker.tag",
				Attributes: map[string]any{},
			},
		},
	}

	p := NewParser()
	svc, err := p.convertServiceWithPackageAndFile(raw, nil, "", "")
	if err != nil {
		t.Fatalf("convertServiceWithPackageAndFile failed: %v", err)
	}

	if len(svc.Tags) != 1 {
		t.Fatalf("expected 1 tag, got %d", len(svc.Tags))
	}

	tag := svc.Tags[0]
	if tag.Name != "marker.tag" {
		t.Errorf("expected tag name 'marker.tag', got '%s'", tag.Name)
	}

	if len(tag.Attributes) != 0 {
		t.Errorf("expected no attributes, got %v", tag.Attributes)
	}
}

func TestServiceTagParsing(t *testing.T) {
	type wantTag struct {
		name  string
		attrs map[string]any // int values compared via attrEqualsInt, others exactly
	}
	tests := []struct {
		name string
		yaml string
		want []wantTag
	}{
		{
			name: "mapping form with attributes",
			yaml: `
services:
  test.service:
    type: string
    tags:
      - name: handler.http
        priority: 10
        path: /api/test
      - name: marker.tag
`,
			want: []wantTag{
				{name: "handler.http", attrs: map[string]any{"priority": int64(10), "path": "/api/test"}},
				{name: "marker.tag"},
			},
		},
		{
			name: "string shorthand mixed with mapping",
			yaml: `
services:
  test.service:
    type: string
    tags:
      - marker.tag
      - name: handler.http
        priority: 10
`,
			want: []wantTag{
				{name: "marker.tag"},
				{name: "handler.http", attrs: map[string]any{"priority": int64(10)}},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var raw RawConfig
			if err := yamllib.Unmarshal([]byte(tt.yaml), &raw); err != nil {
				t.Fatalf("failed to unmarshal YAML: %v", err)
			}

			svc, ok := raw.Services["test.service"]
			if !ok {
				t.Fatal("service 'test.service' not found")
			}
			if len(svc.Tags) != len(tt.want) {
				t.Fatalf("expected %d tags, got %d", len(tt.want), len(svc.Tags))
			}

			for i, want := range tt.want {
				tag := svc.Tags[i]
				if tag.Name != want.name {
					t.Errorf("tag %d: expected name %q, got %q", i, want.name, tag.Name)
				}
				if len(tag.Attributes) != len(want.attrs) {
					t.Errorf("tag %q: expected %d attributes, got %v", want.name, len(want.attrs), tag.Attributes)
				}
				for k, wantVal := range want.attrs {
					got := tag.Attributes[k]
					switch w := wantVal.(type) {
					case int64:
						if !attrEqualsInt(got, w) {
							t.Errorf("tag %q attr %q: expected %d, got %v (%T)", want.name, k, w, got, got)
						}
					default:
						if got != wantVal {
							t.Errorf("tag %q attr %q: expected %v, got %v", want.name, k, wantVal, got)
						}
					}
				}
			}
		})
	}
}

// attrEqualsInt accepts both int (yaml.v3) and uint64/int64 (goccy)
// representations of decoded YAML integers, so tests survive parser
// library changes.
func attrEqualsInt(got any, want int64) bool {
	switch v := got.(type) {
	case int:
		return int64(v) == want
	case int64:
		return v == want
	case uint64:
		return int64(v) == want
	}
	return false
}

func TestValidateDefaultsRejectsInvalidFields(t *testing.T) {
	tests := []struct {
		name        string
		service     *RawService
		expectError string
	}{
		{
			name: "type not allowed",
			service: &RawService{
				Type: "string",
			},
			expectError: "type",
		},
		{
			name: "constructor not allowed",
			service: &RawService{
				Constructor: RawConstructor{
					Func: "NewFoo",
				},
			},
			expectError: "constructor",
		},
		{
			name: "alias not allowed",
			service: &RawService{
				Alias: "@foo",
			},
			expectError: "alias",
		},
		{
			name: "decorates not allowed",
			service: &RawService{
				Decorates: "base",
			},
			expectError: "decorates",
		},
		{
			name: "decoration_priority not allowed",
			service: &RawService{
				DecorationPriority: 10,
			},
			expectError: "decoration_priority",
		},
		{
			name: "tags not allowed",
			service: &RawService{
				Tags: []RawServiceTag{{Name: "foo"}},
			},
			expectError: "tags",
		},
		{
			name: "only shared allowed",
			service: &RawService{
				Shared: boolPtr(true),
			},
			expectError: "",
		},
		{
			name: "only public allowed",
			service: &RawService{
				Public: boolPtr(true),
			},
			expectError: "",
		},
	}

	p := NewParser()
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := p.validateDefaults(tt.service)
			if tt.expectError == "" {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				}
			} else {
				if err == nil {
					t.Errorf("expected error containing %q, got nil", tt.expectError)
				} else if !strings.Contains(err.Error(), tt.expectError) {
					t.Errorf("expected error containing %q, got: %v", tt.expectError, err)
				}
			}
		})
	}
}

// Helper functions
func boolPtr(b bool) *bool {
	return &b
}

func strPtr(s string) *string {
	return &s
}

func resolveBoolPtr(b *bool) bool {
	if b == nil {
		return true // Default shared is true
	}
	return *b
}

func TestThisSubstitutionInConstructor(t *testing.T) {
	tests := []struct {
		name       string
		raw        *RawService
		thisPkg    string
		wantFunc   string
		wantMethod string
	}{
		{
			name:     "func substituted",
			raw:      &RawService{Type: "string", Constructor: RawConstructor{Func: "$this.NewService"}},
			thisPkg:  "github.com/example/app",
			wantFunc: "github.com/example/app.NewService",
		},
		{
			// A method constructor addresses a service, so $this has no
			// meaning there and the value is passed through untouched.
			name:       "method left untouched",
			raw:        &RawService{Type: "string", Constructor: RawConstructor{Method: "$this.@service.Method"}},
			thisPkg:    "github.com/example/app",
			wantMethod: "$this.@service.Method",
		},
		{
			name:     "no package leaves $this unchanged",
			raw:      &RawService{Type: "string", Constructor: RawConstructor{Func: "$this.NewService"}},
			thisPkg:  "",
			wantFunc: "$this.NewService",
		},
		{
			name:     "$this not at start stays unchanged",
			raw:      &RawService{Type: "string", Constructor: RawConstructor{Func: "github.com/other/$this.NewService"}},
			thisPkg:  "github.com/example/app",
			wantFunc: "github.com/other/$this.NewService",
		},
		{
			name:    "alias has no constructor",
			raw:     &RawService{Alias: "@other"},
			thisPkg: "github.com/example/app",
		},
		{
			name:     "both type and func substituted",
			raw:      &RawService{Type: "$this.Logger", Constructor: RawConstructor{Func: "$this.NewLogger"}},
			thisPkg:  "github.com/example/app",
			wantFunc: "github.com/example/app.NewLogger",
		},
		{
			name:     "func generic type args substituted",
			raw:      &RawService{Type: "string", Constructor: RawConstructor{Func: "$this.NewPool[$this.Message]"}},
			thisPkg:  "github.com/example/app",
			wantFunc: "github.com/example/app.NewPool[github.com/example/app.Message]",
		},
		{
			name:     "func nested generic type args substituted",
			raw:      &RawService{Type: "string", Constructor: RawConstructor{Func: "external.com/pkg.NewMap[string, chan $this.User]"}},
			thisPkg:  "github.com/example/app",
			wantFunc: "external.com/pkg.NewMap[string, chan github.com/example/app.User]",
		},
	}

	p := NewParser()
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc, err := p.convertServiceWithPackageAndFile(tt.raw, nil, tt.thisPkg, "")
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if tt.wantFunc != "" && svc.Constructor.Func != tt.wantFunc {
				t.Errorf("Func = %q, want %q", svc.Constructor.Func, tt.wantFunc)
			}
			if tt.wantMethod != "" && svc.Constructor.Method != tt.wantMethod {
				t.Errorf("Method = %q, want %q", svc.Constructor.Method, tt.wantMethod)
			}
		})
	}
}

func TestThisSubstitutionInType(t *testing.T) {
	tests := []struct {
		name     string
		typ      string
		wantType string
	}{
		{"plain", "$this.Logger", "github.com/example/app.Logger"},
		{"pointer", "*$this.Logger", "*github.com/example/app.Logger"},
		{"slice", "[]$this.Logger", "[]github.com/example/app.Logger"},
		{"map value", "map[string]$this.Logger", "map[string]github.com/example/app.Logger"},
	}

	p := NewParser()
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			raw := &RawService{
				Type:        tt.typ,
				Constructor: RawConstructor{Func: "pkg.New"},
			}
			svc, err := p.convertServiceWithPackageAndFile(raw, nil, "github.com/example/app", "")
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if svc.Type != tt.wantType {
				t.Errorf("Type = %q, want %q", svc.Type, tt.wantType)
			}
		})
	}
}

func TestThisSubstitutionInTagElementType(t *testing.T) {
	tests := []struct {
		name        string
		elementType string
		configDir   string
		want        string
	}{
		{"plain", "$this.Notifier", "WITH_MOD", "github.com/example/app.Notifier"},
		{"pointer", "*$this.Handler", "WITH_MOD", "*github.com/example/app.Handler"},
		{"slice", "[]$this.Middleware", "WITH_MOD", "[]github.com/example/app.Middleware"},
		{"no package", "$this.Notifier", "", "$this.Notifier"},
	}

	p := NewParser()
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			configDir := tt.configDir
			if configDir == "WITH_MOD" {
				tempDir := t.TempDir()
				modFile := filepath.Join(tempDir, "go.mod")
				if err := os.WriteFile(modFile, []byte("module github.com/example/app\n\ngo 1.21\n"), 0o644); err != nil {
					t.Fatalf("failed to write go.mod: %v", err)
				}
				configDir = tempDir
			}

			raw := &RawConfig{
				Tags: map[string]RawTag{
					"test.tag": {ElementType: tt.elementType},
				},
			}
			cfg, err := p.ConvertConfigWithDirAndFile(raw, configDir, "")
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			tag, ok := cfg.Tags["test.tag"]
			if !ok {
				t.Fatal("tag not found")
			}
			if tag.ElementType != tt.want {
				t.Errorf("ElementType = %q, want %q", tag.ElementType, tt.want)
			}
		})
	}
}

func TestConvertLiteralTypes(t *testing.T) {
	p := NewParser()

	tests := []struct {
		name    string
		yaml    string
		wantErr string
	}{
		{name: "float", yaml: "3.14"},
		{name: "bool", yaml: "true"},
		{name: "null", yaml: "null"},
		{
			// A mapping where a scalar is expected — exercises the
			// "unsupported literal type" branch in convertLiteral.
			name:    "unsupported",
			yaml:    "{a: b}",
			wantErr: "unsupported literal type",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			node := mustParseNode(t, tt.yaml)
			_, err := p.convertLiteral(node, "")
			if tt.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("expected error containing %q, got: %v", tt.wantErr, err)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
		})
	}
}

func TestConvertConfigWithDirAndFile(t *testing.T) {
	p := NewParser()

	t.Run("parameter_mapping_value_rejected", func(t *testing.T) {
		raw := &RawConfig{
			Parameters: map[string]RawParameter{
				"port": {Value: mustParseNode(t, "{type: int, value: 8080}")},
			},
		}
		_, err := p.ConvertConfigWithDirAndFile(raw, "", "")
		if err == nil || !strings.Contains(err.Error(), "value must be a plain scalar, got a mapping") {
			t.Fatalf("expected a mapping value to be rejected, got: %v", err)
		}
	})

	t.Run("parameter_ok", func(t *testing.T) {
		raw := &RawConfig{
			Parameters: map[string]RawParameter{
				"host": {Value: mustParseNode(t, "localhost")},
			},
		}
		cfg, err := p.ConvertConfigWithDirAndFile(raw, "", "")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if cfg.Parameters["host"].Value.String() != "localhost" {
			t.Fatal("expected parameter 'host' with value localhost")
		}
	})

	t.Run("default_applied_to_services", func(t *testing.T) {
		raw := &RawConfig{
			Services: map[string]*RawService{
				"_default": {Shared: boolPtr(false)},
				"svc": {
					Constructor: RawConstructor{Func: "pkg.New"},
				},
			},
		}
		cfg, err := p.ConvertConfigWithDirAndFile(raw, "", "")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if cfg.Services["svc"].Shared != false {
			t.Fatal("expected shared=false from _default")
		}
		if _, ok := cfg.Services["_default"]; ok {
			t.Fatal("_default should not appear in output services")
		}
	})

	t.Run("default_invalid", func(t *testing.T) {
		raw := &RawConfig{
			Services: map[string]*RawService{
				"_default": {Type: "bad"},
			},
		}
		_, err := p.ConvertConfigWithDirAndFile(raw, "", "")
		if err == nil || !strings.Contains(err.Error(), "_default") {
			t.Fatalf("expected _default error, got: %v", err)
		}
	})

	t.Run("service_convert_error", func(t *testing.T) {
		// Mapping where a literal is expected — exercises convertLiteral
		// "unsupported literal type" path through the arg conversion.
		badNode := mustParseNode(t, "{a: b}")
		raw := &RawConfig{
			Services: map[string]*RawService{
				"bad": {
					Constructor: RawConstructor{
						Args: []RawArgument{{Node: badNode}},
					},
				},
			},
		}
		_, err := p.ConvertConfigWithDirAndFile(raw, "", "")
		if err == nil {
			t.Fatal("expected service conversion error")
		}
	})
}

func TestConvertArgumentEmpty(t *testing.T) {
	p := NewParser()
	_, err := p.convertArgumentWithFile(&RawArgument{}, "")
	if err == nil || !strings.Contains(err.Error(), "must have a value") {
		t.Fatalf("expected 'must have a value' error, got: %v", err)
	}
}

func TestThisSubstitutionInGoAndFieldArgs(t *testing.T) {
	p := NewParser()

	t.Run("go_ref_this", func(t *testing.T) {
		goVal := "!go:$this.DefaultLevel"
		raw := &RawService{
			Constructor: RawConstructor{
				Func: "pkg.New",
				Args: []RawArgument{{Value: &goVal}},
			},
		}
		svc, err := p.convertServiceWithPackageAndFile(raw, nil, "github.com/app", "")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if svc.Constructor.Args[0].Value != "github.com/app.DefaultLevel" {
			t.Fatalf("expected substituted go ref, got: %s", svc.Constructor.Args[0].Value)
		}
	})

	t.Run("field_go_ref_this", func(t *testing.T) {
		fieldVal := "!field:!go:$this.DefaultCfg.Host"
		raw := &RawService{
			Constructor: RawConstructor{
				Func: "pkg.New",
				Args: []RawArgument{{Value: &fieldVal}},
			},
		}
		svc, err := p.convertServiceWithPackageAndFile(raw, nil, "github.com/app", "")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if svc.Constructor.Args[0].Kind != di.ArgFieldAccessGo {
			t.Fatalf("expected ArgFieldAccessGo, got: %d", svc.Constructor.Args[0].Kind)
		}
		expected := "github.com/app.DefaultCfg.Host"
		if svc.Constructor.Args[0].Value != expected {
			t.Fatalf("expected %q, got: %q", expected, svc.Constructor.Args[0].Value)
		}
	})
}

func TestTagAutoconfigureParsed(t *testing.T) {
	raw := &RawConfig{
		Tags: map[string]RawTag{
			"auto.tag": {
				ElementType:   "string",
				Autoconfigure: true,
			},
		},
	}
	p := NewParser()
	cfg, err := p.ConvertConfigWithDirAndFile(raw, "", "")
	if err != nil {
		t.Fatalf("convertConfigWithDir failed: %v", err)
	}

	tag, ok := cfg.Tags["auto.tag"]
	if !ok {
		t.Fatal("tag 'auto.tag' not found")
	}

	if !tag.Autoconfigure {
		t.Fatal("expected tag 'auto.tag' to have autoconfigure enabled")
	}
}

// TestConvertLiteral_LocatedErrors verifies that every rejection branch
// in convertLiteral produces a *srcloc.Error with a Loc, so the renderer
// can show snippet + caret for the offending node.
func TestConvertLiteral_LocatedErrors(t *testing.T) {
	tests := []struct {
		name string
		// yaml is a YAML scalar that triggers the rejection branch.
		yaml string
		// wantInMsg, when non-empty, must appear in the error Message.
		wantInMsg string
	}{
		{
			// Value chosen to overflow int64 (max = 9223372036854775807)
			// but still fit in uint64 (max = 18446744073709551615), so
			// goccy parses it as IntegerNode{Value: uint64(...)} and
			// convertLiteral hits the overflow branch. Numbers larger
			// than uint64 max get parsed as *ast.StringNode by goccy
			// and would not exercise this path.
			name: "integer_overflow",
			yaml: "9999999999999999999",
		},
		{
			name:      "infinity_rejected",
			yaml:      ".inf",
			wantInMsg: ".inf",
		},
		{
			name:      "nan_rejected",
			yaml:      ".nan",
			wantInMsg: ".nan",
		},
		{
			name: "mapping_unsupported",
			yaml: "{a: b}",
		},
	}

	p := NewParser()
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			node := mustParseNode(t, tt.yaml)
			_, err := p.convertLiteral(node, "/x.yaml")
			if err == nil {
				t.Fatal("expected error")
			}
			var le *srcloc.Error
			if !errors.As(err, &le) || le.Loc == nil {
				t.Fatalf("expected located *srcloc.Error, got %T: %v", err, err)
			}
			if tt.wantInMsg != "" && !strings.Contains(le.Message, tt.wantInMsg) {
				t.Errorf("expected %q in Message, got %q", tt.wantInMsg, le.Message)
			}
		})
	}
}

func TestParameterMissingValueError(t *testing.T) {
	raw := &RawConfig{
		Parameters: map[string]RawParameter{
			"p": {},
		},
	}
	_, err := NewParser().ConvertConfigWithDirAndFile(raw, "", "")
	if err == nil || !strings.Contains(err.Error(), "null value is not supported") {
		t.Fatalf("expected null-value error, got %v", err)
	}
}
