package ir

import (
	"math"
	"strings"
	"testing"

	di "github.com/gendi-org/gendi"
	"github.com/gendi-org/gendi/srcloc"
)

func TestServicePhaseValidatesEmptyID(t *testing.T) {
	tests := []struct {
		name        string
		serviceID   string
		expectError bool
		errorMsg    string
	}{
		{
			name:        "empty service ID",
			serviceID:   "",
			expectError: true,
			errorMsg:    "service ID cannot be empty",
		},
		{
			name:        "whitespace-only service ID - single space",
			serviceID:   " ",
			expectError: true,
			errorMsg:    "cannot be whitespace-only",
		},
		{
			name:        "whitespace-only service ID - multiple spaces",
			serviceID:   "   ",
			expectError: true,
			errorMsg:    "cannot be whitespace-only",
		},
		{
			name:        "whitespace-only service ID - tab",
			serviceID:   "\t",
			expectError: true,
			errorMsg:    "cannot be whitespace-only",
		},
		{
			name:        "valid service ID",
			serviceID:   "my-service",
			expectError: false,
		},
		{
			name:        "valid service ID with spaces",
			serviceID:   "my service",
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &di.Config{
				Services: map[string]di.Service{
					tt.serviceID: {
						Public: true,
					},
				},
			}

			container := NewContainer()

			phase := &servicePhase{}
			err := phase.Apply(cfg, container)

			if tt.expectError {
				if err == nil {
					t.Errorf("expected error containing %q, got nil", tt.errorMsg)
				} else if !strings.Contains(err.Error(), tt.errorMsg) {
					t.Errorf("expected error containing %q, got: %v", tt.errorMsg, err)
				}
			} else {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				}
			}
		})
	}
}

func TestServicePhaseRejectsSharedAlias(t *testing.T) {
	cfg := &di.Config{
		Services: map[string]di.Service{
			"target": {},
			"alias": {
				Alias:  "target",
				Shared: true,
			},
		},
	}

	err := (&servicePhase{}).Apply(cfg, NewContainer())
	if err == nil {
		t.Fatal("expected shared alias to fail")
	}
	if !strings.Contains(err.Error(), `service "alias": alias cannot define shared`) {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestServicePhaseRejectsAutoconfiguredAlias(t *testing.T) {
	cfg := &di.Config{
		Services: map[string]di.Service{
			"target": {Autoconfigure: true},
			"alias": {
				Alias:         "target",
				Autoconfigure: true,
			},
		},
	}

	err := (&servicePhase{}).Apply(cfg, NewContainer())
	if err == nil {
		t.Fatal("expected autoconfigured alias to fail")
	}
	want := `service "alias": alias cannot define autoconfigure; autoconfigure applies to target "target"`
	if err.Error() != want {
		t.Fatalf("error = %q, want %q", err.Error(), want)
	}
}

func TestServicePhaseSortKey(t *testing.T) {
	loc := &srcloc.Location{File: "gendi.yaml", Line: 7, Column: 9}
	prefix := `gendi.yaml:7:9: service "svc": tag "handler": sort_by attribute "priority": `

	tests := []struct {
		name    string
		attrs   map[string]any
		want    int
		wantErr string
	}{
		{name: "int", attrs: map[string]any{"priority": 5}, want: 5},
		{name: "negative int64", attrs: map[string]any{"priority": int64(-3)}, want: -3},
		{name: "uint64", attrs: map[string]any{"priority": uint64(7)}, want: 7},
		{name: "integral float", attrs: map[string]any{"priority": 2.0}, want: 2},
		{name: "decimal string", attrs: map[string]any{"priority": "10"}, want: 10},
		{name: "attribute absent", attrs: map[string]any{"other": "x"}, want: 0},
		{
			name:    "word string",
			attrs:   map[string]any{"priority": "high"},
			wantErr: prefix + `"high" is not an integer`,
		},
		{
			name:    "fractional float",
			attrs:   map[string]any{"priority": 1.5},
			wantErr: prefix + `1.5 is not an integer`,
		},
		{
			name:    "bool",
			attrs:   map[string]any{"priority": true},
			wantErr: prefix + `true is not an integer`,
		},
		{
			name:    "null",
			attrs:   map[string]any{"priority": nil},
			wantErr: prefix + `<nil> is not an integer`,
		},
		{
			name:    "uint64 above MaxInt",
			attrs:   map[string]any{"priority": uint64(math.MaxUint64)},
			wantErr: prefix + `18446744073709551615 overflows int`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &di.Config{
				Services: map[string]di.Service{
					"svc": {Tags: []di.ServiceTag{{
						Name:       "handler",
						Attributes: tt.attrs,
						SourceLoc:  loc,
					}}},
				},
			}
			container := NewContainer()
			container.tags["handler"] = &Tag{Name: "handler", SortBy: "priority"}

			err := (&servicePhase{}).Apply(cfg, container)
			if tt.wantErr != "" {
				if err == nil {
					t.Fatalf("expected error %q, got nil", tt.wantErr)
				}
				if err.Error() != tt.wantErr {
					t.Fatalf("error = %q, want %q", err.Error(), tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got := container.Services["svc"].Tags[0].SortKey; got != tt.want {
				t.Errorf("SortKey = %d, want %d", got, tt.want)
			}
		})
	}
}

func TestServicePhaseIgnoresAttributesWithoutSortBy(t *testing.T) {
	cfg := &di.Config{
		Services: map[string]di.Service{
			"svc": {Tags: []di.ServiceTag{{
				Name:       "handler",
				Attributes: map[string]any{"priority": "high"},
			}}},
		},
	}

	if err := (&servicePhase{}).Apply(cfg, NewContainer()); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestServicePhaseErrorIsDeterministic(t *testing.T) {
	tests := []struct {
		name     string
		services map[string]di.Service
		wantErr  string
	}{
		{
			name: "shared alias",
			services: map[string]di.Service{
				"target": {},
				"zzz":    {Alias: "target", Shared: true},
				"mmm":    {Alias: "target", Shared: true},
				"kkk":    {Alias: "target", Shared: true},
			},
			wantErr: `service "kkk": alias cannot define shared; lifecycle is inherited from target "target"`,
		},
		{
			name: "whitespace-only ID",
			services: map[string]di.Service{
				"\t": {},
				"  ": {},
				" ":  {},
			},
			wantErr: `service ID "\t" cannot be whitespace-only`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &di.Config{Services: tt.services}
			for range 50 {
				err := (&servicePhase{}).Apply(cfg, NewContainer())
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
