package di

import (
	"errors"
	"reflect"
	"strings"
	"testing"

	"github.com/gendi-org/gendi/srcloc"
	"github.com/gendi-org/gendi/xmaps"
)

func TestDecoratorPassSingleDecorator(t *testing.T) {
	cfg := &Config{
		Services: map[string]Service{
			"base": {
				Autoconfigure: true,
				Constructor: Constructor{
					Func: "app.NewBase",
				},
			},
			"decorator": {
				Decorates:          "base",
				DecorationPriority: 10,
				Constructor: Constructor{
					Func: "app.NewDecorator",
					Args: []Argument{
						{Kind: ArgInner},
					},
				},
			},
		},
	}

	pass := &DecoratorPass{}
	result, err := pass.Process(cfg)
	if err != nil {
		t.Fatalf("DecoratorPass failed: %v", err)
	}

	// Should have 3 services: decorator.inner, decorator, base (alias)
	if len(result.Services) != 3 {
		t.Fatalf("expected 3 services, got %d", len(result.Services))
	}

	// Check inner service
	innerSvc, ok := result.Services["decorator.inner"]
	if !ok {
		t.Fatalf("expected decorator.inner service to be created")
	}
	if innerSvc.Constructor.Func != "app.NewBase" {
		t.Fatalf("expected inner service to have original constructor")
	}
	if innerSvc.Public {
		t.Fatalf("expected inner service to not be public")
	}
	if innerSvc.Autoconfigure {
		t.Fatalf("expected inner service to have autoconfigure disabled")
	}

	// Check base becomes alias
	baseSvc := result.Services["base"]
	if baseSvc.Alias != "decorator" {
		t.Fatalf("expected base to become alias to decorator, got: %q", baseSvc.Alias)
	}
	if baseSvc.Constructor.Func != "" {
		t.Fatalf("expected base constructor to be cleared")
	}

	// Check decorator args are rewritten
	decSvc := result.Services["decorator"]
	if len(decSvc.Constructor.Args) != 1 {
		t.Fatalf("expected decorator to have 1 arg")
	}
	arg := decSvc.Constructor.Args[0]
	if arg.Kind != ArgServiceRef {
		t.Fatalf("expected @.inner to be rewritten to service ref")
	}
	if arg.Value != "decorator.inner" {
		t.Fatalf("expected arg to reference decorator.inner, got: %q", arg.Value)
	}
	if decSvc.Decorates != "" {
		t.Fatalf("expected Decorates field to be cleared")
	}
}

func TestDecoratorPassChain(t *testing.T) {
	cfg := &Config{
		Services: map[string]Service{
			"base": {
				Constructor: Constructor{
					Func: "app.NewBase",
				},
			},
			"decA": {
				Decorates:          "base",
				DecorationPriority: 20,
				Constructor: Constructor{
					Func: "app.NewDecoratorA",
					Args: []Argument{{Kind: ArgInner}},
				},
			},
			"decB": {
				Decorates:          "base",
				DecorationPriority: 10,
				Constructor: Constructor{
					Func: "app.NewDecoratorB",
					Args: []Argument{{Kind: ArgInner}},
				},
			},
		},
	}

	pass := &DecoratorPass{}
	result, err := pass.Process(cfg)
	if err != nil {
		t.Fatalf("DecoratorPass failed: %v", err)
	}

	// Should have 4 services: decA.inner, decA, decB, base (alias)
	if len(result.Services) != 4 {
		t.Logf("Services found:")
		for id := range result.Services {
			t.Logf("  - %s", id)
		}
		t.Fatalf("expected 4 services, got %d", len(result.Services))
	}

	// Check inner service
	innerSvc, ok := result.Services["decA.inner"]
	if !ok {
		t.Fatalf("expected decA.inner service to be created")
	}
	if innerSvc.Constructor.Func != "app.NewBase" {
		t.Fatalf("expected inner service to have original constructor")
	}

	// Check base becomes alias to outermost decorator (decB)
	baseSvc := result.Services["base"]
	if baseSvc.Alias != "decB" {
		t.Fatalf("expected base to be alias to decB, got: %q", baseSvc.Alias)
	}

	// Check decA references inner
	decA := result.Services["decA"]
	if len(decA.Constructor.Args) != 1 {
		t.Fatalf("expected decA to have 1 arg")
	}
	if decA.Constructor.Args[0].Value != "decA.inner" {
		t.Fatalf("expected decA to reference decA.inner, got: %q", decA.Constructor.Args[0].Value)
	}

	// Check decB references decA
	decB := result.Services["decB"]
	if len(decB.Constructor.Args) != 1 {
		t.Fatalf("expected decB to have 1 arg")
	}
	if decB.Constructor.Args[0].Value != "decA" {
		t.Fatalf("expected decB to reference decA, got: %q", decB.Constructor.Args[0].Value)
	}
}

func TestDecoratorPassEqualPriorityUsesIDTieBreak(t *testing.T) {
	cfg := &Config{
		Services: map[string]Service{
			"base": {
				Constructor: Constructor{
					Func: "app.NewBase",
				},
			},
			"decorator.a": {
				Decorates:          "base",
				DecorationPriority: 10,
				Constructor: Constructor{
					Func: "app.NewDecoratorA",
					Args: []Argument{{Kind: ArgInner}},
				},
			},
			"decorator.b": {
				Decorates:          "base",
				DecorationPriority: 10,
				Constructor: Constructor{
					Func: "app.NewDecoratorB",
					Args: []Argument{{Kind: ArgInner}},
				},
			},
		},
	}

	pass := &DecoratorPass{}
	result, err := pass.Process(cfg)
	if err != nil {
		t.Fatalf("DecoratorPass failed: %v", err)
	}

	baseSvc := result.Services["base"]
	if baseSvc.Alias != "decorator.b" {
		t.Fatalf("expected base to be alias to lexicographically later outer decorator, got: %q", baseSvc.Alias)
	}

	decA := result.Services["decorator.a"]
	if len(decA.Constructor.Args) != 1 {
		t.Fatalf("expected decorator.a to have 1 arg")
	}
	if decA.Constructor.Args[0].Value != "decorator.a.inner" {
		t.Fatalf("expected decorator.a to reference its inner service first, got: %q", decA.Constructor.Args[0].Value)
	}

	decB := result.Services["decorator.b"]
	if len(decB.Constructor.Args) != 1 {
		t.Fatalf("expected decorator.b to have 1 arg")
	}
	if decB.Constructor.Args[0].Value != "decorator.a" {
		t.Fatalf("expected decorator.b to wrap decorator.a on equal priority tie, got: %q", decB.Constructor.Args[0].Value)
	}
}

func TestDecoratorPassKeepsSharedFlagsIndependent(t *testing.T) {
	for _, tc := range []struct {
		name            string
		baseShared      bool
		decoratorShared bool
	}{
		{name: "shared base and non-shared decorator", baseShared: true, decoratorShared: false},
		{name: "non-shared base and shared decorator", baseShared: false, decoratorShared: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			cfg := &Config{
				Services: map[string]Service{
					"base": {
						Shared:      tc.baseShared,
						Constructor: Constructor{Func: "app.NewBase"},
					},
					"decorator": {
						Shared:    tc.decoratorShared,
						Decorates: "base",
						Constructor: Constructor{
							Func: "app.NewDecorator",
							Args: []Argument{{Kind: ArgInner}},
						},
					},
				},
			}

			result, err := (&DecoratorPass{}).Process(cfg)
			if err != nil {
				t.Fatalf("DecoratorPass failed: %v", err)
			}
			if got := result.Services["decorator.inner"].Shared; got != tc.baseShared {
				t.Errorf("inner shared = %t, want %t", got, tc.baseShared)
			}
			if got := result.Services["decorator"].Shared; got != tc.decoratorShared {
				t.Errorf("decorator shared = %t, want %t", got, tc.decoratorShared)
			}
			if result.Services["base"].Shared {
				t.Error("alias must not define shared")
			}
		})
	}
}

func TestDecoratorPassErrors(t *testing.T) {
	tests := []struct {
		name    string
		cfg     *Config
		wantErr string
	}{
		{
			// Cycle A -> B -> C -> A is caught by the "cannot be decorated"
			// check: processing C (decorates A) finds A.Decorates="b" non-empty.
			name: "cycle detection",
			cfg: &Config{
				Services: map[string]Service{
					"a": {Decorates: "b", Constructor: Constructor{Func: "app.NewA"}},
					"b": {Decorates: "c", Constructor: Constructor{Func: "app.NewB"}},
					"c": {Decorates: "a", Constructor: Constructor{Func: "app.NewC"}},
				},
			},
			wantErr: "cannot be decorated",
		},
		{
			name: "decorator cannot be decorated",
			cfg: &Config{
				Services: map[string]Service{
					"base": {Constructor: Constructor{Func: "app.NewBase"}},
					"decA": {Decorates: "base", Constructor: Constructor{Func: "app.NewDecoratorA"}},
					"decB": {Decorates: "decA", Constructor: Constructor{Func: "app.NewDecoratorB"}},
				},
			},
			wantErr: "cannot be decorated",
		},
		{
			name: "unknown base",
			cfg: &Config{
				Services: map[string]Service{
					"decorator": {Decorates: "unknown", Constructor: Constructor{Func: "app.NewDecorator"}},
				},
			},
			wantErr: "unknown service",
		},
		{
			name: "rejects spread inner",
			cfg: &Config{
				Services: map[string]Service{
					"base": {Constructor: Constructor{Func: "app.NewBase"}},
					"decorator": {
						Decorates: "base",
						Constructor: Constructor{
							Func: "app.NewDecorator",
							Args: []Argument{{Kind: ArgSpread, Value: "@.inner"}},
						},
					},
				},
			},
			wantErr: "!spread:@.inner is not supported",
		},
		{
			name: "rejects field access on inner",
			cfg: &Config{
				Services: map[string]Service{
					"base": {Constructor: Constructor{Func: "app.NewBase"}},
					"decorator": {
						Decorates: "base",
						Constructor: Constructor{
							Func: "app.NewDecorator",
							Args: []Argument{{Kind: ArgFieldAccessService, Value: ".inner.Field"}},
						},
					},
				},
			},
			wantErr: "!field:@.inner.Field is not supported",
		},
		{
			name: "rejects field access on inner without a field",
			cfg: &Config{
				Services: map[string]Service{
					"base": {Constructor: Constructor{Func: "app.NewBase"}},
					"decorator": {
						Decorates: "base",
						Constructor: Constructor{
							Func: "app.NewDecorator",
							Args: []Argument{{Kind: ArgFieldAccessService, Value: ".inner"}},
						},
					},
				},
			},
			wantErr: "!field:@.inner is not supported",
		},
		{
			name: "rejects reserved .inner suffix service ID",
			cfg: &Config{
				Services: map[string]Service{
					"decorator.inner": {Constructor: Constructor{Func: "app.NewInner"}},
				},
			},
			wantErr: "cannot use reserved .inner suffix",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := (&DecoratorPass{}).Process(tt.cfg)
			if err == nil {
				t.Fatalf("expected error containing %q, got nil", tt.wantErr)
			}
			if !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("expected error containing %q, got: %v", tt.wantErr, err)
			}
		})
	}
}

func TestDecoratorPassErrorLocation(t *testing.T) {
	loc := func(line int) *srcloc.Location { return srcloc.NewLocation("gendi.yaml", line, 3) }
	ctor := Constructor{Func: "app.New"}
	decorates := func(base string, line int) Service {
		return Service{
			Decorates:   base,
			Constructor: Constructor{Func: "app.NewDec", Args: []Argument{{Kind: ArgInner}}},
			SourceLoc:   loc(line),
		}
	}

	tests := []struct {
		name     string
		services map[string]Service
		wantLine int
		wantMsg  string
	}{
		{
			name: "reserved .inner suffix",
			services: map[string]Service{
				"svc.inner": {Constructor: ctor, SourceLoc: loc(2)},
			},
			wantLine: 2,
			wantMsg:  `service ID "svc.inner" cannot use reserved .inner suffix`,
		},
		{
			name: "unknown base",
			services: map[string]Service{
				"dec": decorates("nope", 2),
			},
			wantLine: 2,
			wantMsg:  `decorator "dec" decorates unknown service "nope"`,
		},
		{
			name: "decorated decorator",
			services: map[string]Service{
				"base": {Constructor: ctor, SourceLoc: loc(2)},
				"decA": decorates("base", 4),
				"decB": decorates("decA", 8),
			},
			wantLine: 8,
			wantMsg:  `decorator "decA" cannot be decorated by "decB"`,
		},
		{
			name: "spread of inner",
			services: map[string]Service{
				"base": {Constructor: ctor, SourceLoc: loc(2)},
				"dec": {
					Decorates: "base",
					Constructor: Constructor{
						Func: "app.NewDec",
						Args: []Argument{{Kind: ArgSpread, Value: "@.inner", SourceLoc: loc(7)}},
					},
					SourceLoc: loc(4),
				},
			},
			wantLine: 7,
			wantMsg:  `decorator "dec" arg[0]: !spread:@.inner is not supported; use @.inner directly`,
		},
		{
			name: "field access on inner",
			services: map[string]Service{
				"base": {Constructor: ctor, SourceLoc: loc(2)},
				"dec": {
					Decorates: "base",
					Constructor: Constructor{
						Func: "app.NewDec",
						Args: []Argument{
							{Kind: ArgInner},
							{Kind: ArgFieldAccessService, Value: ".inner.Name", SourceLoc: loc(8)},
						},
					},
					SourceLoc: loc(4),
				},
			},
			wantLine: 8,
			wantMsg:  `decorator "dec" arg[1]: !field:@.inner.Name is not supported; use @.inner directly`,
		},
		{
			// Before the fix, the pass added an empty service "nope", and
			// the generator reported "missing constructor" without a location.
			name: "alias base with an unknown target",
			services: map[string]Service{
				"a":   {Alias: "nope", SourceLoc: loc(2)},
				"dec": decorates("a", 4),
			},
			wantLine: 2,
			wantMsg:  `service "a" alias target "nope" not found`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &Config{Services: tt.services}
			_, err := (&DecoratorPass{}).Process(cfg)
			var locErr *srcloc.Error
			if !errors.As(err, &locErr) || locErr.Loc == nil {
				t.Fatalf("expected located error, got %v", err)
			}
			if locErr.Loc.Line != tt.wantLine {
				t.Errorf("line = %d, want %d", locErr.Loc.Line, tt.wantLine)
			}
			if locErr.Message != tt.wantMsg {
				t.Errorf("message = %q, want %q", locErr.Message, tt.wantMsg)
			}
			if _, ok := cfg.Services["nope"]; ok {
				t.Error(`the pass added a service "nope" that the config does not declare`)
			}
		})
	}
}

func TestDecoratorPassNoDecorators(t *testing.T) {
	cfg := &Config{
		Services: map[string]Service{
			"service1": {
				Constructor: Constructor{
					Func: "app.NewService1",
				},
			},
			"service2": {
				Constructor: Constructor{
					Func: "app.NewService2",
				},
			},
		},
	}

	pass := &DecoratorPass{}
	result, err := pass.Process(cfg)
	if err != nil {
		t.Fatalf("DecoratorPass failed: %v", err)
	}

	// Should remain unchanged
	if len(result.Services) != 2 {
		t.Fatalf("expected 2 services, got %d", len(result.Services))
	}
}

func TestDecoratorPassMultipleArgsWithInner(t *testing.T) {
	cfg := &Config{
		Services: map[string]Service{
			"base": {
				Constructor: Constructor{
					Func: "app.NewBase",
				},
			},
			"decorator": {
				Decorates:          "base",
				DecorationPriority: 10,
				Constructor: Constructor{
					Func: "app.NewDecorator",
					Args: []Argument{
						{Kind: ArgServiceRef, Value: "logger"},
						{Kind: ArgInner},
						{Kind: ArgParam, Value: "timeout"},
					},
				},
			},
		},
	}

	pass := &DecoratorPass{}
	result, err := pass.Process(cfg)
	if err != nil {
		t.Fatalf("DecoratorPass failed: %v", err)
	}

	decSvc := result.Services["decorator"]
	if len(decSvc.Constructor.Args) != 3 {
		t.Fatalf("expected 3 args, got %d", len(decSvc.Constructor.Args))
	}

	// First arg unchanged
	if decSvc.Constructor.Args[0].Kind != ArgServiceRef || decSvc.Constructor.Args[0].Value != "logger" {
		t.Fatalf("expected first arg to be @logger")
	}

	// Second arg rewritten
	if decSvc.Constructor.Args[1].Kind != ArgServiceRef || decSvc.Constructor.Args[1].Value != "decorator.inner" {
		t.Fatalf("expected second arg to be @decorator.inner")
	}

	// Third arg unchanged
	if decSvc.Constructor.Args[2].Kind != ArgParam || decSvc.Constructor.Args[2].Value != "timeout" {
		t.Fatalf("expected third arg to be %%timeout%%")
	}
}

func TestDecoratorPassMovesTags(t *testing.T) {
	handler := func(priority int) ServiceTag {
		return ServiceTag{Name: "handler", Attributes: map[string]any{"priority": priority}}
	}
	decorator := func(base string, priority int, tags ...ServiceTag) Service {
		return Service{
			Decorates:          base,
			DecorationPriority: priority,
			Tags:               tags,
			Constructor: Constructor{
				Func: "app.NewDecorator",
				Args: []Argument{{Kind: ArgInner}},
			},
		}
	}
	base := func(tags ...ServiceTag) Service {
		return Service{
			Type:        "app.Service",
			Tags:        tags,
			Constructor: Constructor{Func: "app.NewBase"},
			SourceLoc:   &srcloc.Location{File: "gendi.yaml", Line: 3},
		}
	}

	tests := []struct {
		name     string
		services map[string]Service
		// wantTags holds the tags of each listed service after the pass.
		// A service that the pass made an alias always has no tags.
		wantTags map[string][]ServiceTag
	}{
		{
			name: "tag on the base",
			services: map[string]Service{
				"base": base(handler(1)),
				"dec":  decorator("base", 0),
			},
			wantTags: map[string][]ServiceTag{
				"dec":       {handler(1)},
				"dec.inner": nil,
			},
		},
		{
			name: "tag on the decorator",
			services: map[string]Service{
				"base": base(),
				"dec":  decorator("base", 0, handler(2)),
			},
			wantTags: map[string][]ServiceTag{
				"dec":       {handler(2)},
				"dec.inner": nil,
			},
		},
		{
			// The entries of the decorator replace all entries of the base
			// with the same name. Other names of the base move.
			name: "tags on both",
			services: map[string]Service{
				"base": base(handler(1), ServiceTag{Name: "other"}, handler(3)),
				"dec":  decorator("base", 0, handler(2)),
			},
			wantTags: map[string][]ServiceTag{
				"dec":       {{Name: "other"}, handler(2)},
				"dec.inner": nil,
			},
		},
		{
			// The tags of the base go to the outermost decorator. The inner
			// decorator keeps its own tags: it is still a service under its
			// own ID. Symfony moves them to the outermost decorator too.
			name: "chain of two decorators",
			services: map[string]Service{
				"base": base(handler(1)),
				"decA": decorator("base", 20, ServiceTag{Name: "a"}),
				"decB": decorator("base", 10),
			},
			wantTags: map[string][]ServiceTag{
				"decA.inner": nil,
				"decA":       {{Name: "a"}},
				"decB":       {handler(1)},
			},
		},
		{
			// Only the outermost decorator replaces the tags of the base. A
			// tag of the same name on the inner decorator is its own tag.
			name: "chain with a tag of the same name on the inner decorator",
			services: map[string]Service{
				"base": base(handler(1)),
				"decA": decorator("base", 20, handler(2)),
				"decB": decorator("base", 10),
			},
			wantTags: map[string][]ServiceTag{
				"decA.inner": nil,
				"decA":       {handler(2)},
				"decB":       {handler(1)},
			},
		},
		{
			name: "chain with a tag of the same name on the outermost decorator",
			services: map[string]Service{
				"base": base(handler(1)),
				"decA": decorator("base", 20, handler(2)),
				"decB": decorator("base", 10, handler(3)),
			},
			wantTags: map[string][]ServiceTag{
				"decA.inner": nil,
				"decA":       {handler(2)},
				"decB":       {handler(3)},
			},
		},
		{
			// Symfony moves no tags from an alias. The target keeps its own
			// tags, and the tags of the alias are dropped.
			name: "base is a user alias",
			services: map[string]Service{
				"target": base(handler(1)),
				"base":   {Alias: "target", Tags: []ServiceTag{{Name: "on_alias"}}},
				"dec":    decorator("base", 0),
			},
			wantTags: map[string][]ServiceTag{
				"target": {handler(1)},
				"dec":    nil,
			},
		},
		{
			// The decorators replace only the name of the alias, so no tags
			// move. Each decorator keeps its own tags.
			name: "chain on a user alias",
			services: map[string]Service{
				"target": base(handler(1)),
				"base":   {Alias: "target"},
				"decA":   decorator("base", 20, ServiceTag{Name: "a"}),
				"decB":   decorator("base", 10),
			},
			wantTags: map[string][]ServiceTag{
				"target": {handler(1)},
				"decA":   {{Name: "a"}},
				"decB":   nil,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := (&DecoratorPass{}).Process(&Config{Services: tt.services})
			if err != nil {
				t.Fatalf("DecoratorPass failed: %v", err)
			}

			for _, id := range xmaps.OrderedKeys(tt.wantTags) {
				want := tt.wantTags[id]
				svc, ok := result.Services[id]
				if !ok {
					t.Fatalf("service %q is missing after the pass", id)
				}
				if !reflect.DeepEqual(svc.Tags, want) {
					t.Errorf("service %q tags:\nhave: %+v\nwant: %+v", id, svc.Tags, want)
				}
			}
			if tags := result.Services["base"].Tags; len(tags) != 0 {
				t.Errorf("expected no tags on the alias base, got %+v", tags)
			}
		})
	}
}

func TestDecoratorPassAliasKeepsTypeAndLocation(t *testing.T) {
	cfg := &Config{
		Services: map[string]Service{
			"base": {
				Type:        "app.Service",
				Constructor: Constructor{Func: "app.NewBase"},
				SourceLoc:   &srcloc.Location{File: "gendi.yaml", Line: 3},
			},
			"decorator": {
				Decorates: "base",
				Constructor: Constructor{
					Func: "app.NewDecorator",
					Args: []Argument{{Kind: ArgInner}},
				},
			},
		},
	}

	result, err := (&DecoratorPass{}).Process(cfg)
	if err != nil {
		t.Fatalf("DecoratorPass failed: %v", err)
	}

	alias := result.Services["base"]
	if alias.Type != "app.Service" {
		t.Errorf("expected alias to keep declared type, got %q", alias.Type)
	}
	if alias.SourceLoc == nil || alias.SourceLoc.Line != 3 {
		t.Errorf("expected alias to keep source location, got %+v", alias.SourceLoc)
	}
}

func TestDecoratorPassKeepsAliasTargetAndDecoratorSharedFlagsIndependent(t *testing.T) {
	cfg := &Config{
		Services: map[string]Service{
			"target": {
				Shared:      true,
				Constructor: Constructor{Func: "app.NewTarget"},
			},
			"base": {Alias: "target"},
			"dec": {
				Decorates: "base",
				Shared:    false,
				Constructor: Constructor{
					Func: "app.NewDec",
					Args: []Argument{{Kind: ArgInner}},
				},
			},
		},
	}

	result, err := (&DecoratorPass{}).Process(cfg)
	if err != nil {
		t.Fatalf("DecoratorPass failed: %v", err)
	}
	if !result.Services["target"].Shared {
		t.Error("alias target must stay shared")
	}
	if result.Services["dec"].Shared {
		t.Error("decorator must stay non-shared")
	}
	if result.Services["base"].Shared {
		t.Error("decorated alias must delegate to the non-shared decorator")
	}
}

func TestDecoratorPassErrorIsDeterministic(t *testing.T) {
	tests := []struct {
		name     string
		services map[string]Service
		wantErr  string
	}{
		{
			name: "reserved .inner suffix",
			services: map[string]Service{
				"zzz.inner": {},
				"mmm.inner": {},
				"kkk.inner": {},
			},
			wantErr: `service ID "kkk.inner" cannot use reserved .inner suffix`,
		},
		{
			name: "unknown base",
			services: map[string]Service{
				"dec-z": {Decorates: "nope-z"},
				"dec-m": {Decorates: "nope-m"},
				"dec-k": {Decorates: "nope-k"},
			},
			wantErr: `decorator "dec-k" decorates unknown service "nope-k"`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			for range 50 {
				_, err := (&DecoratorPass{}).Process(&Config{Services: tt.services})
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
