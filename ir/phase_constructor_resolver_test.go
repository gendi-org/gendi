package ir

import (
	"fmt"
	"go/token"
	"go/types"
	"strings"
	"testing"

	di "github.com/gendi-org/gendi"
)

// funcResolver resolves constructor funcs from a fixed map.
type funcResolver struct {
	funcs map[string]*types.Func // "pkg.Name" → func
}

func (r *funcResolver) LookupType(typeStr string) (types.Type, error) {
	return nil, fmt.Errorf("not supported")
}

func (r *funcResolver) LookupFunc(pkgPath, name string) (*types.Func, error) {
	if fn, ok := r.funcs[pkgPath+"."+name]; ok {
		return fn, nil
	}
	return nil, fmt.Errorf("func %s not found in %s", name, pkgPath)
}

func (r *funcResolver) LookupMethod(recv types.Type, name string) (*types.Func, error) {
	return nil, fmt.Errorf("not supported")
}

func (r *funcResolver) InstantiateFunc(fn *types.Func, typeArgs []string) (*types.Signature, []types.Type, error) {
	return nil, nil, fmt.Errorf("not supported")
}

func (r *funcResolver) LookupVar(pkgPath, name string) (types.Object, error) {
	return nil, fmt.Errorf("not supported")
}

func makeFunc(pkg *types.Package, name string, params []types.Type, result types.Type) *types.Func {
	vars := make([]*types.Var, len(params))
	for i, p := range params {
		vars[i] = types.NewParam(token.NoPos, pkg, fmt.Sprintf("p%d", i), p)
	}
	sig := types.NewSignatureType(nil, nil, nil,
		types.NewTuple(vars...),
		types.NewTuple(types.NewParam(token.NoPos, pkg, "", result)),
		false)
	return types.NewFunc(token.NoPos, pkg, name, sig)
}

// TestFieldAccessResolvesTargetFirst covers !field:@svc.Field where the
// consuming service's ID sorts before the target's, so the target's type is
// not yet resolved when the argument is processed.
func TestFieldAccessResolvesTargetFirst(t *testing.T) {
	pkg := types.NewPackage("test/app", "app")
	stringType := types.Typ[types.String]

	cfgStruct := makeStruct("app", "Host", stringType)
	cfgNamed := types.NewNamed(types.NewTypeName(token.NoPos, pkg, "Config", nil), cfgStruct, nil)
	ptrCfg := types.NewPointer(cfgNamed)

	resolver := &funcResolver{funcs: map[string]*types.Func{
		"test/app.NewApp":    makeFunc(pkg, "NewApp", []types.Type{stringType}, stringType),
		"test/app.NewConfig": makeFunc(pkg, "NewConfig", nil, ptrCfg),
	}}

	// "app" sorts before "zconfig", so it is resolved first.
	cfg := di.NewConfig()
	cfg.Services["app"] = di.Service{
		Constructor: di.Constructor{
			Func: "test/app.NewApp",
			Args: []di.Argument{{Kind: di.ArgFieldAccessService, Value: "zconfig.Host"}},
		},
	}
	cfg.Services["zconfig"] = di.Service{
		Constructor: di.Constructor{Func: "test/app.NewConfig"},
	}

	container := NewContainer()
	container.Services["app"] = &Service{ID: "app"}
	container.Services["zconfig"] = &Service{ID: "zconfig"}

	phase := &constructorResolverPhase{
		typeResolver: resolver,
		argResolver:  &argResolver{typeResolver: resolver},
	}
	if err := phase.Apply(cfg, container); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	app := container.Services["app"]
	if app.Constructor == nil || len(app.Constructor.Args) != 1 {
		t.Fatalf("app constructor not resolved: %+v", app.Constructor)
	}
	fa := app.Constructor.Args[0].FieldAccess
	if fa == nil || fa.Service == nil || fa.Service.ID != "zconfig" {
		t.Fatalf("expected field access on zconfig, got %+v", fa)
	}
	if fa.ResultType != stringType {
		t.Fatalf("expected string result type, got %v", fa.ResultType)
	}
}

// TestFieldAccessSelfReferenceIsCycleError covers !field:@self.Field, which
// must be reported as a circular reference instead of panicking or recursing.
func TestFieldAccessSelfReferenceIsCycleError(t *testing.T) {
	pkg := types.NewPackage("test/app", "app")
	stringType := types.Typ[types.String]

	resolver := &funcResolver{funcs: map[string]*types.Func{
		"test/app.NewApp": makeFunc(pkg, "NewApp", []types.Type{stringType}, stringType),
	}}

	cfg := di.NewConfig()
	cfg.Services["app"] = di.Service{
		Constructor: di.Constructor{
			Func: "test/app.NewApp",
			Args: []di.Argument{{Kind: di.ArgFieldAccessService, Value: "app.Host"}},
		},
	}

	container := NewContainer()
	container.Services["app"] = &Service{ID: "app"}

	phase := &constructorResolverPhase{
		typeResolver: resolver,
		argResolver:  &argResolver{typeResolver: resolver},
	}
	err := phase.Apply(cfg, container)
	if err == nil || !strings.Contains(err.Error(), "circular") {
		t.Fatalf("expected circular reference error, got %v", err)
	}
}

// TestSpreadIntoNonVariadicRejected covers !spread: into a plain slice
// parameter, which would generate invalid Go (nums... into []int).
func TestSpreadIntoNonVariadicRejected(t *testing.T) {
	pkg := types.NewPackage("test/app", "app")
	intType := types.Typ[types.Int]
	intSlice := types.NewSlice(intType)

	resolver := &funcResolver{funcs: map[string]*types.Func{
		"test/app.NewList": makeFunc(pkg, "NewList", []types.Type{intSlice}, types.Typ[types.String]),
		"test/app.NewNums": makeFunc(pkg, "NewNums", nil, intSlice),
	}}

	cfg := di.NewConfig()
	cfg.Services["list"] = di.Service{
		Constructor: di.Constructor{
			Func: "test/app.NewList",
			Args: []di.Argument{{Kind: di.ArgSpread, Value: "@nums"}},
		},
	}
	cfg.Services["nums"] = di.Service{
		Constructor: di.Constructor{Func: "test/app.NewNums"},
	}

	container := NewContainer()
	container.Services["list"] = &Service{ID: "list"}
	container.Services["nums"] = &Service{ID: "nums"}

	phase := &constructorResolverPhase{
		typeResolver: resolver,
		argResolver:  &argResolver{typeResolver: resolver},
	}
	err := phase.Apply(cfg, container)
	if err == nil || !strings.Contains(err.Error(), "variadic") {
		t.Fatalf("expected variadic-only spread error, got %v", err)
	}
}

// TestSpreadMixedWithPositionalVariadicRejected covers mixing positional
// variadic values with a trailing spread, which is invalid Go.
func TestSpreadMixedWithPositionalVariadicRejected(t *testing.T) {
	pkg := types.NewPackage("test/app", "app")
	intType := types.Typ[types.Int]
	intSlice := types.NewSlice(intType)
	variadicSig := types.NewSignatureType(nil, nil, nil,
		types.NewTuple(types.NewParam(token.NoPos, pkg, "p0", intSlice)),
		types.NewTuple(types.NewParam(token.NoPos, pkg, "", types.Typ[types.String])),
		true)
	variadicFunc := types.NewFunc(token.NoPos, pkg, "NewSum", variadicSig)

	resolver := &funcResolver{funcs: map[string]*types.Func{
		"test/app.NewSum":  variadicFunc,
		"test/app.NewNums": makeFunc(pkg, "NewNums", nil, intSlice),
	}}

	cfg := di.NewConfig()
	cfg.Services["sum"] = di.Service{
		Constructor: di.Constructor{
			Func: "test/app.NewSum",
			Args: []di.Argument{
				{Kind: di.ArgLiteral, Literal: di.NewIntLiteral(1)},
				{Kind: di.ArgSpread, Value: "@nums"},
			},
		},
	}
	cfg.Services["nums"] = di.Service{
		Constructor: di.Constructor{Func: "test/app.NewNums"},
	}

	container := NewContainer()
	container.Services["sum"] = &Service{ID: "sum"}
	container.Services["nums"] = &Service{ID: "nums"}

	phase := &constructorResolverPhase{
		typeResolver: resolver,
		argResolver:  &argResolver{typeResolver: resolver},
	}
	err := phase.Apply(cfg, container)
	if err == nil || !strings.Contains(err.Error(), "spread") {
		t.Fatalf("expected mixed positional/spread error, got %v", err)
	}
}

// TestEmptyInterfaceResultRejected covers constructors returning the empty
// interface: such a service type carries no static information, so the
// container could neither validate nor type its getter.
func TestEmptyInterfaceResultRejected(t *testing.T) {
	pkg := types.NewPackage("test/app", "app")
	emptyIface := types.NewInterfaceType(nil, nil).Complete()
	namedEmpty := types.NewNamed(types.NewTypeName(token.NoPos, pkg, "Anything", nil), emptyIface, nil)
	handleSig := types.NewSignatureType(nil, nil, nil, nil, nil, false)
	handlerIface := types.NewInterfaceType(
		[]*types.Func{types.NewFunc(token.NoPos, pkg, "Handle", handleSig)}, nil).Complete()
	namedHandler := types.NewNamed(types.NewTypeName(token.NoPos, pkg, "Handler", nil), handlerIface, nil)

	tests := []struct {
		name       string
		result     types.Type
		wantReject bool
	}{
		{name: "any", result: emptyIface, wantReject: true},
		{name: "named empty interface", result: namedEmpty, wantReject: true},
		{name: "non-empty interface", result: namedHandler},
		{name: "concrete type", result: types.Typ[types.String]},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resolver := &funcResolver{funcs: map[string]*types.Func{
				"test/app.New": makeFunc(pkg, "New", nil, tt.result),
			}}

			cfg := di.NewConfig()
			cfg.Services["svc"] = di.Service{
				Constructor: di.Constructor{Func: "test/app.New"},
			}

			container := NewContainer()
			container.Services["svc"] = &Service{ID: "svc"}

			phase := &constructorResolverPhase{
				typeResolver: resolver,
				argResolver:  &argResolver{typeResolver: resolver},
			}
			err := phase.Apply(cfg, container)
			if !tt.wantReject {
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), "empty interface") {
				t.Fatalf("expected empty interface error, got %v", err)
			}
		})
	}
}
