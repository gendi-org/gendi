package stdlib

import (
	"errors"
	"reflect"
	"testing"

	di "github.com/gendi-org/gendi"
	"github.com/gendi-org/gendi/srcloc"
)

func TestSLogPass(t *testing.T) {
	loc := func(line int) *srcloc.Location { return srcloc.NewLocation("gendi.yaml", line, 7) }
	loggerArg := di.Argument{Kind: di.ArgServiceRef, Value: "logger"}
	taggedService := func(tagLoc *srcloc.Location) di.Service {
		return di.Service{
			Constructor: di.Constructor{Func: "app.New", Args: []di.Argument{loggerArg}},
			Tags: []di.ServiceTag{{
				Name:       "slog",
				Attributes: map[string]any{"channel": "db"},
				SourceLoc:  tagLoc,
			}},
			SourceLoc: loc(2),
		}
	}

	tests := []struct {
		name     string
		services map[string]di.Service
		// wantServices holds the expected services, without SourceLoc and Tags.
		wantServices map[string]di.Service
		wantErr      string
		wantLine     int
	}{
		{
			name:     "adds a channel logger",
			services: map[string]di.Service{"repo": taggedService(loc(5))},
			wantServices: map[string]di.Service{
				"repo": {Constructor: di.Constructor{
					Func: "app.New",
					Args: []di.Argument{{Kind: di.ArgServiceRef, Value: "repo.logger"}},
				}},
				"repo.logger": {Constructor: di.Constructor{
					Method: "@logger.With",
					Args: []di.Argument{
						{Kind: di.ArgLiteral, Literal: di.NewStringLiteral("channel")},
						{Kind: di.ArgLiteral, Literal: di.NewStringLiteral("db")},
					},
				}},
			},
		},
		{
			name: "ignores a tag without a channel",
			services: map[string]di.Service{"repo": {
				Constructor: di.Constructor{Func: "app.New", Args: []di.Argument{loggerArg}},
				Tags:        []di.ServiceTag{{Name: "slog", Attributes: map[string]any{}}},
			}},
			wantServices: map[string]di.Service{
				"repo": {Constructor: di.Constructor{Func: "app.New", Args: []di.Argument{loggerArg}}},
			},
		},
		{
			name: "rejects a declared channel logger",
			services: map[string]di.Service{
				"repo":        taggedService(loc(5)),
				"repo.logger": {Constructor: di.Constructor{Func: "app.NewLogger"}, SourceLoc: loc(9)},
			},
			wantErr:  `service "repo": the slog tag adds the service "repo.logger", but the config already has a service with this ID`,
			wantLine: 5,
		},
		{
			name: "reports the service when the tag has no location",
			services: map[string]di.Service{
				"repo":        taggedService(nil),
				"repo.logger": {Constructor: di.Constructor{Func: "app.NewLogger"}},
			},
			wantErr:  `service "repo": the slog tag adds the service "repo.logger", but the config already has a service with this ID`,
			wantLine: 2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := (&SLogPass{}).Process(&di.Config{Services: tt.services})
			if tt.wantErr != "" {
				var locErr *srcloc.Error
				if !errors.As(err, &locErr) || locErr.Loc == nil {
					t.Fatalf("expected located error %q, got %v", tt.wantErr, err)
				}
				if locErr.Message != tt.wantErr {
					t.Errorf("message = %q, want %q", locErr.Message, tt.wantErr)
				}
				if locErr.Loc.Line != tt.wantLine {
					t.Errorf("line = %d, want %d", locErr.Loc.Line, tt.wantLine)
				}
				return
			}
			if err != nil {
				t.Fatalf("process: %v", err)
			}

			for id, svc := range got.Services {
				svc.SourceLoc = nil
				svc.Tags = nil
				got.Services[id] = svc
			}
			if !reflect.DeepEqual(got.Services, tt.wantServices) {
				t.Errorf("services:\n got: %#v\nwant: %#v", got.Services, tt.wantServices)
			}
		})
	}
}
