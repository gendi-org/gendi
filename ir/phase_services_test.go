package ir

import (
	"strings"
	"testing"

	di "github.com/gendi-org/gendi"
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
