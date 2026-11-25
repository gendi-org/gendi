package di

import (
	"errors"
	"testing"
)

type stubPass struct {
	name    string
	process func(cfg *Config) (*Config, error)
}

func (p *stubPass) Name() string {
	return p.name
}

func (p *stubPass) Process(cfg *Config) (*Config, error) {
	return p.process(cfg)
}

func TestApplyPasses(t *testing.T) {
	errBoom := errors.New("boom")
	keep := func(cfg *Config) (*Config, error) { return cfg, nil }
	addService := func(id string) func(cfg *Config) (*Config, error) {
		return func(cfg *Config) (*Config, error) {
			cfg.Services[id] = Service{}
			return cfg, nil
		}
	}

	tests := []struct {
		name         string
		passes       []Pass
		wantServices []string
		wantErr      string
		wantErrIs    error
	}{
		{
			name: "chains the passes",
			passes: []Pass{
				&stubPass{name: "first", process: addService("a")},
				&stubPass{name: "second", process: addService("b")},
			},
			wantServices: []string{"a", "b"},
		},
		{
			name: "rejects a nil config",
			passes: []Pass{
				&stubPass{name: "broken", process: func(*Config) (*Config, error) { return nil, nil }},
				&stubPass{name: "next", process: keep},
			},
			wantErr: `pass "broken" returned a nil config`,
		},
		{
			name: "wraps the error of a pass",
			passes: []Pass{
				&stubPass{name: "failing", process: func(*Config) (*Config, error) { return nil, errBoom }},
			},
			wantErr:   `pass "failing" failed: boom`,
			wantErrIs: errBoom,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ApplyPasses(NewConfig(), tt.passes)
			if tt.wantErr != "" {
				if err == nil || err.Error() != tt.wantErr {
					t.Fatalf("error = %v, want %q", err, tt.wantErr)
				}
				if tt.wantErrIs != nil && !errors.Is(err, tt.wantErrIs) {
					t.Fatalf("error %v does not wrap %v", err, tt.wantErrIs)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			for _, id := range tt.wantServices {
				if _, ok := got.Services[id]; !ok {
					t.Errorf("service %q is missing", id)
				}
			}
		})
	}
}
