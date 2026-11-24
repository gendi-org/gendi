package ir

import (
	"go/types"
	"testing"

	di "github.com/gendi-org/gendi"
)

func TestTagPhaseOptionalElementType(t *testing.T) {
	tests := []struct {
		name        string
		tags        map[string]di.Tag
		expectError bool
		errorMsg    string
	}{
		{
			name: "tag with element_type",
			tags: map[string]di.Tag{
				"test.tag": {
					ElementType: "string", // Simple type for testing
					SortBy:      "priority",
				},
			},
			expectError: false,
		},
		{
			name: "tag without element_type (marker tag)",
			tags: map[string]di.Tag{
				"marker.tag": {
					SortBy: "priority",
				},
			},
			expectError: false,
		},
		{
			name: "public tag without element_type",
			tags: map[string]di.Tag{
				"public.tag": {
					Public: true,
				},
			},
			expectError: true,
			errorMsg:    "public requires element_type",
		},
		{
			name: "public tag with element_type",
			tags: map[string]di.Tag{
				"public.tag": {
					ElementType: "string",
					Public:      true,
				},
			},
			expectError: false,
		},
		{
			name:        "empty tags map",
			tags:        map[string]di.Tag{},
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &di.Config{Tags: tt.tags}
			container := NewContainer()

			p := &tagPhase{resolver: &mockResolver{}}
			err := p.Apply(cfg, container)

			if tt.expectError {
				if err == nil {
					t.Errorf("expected error containing %q, got nil", tt.errorMsg)
				}
			} else {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				}
			}
		})
	}
}

func TestServicePhaseCreatesTagsOnDemand(t *testing.T) {
	cfg := &di.Config{
		Tags: map[string]di.Tag{}, // No declared tags
		Services: map[string]di.Service{
			"test.service": {
				Type: "string",
				Tags: []di.ServiceTag{
					{Name: "undeclared.tag"},
				},
			},
		},
	}

	container := NewContainer()

	// Build tags first (empty)
	if err := (&tagPhase{resolver: &mockResolver{}}).Apply(cfg, container); err != nil {
		t.Fatalf("tagPhase failed: %v", err)
	}

	// Build services - should create tag on demand
	if err := (&servicePhase{}).Apply(cfg, container); err != nil {
		t.Fatalf("servicePhase failed: %v", err)
	}

	// Check that tag was created
	tag, ok := container.tags["undeclared.tag"]
	if !ok {
		t.Fatal("expected tag 'undeclared.tag' to be created on demand")
	}

	if tag.Name != "undeclared.tag" {
		t.Errorf("expected tag name 'undeclared.tag', got '%s'", tag.Name)
	}

	// ElementType should be nil (will be inferred later from constructor)
	if tag.ElementType != nil {
		t.Errorf("expected nil ElementType for on-demand tag, got %v", tag.ElementType)
	}
}

func TestTagPhaseAutoconfigureValidation(t *testing.T) {
	tests := []struct {
		name        string
		tags        map[string]di.Tag
		expectError bool
		errorMsg    string
	}{
		{
			name: "autoconfigure tag without element_type",
			tags: map[string]di.Tag{
				"auto.tag": {
					Autoconfigure: true,
				},
			},
			expectError: true,
			errorMsg:    "autoconfigure requires element_type",
		},
		{
			name: "autoconfigure tag with sort_by",
			tags: map[string]di.Tag{
				"auto.tag": {
					ElementType:   "iface",
					Autoconfigure: true,
					SortBy:        "priority",
				},
			},
			expectError: true,
			errorMsg:    "autoconfigure cannot be used with sort_by",
		},
		{
			name: "autoconfigure tag with non-interface element_type",
			tags: map[string]di.Tag{
				"auto.tag": {
					ElementType:   "string",
					Autoconfigure: true,
				},
			},
			expectError: true,
			errorMsg:    "autoconfigure element_type must be an interface",
		},
		{
			name: "autoconfigure tag with interface element_type",
			tags: map[string]di.Tag{
				"auto.tag": {
					ElementType:   "iface",
					Autoconfigure: true,
				},
			},
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &di.Config{Tags: tt.tags}
			container := NewContainer()

			p := &tagPhase{resolver: &autoconfigureTagResolver{}}
			err := p.Apply(cfg, container)

			if tt.expectError {
				if err == nil {
					t.Errorf("expected error containing %q, got nil", tt.errorMsg)
				}
				return
			}

			if err != nil {
				t.Errorf("unexpected error: %v", err)
			}
		})
	}
}

// mockResolver is a simple resolver for testing
type mockResolver struct{}

func (m *mockResolver) LookupType(typeStr string) (types.Type, error) {
	// Return a basic type for testing
	return types.Typ[types.String], nil
}

func (m *mockResolver) LookupFunc(pkgPath, name string) (*types.Func, error) {
	return nil, nil
}

func (m *mockResolver) LookupMethod(recv types.Type, name string) (*types.Func, error) {
	return nil, nil
}

func (m *mockResolver) InstantiateFunc(fn *types.Func, typeArgs []string) (*types.Signature, []types.Type, error) {
	return nil, nil, nil
}

func (m *mockResolver) LookupVar(pkgPath, name string) (types.Object, error) {
	return nil, nil
}

type autoconfigureTagResolver struct{}

func (a *autoconfigureTagResolver) LookupType(typeStr string) (types.Type, error) {
	if typeStr == "iface" {
		iface := types.NewInterfaceType(nil, nil)
		iface.Complete()
		return iface, nil
	}
	return types.Typ[types.String], nil
}

func (a *autoconfigureTagResolver) LookupFunc(pkgPath, name string) (*types.Func, error) {
	return nil, nil
}

func (a *autoconfigureTagResolver) LookupMethod(recv types.Type, name string) (*types.Func, error) {
	return nil, nil
}

func (a *autoconfigureTagResolver) InstantiateFunc(fn *types.Func, typeArgs []string) (*types.Signature, []types.Type, error) {
	return nil, nil, nil
}

func (a *autoconfigureTagResolver) LookupVar(pkgPath, name string) (types.Object, error) {
	return nil, nil
}

func TestTagPhaseErrorIsDeterministic(t *testing.T) {
	tests := []struct {
		name    string
		tags    map[string]di.Tag
		wantErr string
	}{
		{
			name: "public without element_type",
			tags: map[string]di.Tag{
				"zzz": {Public: true},
				"mmm": {Public: true},
				"kkk": {Public: true},
			},
			wantErr: `tag "kkk" public requires element_type`,
		},
		{
			name: "autoconfigure without element_type",
			tags: map[string]di.Tag{
				"zzz": {Autoconfigure: true},
				"mmm": {Autoconfigure: true},
				"kkk": {Autoconfigure: true},
			},
			wantErr: `tag "kkk" autoconfigure requires element_type`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &di.Config{Tags: tt.tags}
			for range 50 {
				err := (&tagPhase{resolver: &mockResolver{}}).Apply(cfg, NewContainer())
				if err == nil {
					t.Fatalf("expected error %q, got nil", tt.wantErr)
				}
				if err.Error() != tt.wantErr {
					t.Fatalf("error = %q, want %q", err.Error(), tt.wantErr)
				}
			}
		})
	}
}
