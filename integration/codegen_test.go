package integration

import (
	"strings"
	"testing"

	di "github.com/gendi-org/gendi"
	"github.com/gendi-org/gendi/pipeline"
)

func testEmitOptions(t *testing.T) pipeline.Options {
	t.Helper()
	opts := pipeline.Options{Out: ".", Package: "di"}
	if err := opts.Finalize(); err != nil {
		t.Fatalf("finalize options: %v", err)
	}

	return opts
}

func generate(t *testing.T, cfg *di.Config) string {
	t.Helper()

	code, err := pipeline.Emit(cfg, testEmitOptions(t))
	if err != nil {
		t.Fatalf("emit failed: %v", err)
	}

	return string(code)
}

func generateErr(t *testing.T, cfg *di.Config) error {
	t.Helper()

	_, err := pipeline.Emit(cfg, testEmitOptions(t))
	return err
}

func assertCodegen(t *testing.T, cfg *di.Config, wantContains, wantNotContains, wantErrContains []string) {
	t.Helper()

	if (len(wantContains) == 0 && len(wantNotContains) == 0) == (len(wantErrContains) == 0) {
		t.Fatal("exactly one codegen expectation must be configured")
	}

	if len(wantErrContains) != 0 {
		err := generateErr(t, cfg)
		if err == nil {
			t.Fatal("expected generation error")
		}
		for _, want := range wantErrContains {
			if !strings.Contains(err.Error(), want) {
				t.Fatalf("expected error containing %q, got %v", want, err)
			}
		}
		return
	}

	out := generate(t, cfg)
	for _, want := range wantContains {
		if !strings.Contains(out, want) {
			t.Fatalf("expected generated code containing %q, got:\n%s", want, out)
		}
	}
	for _, unwanted := range wantNotContains {
		if strings.Contains(out, unwanted) {
			t.Fatalf("expected generated code not to contain %q, got:\n%s", unwanted, out)
		}
	}
}

func TestReachabilityFromPublicRoots(t *testing.T) {
	const appPkg = "github.com/gendi-org/gendi/generator/testdata/app"

	for _, tt := range []struct {
		name            string
		cfg             *di.Config
		wantContains    []string
		wantNotContains []string
		wantErrContains []string
	}{
		{
			name: "no public roots",
			cfg: &di.Config{
				Services: map[string]di.Service{
					"a": {Constructor: di.Constructor{Func: appPkg + ".NewA"}},
				},
			},
			wantErrContains: []string{"at least one public service or tag"},
		},
		{
			name: "public service",
			cfg: &di.Config{
				Services: map[string]di.Service{
					"a": {Constructor: di.Constructor{Func: appPkg + ".NewA"}},
					"b": {
						Constructor: di.Constructor{
							Func: appPkg + ".NewB",
							Args: []di.Argument{{Kind: di.ArgServiceRef, Value: "a"}},
						},
						Public: true,
					},
					"unused": {Constructor: di.Constructor{Func: appPkg + ".NewC"}},
				},
			},
			wantContains:    []string{"GetB", "getA"},
			wantNotContains: []string{"GetA", "buildUnused", "getUnused", "GetUnused", "svc_unused"},
		},
		{
			name: "public tag",
			cfg: &di.Config{
				Tags: map[string]di.Tag{
					"svc.tag": {ElementType: appPkg + ".Service", Public: true},
				},
				Services: map[string]di.Service{
					"base": {
						Constructor: di.Constructor{Func: appPkg + ".NewServiceBase"},
						Tags:        []di.ServiceTag{{Name: "svc.tag"}},
					},
				},
			},
			wantContains: []string{
				"GetTaggedWithSvcTag",
				"getTaggedWithSvcTag",
				"[]app.Service",
				"getBase",
				"[]app.Service{",
			},
			wantNotContains: []string{"stdlib.MakeSlice"},
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			assertCodegen(t, tt.cfg, tt.wantContains, tt.wantNotContains, tt.wantErrContains)
		})
	}
}

func TestTaggedConversionCodegen(t *testing.T) {
	const appPkg = "github.com/gendi-org/gendi/generator/testdata/app"

	for _, tt := range []struct {
		name            string
		constructor     string
		wantContains    []string
		wantNotContains []string
	}{
		{
			name:        "interface slice",
			constructor: appPkg + ".NewInterfaceConsumer",
			wantContains: []string{
				"getTaggedWithTestTag",
				"var arg0_tagged_test_tag []interface{}",
				"arg0_tagged_test_tag = make([]interface{}, len(tagged_test_tag))",
				"for idx, item := range tagged_test_tag",
				"arg0_tagged_test_tag[idx] = item",
			},
		},
		{
			// Hub is a non-nilable value type whose constructor returns an
			// error, so the conversion error path must return its zero value.
			name:            "value type error path",
			constructor:     appPkg + ".NewHub",
			wantContains:    []string{"return zero, fmt.Errorf"},
			wantNotContains: []string{"return nil, fmt.Errorf"},
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &di.Config{
				Tags: map[string]di.Tag{
					"test.tag": {ElementType: "*" + appPkg + ".A"},
				},
				Services: map[string]di.Service{
					"a": {
						Constructor: di.Constructor{Func: appPkg + ".NewA"},
						Tags:        []di.ServiceTag{{Name: "test.tag"}},
					},
					"consumer": {
						Constructor: di.Constructor{
							Func: tt.constructor,
							Args: []di.Argument{{Kind: di.ArgTagged, Value: "test.tag"}},
						},
						Public: true,
					},
				},
			}

			assertCodegen(t, cfg, tt.wantContains, tt.wantNotContains, nil)
		})
	}
}

func TestParameterCodegen(t *testing.T) {
	const appPkg = "github.com/gendi-org/gendi/generator/testdata/app"

	for _, tt := range []struct {
		name            string
		parameter       string
		defaultValue    di.Literal
		constructor     string
		wantContains    []string
		wantErrContains []string
	}{
		{
			name:         "string resolution and container options",
			parameter:    "log_prefix",
			defaultValue: di.NewStringLiteral("[app] "),
			constructor:  appPkg + ".NewLogger",
			wantContains: []string{
				"NewContainer",
				`.String("log_prefix")`,
				"WithContainerParameterCaster",
			},
		},
		{
			name:         "duration resolution",
			parameter:    "timeout",
			defaultValue: di.NewStringLiteral("1s"),
			constructor:  appPkg + ".NewTimer",
			wantContains: []string{`.Duration("timeout")`},
		},
		{
			// Value exceeds MaxInt32: as an untyped constant in the defaults
			// map it would fail to compile on 32-bit targets.
			name:         "integer default rendered as int64",
			parameter:    "timeout",
			defaultValue: di.NewIntLiteral(2147483648),
			constructor:  appPkg + ".NewTimer",
			wantContains: []string{"int64(2147483648)"},
		},
		{
			name:            "invalid default rejected",
			parameter:       "timeout",
			defaultValue:    di.NewStringLiteral("not-a-duration"),
			constructor:     appPkg + ".NewTimer",
			wantErrContains: []string{"cannot cast"},
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &di.Config{
				Parameters: map[string]di.Parameter{
					tt.parameter: {Value: tt.defaultValue},
				},
				Services: map[string]di.Service{
					"service": {
						Constructor: di.Constructor{
							Func: tt.constructor,
							Args: []di.Argument{{Kind: di.ArgParam, Value: tt.parameter}},
						},
						Public: true,
					},
				},
			}

			assertCodegen(t, cfg, tt.wantContains, nil, tt.wantErrContains)
		})
	}
}

func TestLiteralArguments(t *testing.T) {
	const appPkg = "github.com/gendi-org/gendi/generator/testdata/app"
	literalServerArgs := func(value di.Literal) []di.Argument {
		return []di.Argument{
			{Kind: di.ArgLiteral, Literal: di.NewStringLiteral("localhost")},
			{Kind: di.ArgLiteral, Literal: value},
		}
	}

	for _, tt := range []struct {
		name            string
		constructor     string
		args            []di.Argument
		wantContains    []string
		wantErrContains []string
	}{
		{
			name:        "null for nilable argument",
			constructor: appPkg + ".NewB",
			args: []di.Argument{
				{Kind: di.ArgLiteral, Literal: di.NewNullLiteral()},
			},
			wantContains: []string{"NewB(nil)"},
		},
		{
			name:            "string rejected",
			constructor:     appPkg + ".NewServerWithAddr",
			args:            literalServerArgs(di.NewStringLiteral("hello world")),
			wantErrContains: []string{"cannot use", "arg[1]"},
		},
		{
			name:            "null rejected",
			constructor:     appPkg + ".NewServerWithAddr",
			args:            literalServerArgs(di.NewNullLiteral()),
			wantErrContains: []string{"not nilable"},
		},
		{
			name:            "bool rejected",
			constructor:     appPkg + ".NewServerWithAddr",
			args:            literalServerArgs(di.NewBoolLiteral(true)),
			wantErrContains: []string{"cannot use"},
		},
		{
			// Go permits untyped float constants with integral values for
			// integer targets, so 5.0 must keep generating.
			name:         "integral float accepted",
			constructor:  appPkg + ".NewServerWithAddr",
			args:         literalServerArgs(di.NewFloatLiteral(5.0)),
			wantContains: []string{`NewServerWithAddr("localhost", 5.0)`},
		},
		{
			name:        "integer for duration argument",
			constructor: appPkg + ".NewTimer",
			args: []di.Argument{
				{Kind: di.ArgLiteral, Literal: di.NewIntLiteral(5000000000)},
			},
			wantContains: []string{"NewTimer(5000000000)"},
		},
		{
			// Literals in variadic positions resolve against the element type
			// instead of the raw slice parameter.
			name:        "variadic duration strings",
			constructor: appPkg + ".NewTimed",
			args: []di.Argument{
				{Kind: di.ArgLiteral, Literal: di.NewStringLiteral("5s")},
				{Kind: di.ArgLiteral, Literal: di.NewStringLiteral("10s")},
			},
			wantContains: []string{"app.NewTimed("},
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &di.Config{
				Services: map[string]di.Service{
					"service": {
						Constructor: di.Constructor{Func: tt.constructor, Args: tt.args},
						Public:      true,
					},
				},
			}

			assertCodegen(t, cfg, tt.wantContains, nil, tt.wantErrContains)
		})
	}
}

func TestServiceAliasCodegen(t *testing.T) {
	for _, tt := range []struct {
		name            string
		alias           di.Service
		wantContains    []string
		wantNotContains []string
		wantErrContains []string
	}{
		{
			name:            "public getter forwards to target",
			alias:           di.Service{Alias: "logger", Public: true},
			wantContains:    []string{"GetLoggerAlias", "return c.getLogger()"},
			wantNotContains: []string{"buildLoggerAlias"},
		},
		{
			name:            "shared rejected",
			alias:           di.Service{Alias: "logger", Shared: true},
			wantErrContains: []string{`service "logger.alias": alias cannot define shared`},
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &di.Config{
				Parameters: map[string]di.Parameter{
					"log_prefix": {Value: di.NewStringLiteral("[app] ")},
				},
				Services: map[string]di.Service{
					"logger": {
						Constructor: di.Constructor{
							Func: "github.com/gendi-org/gendi/generator/testdata/app.NewLogger",
							Args: []di.Argument{{Kind: di.ArgParam, Value: "log_prefix"}},
						},
						Public: true,
					},
					"logger.alias": tt.alias,
				},
			}

			assertCodegen(t, cfg, tt.wantContains, tt.wantNotContains, tt.wantErrContains)
		})
	}
}

func TestDecoratorPrivateGetterGeneration(t *testing.T) {
	const appPkg = "github.com/gendi-org/gendi/generator/testdata/app"

	for _, tt := range []struct {
		name             string
		cfg              *di.Config
		wantContains     []string
		wantNotContains  []string
		wantCountAtLeast map[string]int
	}{
		{
			name: "decorator chain",
			cfg: &di.Config{
				Services: map[string]di.Service{
					"svc": {
						Constructor: di.Constructor{Func: appPkg + ".NewServiceBase"},
						Public:      true,
						Shared:      true,
					},
					"svc.decoratorA": {
						Constructor: di.Constructor{
							Func: appPkg + ".NewServiceDecoratorA",
							Args: []di.Argument{{Kind: di.ArgInner}},
						},
						Decorates:          "svc",
						DecorationPriority: 10,
						Shared:             true,
					},
					"svc.decoratorB": {
						Constructor: di.Constructor{
							Func: appPkg + ".NewServiceDecoratorB",
							Args: []di.Argument{{Kind: di.ArgInner}},
						},
						Decorates:          "svc",
						DecorationPriority: 20,
						Shared:             true,
					},
				},
			},
			wantContains: []string{
				"getSvcDecoratorB",
				"getSvcDecoratorA",
				"getSvcDecoratorAInner",
				"svc_svc_decoratorB",
			},
			wantNotContains: []string{"svc_svc_decoratorA "},
		},
		{
			name: "referenced decorator",
			cfg: &di.Config{
				Services: map[string]di.Service{
					"svc": {
						Constructor: di.Constructor{Func: appPkg + ".NewServiceBase"},
					},
					"svc.decoratorA": {
						Constructor: di.Constructor{
							Func: appPkg + ".NewServiceDecoratorA",
							Args: []di.Argument{{Kind: di.ArgInner}},
						},
						Decorates:          "svc",
						DecorationPriority: 10,
					},
					"consumer": {
						Constructor: di.Constructor{
							Func: appPkg + ".NewConsumer",
							Args: []di.Argument{{Kind: di.ArgServiceRef, Value: "svc.decoratorA"}},
						},
						Public: true,
					},
				},
			},
			wantContains:    []string{"getSvcDecoratorA"},
			wantNotContains: []string{"svc_svc_decoratorA "},
		},
		{
			name: "public tag",
			cfg: &di.Config{
				Tags: map[string]di.Tag{
					"public.tag": {
						ElementType: appPkg + ".Service",
						Public:      true,
					},
				},
				Services: map[string]di.Service{
					"svc": {
						Constructor: di.Constructor{Func: appPkg + ".NewServiceBase"},
					},
					"svc.decorator": {
						Constructor: di.Constructor{
							Func: appPkg + ".NewServiceDecoratorA",
							Args: []di.Argument{{Kind: di.ArgInner}},
						},
						Decorates:          "svc",
						DecorationPriority: 10,
						Tags:               []di.ServiceTag{{Name: "public.tag"}},
					},
				},
			},
			wantContains:     []string{"func (c *Container) getSvcDecorator()"},
			wantCountAtLeast: map[string]int{"getSvcDecorator()": 2},
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			out := generate(t, tt.cfg)
			for _, want := range tt.wantContains {
				if !strings.Contains(out, want) {
					t.Errorf("expected generated code containing %q, got:\n%s", want, out)
				}
			}
			for _, unwanted := range tt.wantNotContains {
				if strings.Contains(out, unwanted) {
					t.Errorf("expected generated code not to contain %q, got:\n%s", unwanted, out)
				}
			}
			for value, minCount := range tt.wantCountAtLeast {
				if got := strings.Count(out, value); got < minCount {
					t.Errorf("generated code contains %q %d times, want at least %d:\n%s", value, got, minCount, out)
				}
			}
		})
	}
}

func TestDeclaredServiceTypeAssignability(t *testing.T) {
	for _, tt := range []struct {
		name     string
		services map[string]di.Service
	}{
		{
			name: "constructor result",
			services: map[string]di.Service{
				"svc": {
					Type: "github.com/gendi-org/gendi/generator/testdata/app.Service",
					Constructor: di.Constructor{
						Func: "github.com/gendi-org/gendi/generator/testdata/app.NewServiceBaseConcrete",
					},
					Public: true,
				},
			},
		},
		{
			name: "decorator result",
			services: map[string]di.Service{
				"svc": {
					Type: "github.com/gendi-org/gendi/generator/testdata/app.Service",
					Constructor: di.Constructor{
						Func: "github.com/gendi-org/gendi/generator/testdata/app.NewServiceBaseConcrete",
					},
					Public: true,
				},
				"svc.decorator": {
					Constructor: di.Constructor{
						Func: "github.com/gendi-org/gendi/generator/testdata/app.NewServiceDecoratorAConcrete",
						Args: []di.Argument{
							{Kind: di.ArgInner},
						},
					},
					Decorates:          "svc",
					DecorationPriority: 10,
				},
			},
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			if err := generateErr(t, &di.Config{Services: tt.services}); err != nil {
				t.Fatalf("generate failed: %v", err)
			}
		})
	}
}

func TestGenericCodegen(t *testing.T) {
	const genericPkg = "github.com/gendi-org/gendi/generator/testdata/generics"

	tests := []struct {
		name         string
		serviceType  string
		constructor  di.Constructor
		wantContains []string
	}{
		{
			name: "named type argument",
			constructor: di.Constructor{
				Func: genericPkg + ".NewChan[" + genericPkg + ".Event]",
				Args: []di.Argument{{Kind: di.ArgLiteral, Literal: di.NewIntLiteral(100)}},
			},
			wantContains: []string{"NewChan[generics.Event]", "chan generics.Event"},
		},
		{
			name: "generic result type",
			constructor: di.Constructor{
				Func: genericPkg + ".NewPool[" + genericPkg + ".Message]",
				Args: []di.Argument{{Kind: di.ArgLiteral, Literal: di.NewIntLiteral(10)}},
			},
			wantContains: []string{"NewPool[generics.Message]", "*generics.Pool[generics.Message]"},
		},
		{
			name:        "explicit generic service type",
			serviceType: "*" + genericPkg + ".Pool[" + genericPkg + ".Message]",
			constructor: di.Constructor{
				Func: genericPkg + ".NewPool[" + genericPkg + ".Message]",
				Args: []di.Argument{{Kind: di.ArgLiteral, Literal: di.NewIntLiteral(10)}},
			},
			wantContains: []string{"*generics.Pool[generics.Message]"},
		},
		{
			name: "slice type argument",
			constructor: di.Constructor{
				Func: genericPkg + ".NewSlice[[]" + genericPkg + ".Event]",
				Args: []di.Argument{{Kind: di.ArgLiteral, Literal: di.NewIntLiteral(10)}},
			},
			wantContains: []string{"NewSlice[[]generics.Event]"},
		},
		{
			name: "multiple type arguments",
			constructor: di.Constructor{
				Func: genericPkg + ".NewMap[string, " + genericPkg + ".Event]",
			},
			wantContains: []string{"NewMap[string, generics.Event]", "map[string]generics.Event"},
		},
		{
			name: "channel type argument",
			constructor: di.Constructor{
				Func: genericPkg + ".NewChan[chan " + genericPkg + ".Event]",
				Args: []di.Argument{{Kind: di.ArgLiteral, Literal: di.NewIntLiteral(5)}},
			},
			wantContains: []string{"NewChan[chan generics.Event]", "chan chan generics.Event"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &di.Config{
				Services: map[string]di.Service{
					"service": {
						Type:        tt.serviceType,
						Constructor: tt.constructor,
						Public:      true,
					},
				},
			}

			assertCodegen(t, cfg, tt.wantContains, nil, nil)
		})
	}
}

func TestGenericValidationErrors(t *testing.T) {
	const genericPkg = "github.com/gendi-org/gendi/generator/testdata/generics"

	tests := []struct {
		name            string
		service         di.Service
		wantErrContains []string
	}{
		{
			name: "generic function without type arguments",
			service: di.Service{
				Constructor: di.Constructor{
					Func: genericPkg + ".NewChan",
					Args: []di.Argument{{Kind: di.ArgLiteral, Literal: di.NewIntLiteral(100)}},
				},
			},
			wantErrContains: []string{"generic function", "requires"},
		},
		{
			name: "generic type without type arguments",
			service: di.Service{
				Type: genericPkg + ".Pool",
				Constructor: di.Constructor{
					Func: genericPkg + ".NewPool[" + genericPkg + ".Message]",
					Args: []di.Argument{{Kind: di.ArgLiteral, Literal: di.NewIntLiteral(10)}},
				},
			},
			wantErrContains: []string{"generic type", "requires"},
		},
		{
			name: "non-generic type with type arguments",
			service: di.Service{
				Type: genericPkg + ".Event[string]",
				Constructor: di.Constructor{
					Func: genericPkg + ".NewChan[" + genericPkg + ".Event]",
					Args: []di.Argument{{Kind: di.ArgLiteral, Literal: di.NewIntLiteral(10)}},
				},
			},
			wantErrContains: []string{"not generic"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			service := tt.service
			service.Public = true
			cfg := &di.Config{Services: map[string]di.Service{"service": service}}

			assertCodegen(t, cfg, nil, nil, tt.wantErrContains)
		})
	}
}

func TestSpreadArguments(t *testing.T) {
	tests := []struct {
		name            string
		cfg             *di.Config
		wantContains    []string
		wantErrContains []string
	}{
		{
			name: "service reference",
			cfg: &di.Config{
				Services: map[string]di.Service{
					"handler.a": {
						Constructor: di.Constructor{
							Func: "github.com/gendi-org/gendi/generator/testdata/app.NewHandlerA",
						},
					},
					"handler.b": {
						Constructor: di.Constructor{
							Func: "github.com/gendi-org/gendi/generator/testdata/app.NewHandlerB",
						},
					},
					"all_handlers": {
						Constructor: di.Constructor{
							Func: "github.com/gendi-org/gendi/generator/testdata/app.GetAllHandlers",
							Args: []di.Argument{
								{Kind: di.ArgServiceRef, Value: "handler.a"},
								{Kind: di.ArgServiceRef, Value: "handler.a"},
							},
						},
						Shared: true,
					},
					"server": {
						Constructor: di.Constructor{
							Func: "github.com/gendi-org/gendi/generator/testdata/app.NewServer",
							Args: []di.Argument{
								{Kind: di.ArgSpread, Value: "@all_handlers"},
							},
						},
						Public: true,
					},
				},
			},
			wantContains: []string{"NewServer(", "...", "getAllHandlers()"},
		},
		{
			name: "tagged services",
			cfg: &di.Config{
				Tags: map[string]di.Tag{
					"handler": {
						ElementType: "github.com/gendi-org/gendi/generator/testdata/app.Handler",
					},
				},
				Services: map[string]di.Service{
					"handler.a": {
						Constructor: di.Constructor{
							Func: "github.com/gendi-org/gendi/generator/testdata/app.NewHandlerA",
						},
						Tags: []di.ServiceTag{{Name: "handler"}},
					},
					"handler.b": {
						Constructor: di.Constructor{
							Func: "github.com/gendi-org/gendi/generator/testdata/app.NewHandlerB",
						},
						Tags: []di.ServiceTag{{Name: "handler"}},
					},
					"server": {
						Constructor: di.Constructor{
							Func: "github.com/gendi-org/gendi/generator/testdata/app.NewServer",
							Args: []di.Argument{
								{Kind: di.ArgSpread, Value: "!tagged:handler"},
							},
						},
						Public: true,
					},
				},
			},
			wantContains: []string{"NewServer(", "...", "handler"},
		},
		{
			name: "after regular argument",
			cfg: &di.Config{
				Services: map[string]di.Service{
					"handler.a": {
						Constructor: di.Constructor{
							Func: "github.com/gendi-org/gendi/generator/testdata/app.NewHandlerA",
						},
					},
					"handler.b": {
						Constructor: di.Constructor{
							Func: "github.com/gendi-org/gendi/generator/testdata/app.NewHandlerB",
						},
					},
					"more_handlers": {
						Constructor: di.Constructor{
							Func: "github.com/gendi-org/gendi/generator/testdata/app.GetAllHandlers",
							Args: []di.Argument{
								{Kind: di.ArgServiceRef, Value: "handler.a"},
								{Kind: di.ArgServiceRef, Value: "handler.a"},
							},
						},
						Shared: true,
					},
					"server": {
						Constructor: di.Constructor{
							Func: "github.com/gendi-org/gendi/generator/testdata/app.NewPrefixedServer",
							Args: []di.Argument{
								{Kind: di.ArgLiteral, Literal: di.NewStringLiteral("api")},
								{Kind: di.ArgSpread, Value: "@more_handlers"},
							},
						},
						Public: true,
					},
				},
			},
			wantContains: []string{"NewPrefixedServer(", "...", "getHandlerA()", "getMoreHandlers()"},
		},
		{
			name: "mixed with positional variadic argument",
			cfg: &di.Config{
				Services: map[string]di.Service{
					"handler.a": {
						Constructor: di.Constructor{
							Func: "github.com/gendi-org/gendi/generator/testdata/app.NewHandlerA",
						},
					},
					"more_handlers": {
						Constructor: di.Constructor{
							Func: "github.com/gendi-org/gendi/generator/testdata/app.GetAllHandlers",
							Args: []di.Argument{
								{Kind: di.ArgServiceRef, Value: "handler.a"},
								{Kind: di.ArgServiceRef, Value: "handler.a"},
							},
						},
					},
					"server": {
						Constructor: di.Constructor{
							Func: "github.com/gendi-org/gendi/generator/testdata/app.NewServer",
							Args: []di.Argument{
								{Kind: di.ArgServiceRef, Value: "handler.a"},
								{Kind: di.ArgSpread, Value: "@more_handlers"},
							},
						},
						Public: true,
					},
				},
			},
			wantErrContains: []string{"positional variadic"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assertCodegen(t, tt.cfg, tt.wantContains, nil, tt.wantErrContains)
		})
	}
}

func TestSpecialArgumentDependencyErrorsPropagated(t *testing.T) {
	const appPkg = "github.com/gendi-org/gendi/generator/testdata/app"

	for _, tt := range []struct {
		name   string
		cfg    *di.Config
		getter string
	}{
		{
			name: "spread service reference",
			cfg: &di.Config{
				Services: map[string]di.Service{
					"handler.a": {
						Constructor: di.Constructor{Func: appPkg + ".NewHandlerA"},
					},
					"all_handlers": {
						Constructor: di.Constructor{
							Func: appPkg + ".GetAllHandlersWithError",
							Args: []di.Argument{
								{Kind: di.ArgServiceRef, Value: "handler.a"},
								{Kind: di.ArgServiceRef, Value: "handler.a"},
							},
						},
					},
					"server": {
						Constructor: di.Constructor{
							Func: appPkg + ".NewServer",
							Args: []di.Argument{{Kind: di.ArgSpread, Value: "@all_handlers"}},
						},
						Public: true,
					},
				},
			},
			getter: "getAllHandlers",
		},
		{
			name: "service field access",
			cfg: &di.Config{
				Services: map[string]di.Service{
					"config": {
						Constructor: di.Constructor{Func: appPkg + ".LoadConfigWithError"},
					},
					"server": {
						Constructor: di.Constructor{
							Func: appPkg + ".NewLogger",
							Args: []di.Argument{{Kind: di.ArgFieldAccessService, Value: "config.Host"}},
						},
						Public: true,
					},
				},
			},
			getter: "getConfig",
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			getterCall := "c." + tt.getter + "()"
			assertCodegen(
				t,
				tt.cfg,
				[]string{", err := " + getterCall},
				[]string{", _ := " + getterCall},
				nil,
			)
		})
	}
}

func TestGoRefArguments(t *testing.T) {
	const appPkg = "github.com/gendi-org/gendi/generator/testdata/app"

	tests := []struct {
		name            string
		constructor     string
		value           string
		wantContains    []string
		wantErrContains []string
	}{
		{
			name:         "standard library variable",
			constructor:  appPkg + ".NewWriter",
			value:        "os.Stdout",
			wantContains: []string{"os.Stdout", "NewWriter(os.Stdout)"},
		},
		{
			name:         "package-level variable",
			constructor:  appPkg + ".NewLogger",
			value:        appPkg + ".DefaultPrefix",
			wantContains: []string{"app.DefaultPrefix", "NewLogger(app.DefaultPrefix)"},
		},
		{
			name:            "type mismatch",
			constructor:     appPkg + ".NewLogger",
			value:           "os.Stdout",
			wantErrContains: []string{"not assignable"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &di.Config{
				Services: map[string]di.Service{
					"service": {
						Constructor: di.Constructor{
							Func: tt.constructor,
							Args: []di.Argument{{Kind: di.ArgGoRef, Value: tt.value}},
						},
						Public: true,
					},
				},
			}

			assertCodegen(t, cfg, tt.wantContains, nil, tt.wantErrContains)
		})
	}
}

func TestFieldAccessArguments(t *testing.T) {
	const appPkg = "github.com/gendi-org/gendi/generator/testdata/app"

	tests := []struct {
		name            string
		constructor     string
		args            []di.Argument
		withConfig      bool
		wantContains    []string
		wantErrContains []string
	}{
		{
			name:        "service fields",
			constructor: appPkg + ".NewServerWithAddr",
			args: []di.Argument{
				{Kind: di.ArgFieldAccessService, Value: "config.Host"},
				{Kind: di.ArgFieldAccessService, Value: "config.Port"},
			},
			withConfig:   true,
			wantContains: []string{".Host", ".Port"},
		},
		{
			name:         "nested service field",
			constructor:  appPkg + ".NewLogger",
			args:         []di.Argument{{Kind: di.ArgFieldAccessService, Value: "config.Database.DSN"}},
			withConfig:   true,
			wantContains: []string{".Database.DSN"},
		},
		{
			name:         "Go reference field",
			constructor:  appPkg + ".NewTimer",
			args:         []di.Argument{{Kind: di.ArgFieldAccessGo, Value: "net/http.DefaultClient.Timeout"}},
			wantContains: []string{"http.DefaultClient.Timeout"},
		},
		{
			name:            "type mismatch",
			constructor:     appPkg + ".NewTimer",
			args:            []di.Argument{{Kind: di.ArgFieldAccessService, Value: "config.Host"}},
			withConfig:      true,
			wantErrContains: []string{"not assignable"},
		},
		{
			name:            "unknown field",
			constructor:     appPkg + ".NewLogger",
			args:            []di.Argument{{Kind: di.ArgFieldAccessService, Value: "config.NonExistentField"}},
			withConfig:      true,
			wantErrContains: []string{"not found"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			services := map[string]di.Service{
				"consumer": {
					Constructor: di.Constructor{Func: tt.constructor, Args: tt.args},
					Public:      true,
				},
			}
			if tt.withConfig {
				services["config"] = di.Service{
					Constructor: di.Constructor{Func: appPkg + ".LoadConfig"},
				}
			}
			cfg := &di.Config{Services: services}

			assertCodegen(t, cfg, tt.wantContains, nil, tt.wantErrContains)
		})
	}
}

func TestDecoratorOnAlias(t *testing.T) {
	cfg := &di.Config{
		Services: map[string]di.Service{
			"svc": {
				Constructor: di.Constructor{
					Func: "github.com/gendi-org/gendi/generator/testdata/app.NewServiceBase",
				},
				Public: true,
			},
			"svc.alias": {
				Alias:  "svc",
				Public: true,
			},
			"svc.decorator": {
				Constructor: di.Constructor{
					Func: "github.com/gendi-org/gendi/generator/testdata/app.NewServiceDecoratorA",
					Args: []di.Argument{
						{Kind: di.ArgInner},
					},
				},
				Decorates:          "svc.alias",
				DecorationPriority: 10,
			},
		},
	}

	out := generate(t, cfg)

	// Verify that both base and decorator constructors are used in generated code
	if !strings.Contains(out, "NewServiceBase(") {
		t.Fatalf("expected generated code to build underlying service for decorated alias")
	}
	if !strings.Contains(out, "NewServiceDecoratorA(") {
		t.Fatalf("expected generated code to build decorator")
	}
}

func TestDecoratorSpreadInnerRejected(t *testing.T) {
	cfg := &di.Config{
		Services: map[string]di.Service{
			"svc": {
				Constructor: di.Constructor{
					Func: "github.com/gendi-org/gendi/generator/testdata/app.NewServiceBase",
				},
				Public: true,
			},
			"svc.decorator": {
				Constructor: di.Constructor{
					Func: "github.com/gendi-org/gendi/generator/testdata/app.NewServiceDecoratorA",
					Args: []di.Argument{
						{Kind: di.ArgSpread, Value: "@.inner"},
					},
				},
				Decorates: "svc",
			},
		},
	}

	err := generateErr(t, cfg)
	if err == nil {
		t.Fatal("expected unsupported !spread:@.inner error")
	}
	if !strings.Contains(err.Error(), "!spread:@.inner is not supported") {
		t.Fatalf("expected unsupported spread-inner error, got: %v", err)
	}
}

func TestDecoratorStorage(t *testing.T) {
	const appPkg = "github.com/gendi-org/gendi/generator/testdata/app"

	for _, tt := range []struct {
		name            string
		baseShared      bool
		decoratorShared bool
		decoratorPublic bool
		wantContains    []string
		wantNotContains []string
		wantExactCounts map[string]int
	}{
		{
			name:            "shared base and shared decorator",
			baseShared:      true,
			decoratorShared: true,
			decoratorPublic: true,
			wantContains: []string{
				"svc_svc_decorator ",
				"if c.svc_svc_decorator != nil",
				"return c.getSvcDecorator()",
			},
			wantNotContains: []string{"svc_svc "},
			wantExactCounts: map[string]int{"if c.svc_svc_decorator != nil": 1},
		},
		{
			name:            "shared base and non-shared decorator",
			baseShared:      true,
			wantContains:    []string{"svc_svc_decorator_inner "},
			wantNotContains: []string{"svc_svc_decorator "},
		},
		{
			name:            "non-shared base and shared decorator",
			decoratorShared: true,
			wantContains:    []string{"svc_svc_decorator "},
			wantNotContains: []string{"svc_svc_decorator_inner "},
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &di.Config{
				Services: map[string]di.Service{
					"svc": {
						Constructor: di.Constructor{Func: appPkg + ".NewServiceBase"},
						Public:      true,
						Shared:      tt.baseShared,
					},
					"svc.decorator": {
						Constructor: di.Constructor{
							Func: appPkg + ".NewServiceDecoratorA",
							Args: []di.Argument{{Kind: di.ArgInner}},
						},
						Decorates: "svc",
						Public:    tt.decoratorPublic,
						Shared:    tt.decoratorShared,
					},
				},
			}

			out := generate(t, cfg)
			for _, want := range tt.wantContains {
				if !strings.Contains(out, want) {
					t.Errorf("expected generated code containing %q, got:\n%s", want, out)
				}
			}
			for _, unwanted := range tt.wantNotContains {
				if strings.Contains(out, unwanted) {
					t.Errorf("expected generated code not to contain %q, got:\n%s", unwanted, out)
				}
			}
			for value, wantCount := range tt.wantExactCounts {
				if got := strings.Count(out, value); got != wantCount {
					t.Errorf("generated code contains %q %d times, want %d:\n%s", value, got, wantCount, out)
				}
			}
		})
	}
}

func TestMustGettersGenerated(t *testing.T) {
	cfg := &di.Config{
		Services: map[string]di.Service{
			"foo": {
				Constructor: di.Constructor{
					Func: "github.com/gendi-org/gendi/generator/testdata/app.NewA",
				},
				Public: true,
			},
			"bar": {
				Constructor: di.Constructor{
					Func: "github.com/gendi-org/gendi/generator/testdata/app.NewC",
				},
				Public: true,
			},
			"internal": {
				Constructor: di.Constructor{
					Func: "github.com/gendi-org/gendi/generator/testdata/app.NewServiceBase",
				},
				Public: false, // not public
			},
		},
	}

	out := generate(t, cfg)

	// Check that onMustCallFailed field is present
	if !strings.Contains(out, "onMustCallFailed func(serviceName string, err error)") {
		t.Errorf("expected onMustCallFailed field in Container struct")
	}

	// Check that WithContainerErrorHandler is generated
	if !strings.Contains(out, "func WithContainerErrorHandler(handler func(serviceName string, err error)) ContainerOption") {
		t.Errorf("expected WithContainerErrorHandler function")
	}

	// Check that NewContainer accepts options
	if !strings.Contains(out, "func NewContainer(params parameters.Provider, opts ...ContainerOption)") {
		t.Errorf("expected NewContainer to accept options")
	}

	// Check that Must* methods are generated for public services
	if !strings.Contains(out, "func (c *Container) MustFoo()") {
		t.Errorf("expected MustFoo method")
	}
	if !strings.Contains(out, "func (c *Container) MustBar()") {
		t.Errorf("expected MustBar method")
	}

	// Check that Must* methods are NOT generated for private services
	if strings.Contains(out, "func (c *Container) MustInternal()") {
		t.Errorf("unexpected MustInternal method for private service")
	}

	// Check that Must* methods call onMustCallFailed callback
	if !strings.Contains(out, "c.onMustCallFailed(") {
		t.Errorf("expected Must methods to call onMustCallFailed callback")
	}

	// Check that Must* methods panic after callback
	if !strings.Contains(out, `panic(err)`) {
		t.Errorf("expected Must methods to panic after onMustCallFailed")
	}
}

func TestFmtImport(t *testing.T) {
	for _, tt := range []struct {
		name       string
		cfg        *di.Config
		wantImport bool
	}{
		{
			name: "omitted without error handling",
			cfg: &di.Config{
				Services: map[string]di.Service{
					"a": {
						Constructor: di.Constructor{
							Func: "github.com/gendi-org/gendi/generator/testdata/app.NewA",
						},
						Public: true,
					},
				},
			},
		},
		{
			name: "included with error handling",
			cfg: &di.Config{
				Services: map[string]di.Service{
					"a": {
						Constructor: di.Constructor{
							Func: "github.com/gendi-org/gendi/generator/testdata/app.NewA",
						},
					},
					"b": {
						Constructor: di.Constructor{
							Func: "github.com/gendi-org/gendi/generator/testdata/app.NewB",
							Args: []di.Argument{
								{Kind: di.ArgServiceRef, Value: "a"},
							},
						},
						Public: true,
					},
				},
			},
			wantImport: true,
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			out := generate(t, tt.cfg)
			if got := strings.Contains(out, "\"fmt\""); got != tt.wantImport {
				t.Fatalf("fmt import present = %t, want %t:\n%s", got, tt.wantImport, out)
			}
			if !tt.wantImport && strings.Contains(out, "fmt.") {
				t.Fatalf("expected no fmt usage in generated code:\n%s", out)
			}
		})
	}
}

func TestServiceRefTypeMismatchFailsGeneration(t *testing.T) {
	cfg := &di.Config{
		Services: map[string]di.Service{
			"c": {
				Constructor: di.Constructor{
					Func: "github.com/gendi-org/gendi/generator/testdata/app.NewC",
				},
			},
			"b": {
				Constructor: di.Constructor{
					Func: "github.com/gendi-org/gendi/generator/testdata/app.NewB",
					Args: []di.Argument{
						{Kind: di.ArgServiceRef, Value: "c"},
					},
				},
				Public: true,
			},
		},
	}

	err := generateErr(t, cfg)
	if err == nil || !strings.Contains(err.Error(), "not assignable") {
		t.Fatalf("expected type mismatch error, got %v", err)
	}
}

func TestUserPackageNamedParameters(t *testing.T) {
	// The gendi parameters package is always imported unaliased, so a user
	// package with the same name must get a distinct alias even when the
	// config declares no parameters.
	cfg := &di.Config{
		Services: map[string]di.Service{
			"store": {
				Constructor: di.Constructor{
					Func: "github.com/gendi-org/gendi/generator/testdata/parameters.NewStore",
				},
				Public: true,
			},
		},
	}

	out := generate(t, cfg)
	if !strings.Contains(out, "parameters2 \"github.com/gendi-org/gendi/generator/testdata/parameters\"") {
		t.Fatalf("expected user parameters package to get a distinct alias:\n%s", out)
	}
}

func TestEmitTwiceOnSameConfig(t *testing.T) {
	// pipeline.Build must not mutate the caller's config: a second Emit on
	// the same config used to fail on the .inner services added by the first.
	cfg := &di.Config{
		Services: map[string]di.Service{
			"svc": {
				Constructor: di.Constructor{
					Func: "github.com/gendi-org/gendi/generator/testdata/app.NewServiceBase",
				},
				Public: true,
				Shared: true,
			},
			"svc.decorator": {
				Constructor: di.Constructor{
					Func: "github.com/gendi-org/gendi/generator/testdata/app.NewServiceDecoratorA",
					Args: []di.Argument{
						{Kind: di.ArgInner},
					},
				},
				Decorates: "svc",
				Shared:    true,
			},
		},
	}

	first := generate(t, cfg)
	second := generate(t, cfg)
	if first != second {
		t.Fatalf("expected identical output from repeated Emit")
	}
	if _, ok := cfg.Services["svc.decorator.inner"]; ok {
		t.Fatalf("expected caller's config to stay unmodified")
	}
}

func TestBuildTagsReachTypeResolver(t *testing.T) {
	cfg := &di.Config{
		Services: map[string]di.Service{
			"svc": {
				Constructor: di.Constructor{
					// NewService lives in a file guarded by //go:build taggedsvc,
					// so type resolution only succeeds when the build tags are
					// passed through to the package loader.
					Func: "github.com/gendi-org/gendi/generator/testdata/tagged.NewService",
				},
				Public: true,
			},
		},
	}

	opts := testEmitOptions(t)
	opts.BuildTags = "taggedsvc"

	code, err := pipeline.Emit(cfg, opts)
	if err != nil {
		t.Fatalf("emit failed: %v", err)
	}

	out := string(code)
	if !strings.Contains(out, "//go:build taggedsvc\n") {
		t.Fatalf("expected //go:build header in output:\n%s", out)
	}
	if strings.Contains(out, "// +build") {
		t.Fatalf("unexpected legacy // +build line in output:\n%s", out)
	}
}

func TestMapArgumentCodegen(t *testing.T) {
	const appPkg = "github.com/gendi-org/gendi/generator/testdata/app"

	mapArg := func(entries ...di.ArgEntry) di.Argument {
		return di.Argument{Kind: di.ArgMap, Entries: entries}
	}

	for _, tt := range []struct {
		name            string
		cfg             *di.Config
		wantContains    []string
		wantNotContains []string
	}{
		{
			name: "two entries pointing at the same service get distinct vars",
			cfg: &di.Config{
				Services: map[string]di.Service{
					"handler": {Constructor: di.Constructor{Func: appPkg + ".NewHandlerA"}},
					"router": {
						Constructor: di.Constructor{
							Func: appPkg + ".NewRouter",
							// Keys are the same length on purpose: gofmt column-aligns
							// key:value pairs of differing key length in a composite
							// literal, which would break the literal "key: var"
							// substring checks below.
							Args: []di.Argument{mapArg(
								di.ArgEntry{Key: di.NewStringLiteral("/a"), Value: di.Argument{Kind: di.ArgServiceRef, Value: "handler"}},
								di.ArgEntry{Key: di.NewStringLiteral("/b"), Value: di.Argument{Kind: di.ArgServiceRef, Value: "handler"}},
							)},
						},
						Public: true,
					},
				},
			},
			wantContains: []string{
				"arg0_e1_handler",
				"arg0_e2_handler",
				`"/a": arg0_e1_handler`,
				`"/b": arg0_e2_handler`,
			},
		},
		{
			// The entry error snippet exists only on the error-returning path:
			// NewRouter returns no error, so its generated code discards it
			// (`arg0_e1_handler, _ := c.getHandler()`) and no snippet is emitted.
			// This case needs an error-returning constructor, so it uses
			// NewRouterWithError instead.
			name: "entry error names the key",
			cfg: &di.Config{
				Services: map[string]di.Service{
					"handler": {Constructor: di.Constructor{Func: appPkg + ".NewHandlerA"}},
					"router": {
						Constructor: di.Constructor{
							Func: appPkg + ".NewRouterWithError",
							Args: []di.Argument{mapArg(
								di.ArgEntry{Key: di.NewStringLiteral("/a"), Value: di.Argument{Kind: di.ArgServiceRef, Value: "handler"}},
							)},
						},
						Public: true,
					},
				},
			},
			wantContains: []string{
				`arg[%d] key %s: %w`,
				`"router", 0, "\"/a\""`,
			},
		},
		{
			name: "empty mapping emits an empty literal",
			cfg: &di.Config{
				Services: map[string]di.Service{
					"router": {
						Constructor: di.Constructor{Func: appPkg + ".NewRouter", Args: []di.Argument{mapArg()}},
						Public:      true,
					},
				},
			},
			wantContains: []string{"{}"},
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			assertCodegen(t, tt.cfg, tt.wantContains, tt.wantNotContains, nil)
		})
	}
}

func TestMapArgumentBuildsDependencyEdge(t *testing.T) {
	const appPkg = "github.com/gendi-org/gendi/generator/testdata/app"

	cfg := &di.Config{
		Services: map[string]di.Service{
			"handler": {Constructor: di.Constructor{Func: appPkg + ".NewHandlerA"}},
			"router": {
				Constructor: di.Constructor{
					Func: appPkg + ".NewRouter",
					Args: []di.Argument{{
						Kind: di.ArgMap,
						Entries: []di.ArgEntry{{
							Key:   di.NewStringLiteral("/"),
							Value: di.Argument{Kind: di.ArgServiceRef, Value: "handler"},
						}},
					}},
				},
				Public: true,
			},
		},
	}

	// The handler is reachable only through the map entry: if the walker misses
	// it, unreachable pruning drops the service and generation fails.
	assertCodegen(t, cfg, []string{"getHandler"}, nil, nil)
}

// TestMapArgumentEntryErrorHandling guards against a build function that
// compiles a reference to an undeclared "zero" fallback. NewLabeledRouter
// itself never returns an error, so the build function only needs error
// handling because one of its two map arguments carries an error source:
// here, a %param% nested inside the second one. If that error source isn't
// detected, the generator skips the "var zero" declaration but the parameter
// builder still emits a reference to it, which fails to compile. The same fix
// must also give the service ref nested in the first map argument a real
// error check instead of discarding it with `_ :=`, since both arguments now
// share the same returnsErr decision.
func TestMapArgumentEntryErrorHandling(t *testing.T) {
	const appPkg = "github.com/gendi-org/gendi/generator/testdata/app"

	cfg := &di.Config{
		Parameters: map[string]di.Parameter{
			"label": {Value: di.NewStringLiteral("europe")},
		},
		Services: map[string]di.Service{
			"handler": {Constructor: di.Constructor{Func: appPkg + ".NewHandlerA"}},
			"router": {
				Constructor: di.Constructor{
					Func: appPkg + ".NewLabeledRouter",
					Args: []di.Argument{
						{
							Kind: di.ArgMap,
							Entries: []di.ArgEntry{{
								Key:   di.NewStringLiteral("/"),
								Value: di.Argument{Kind: di.ArgServiceRef, Value: "handler"},
							}},
						},
						{
							Kind: di.ArgMap,
							Entries: []di.ArgEntry{{
								Key:   di.NewStringLiteral("eu"),
								Value: di.Argument{Kind: di.ArgParam, Value: "label"},
							}},
						},
					},
				},
				Public: true,
			},
		},
	}

	assertCodegen(t, cfg, []string{
		"var zero",
		"if err != nil",
		"arg0_e1_handler, err := c.getHandler()",
	}, []string{
		"arg0_e1_handler, _ := c.getHandler()",
	}, nil)
}
