package ir

import (
	"fmt"
	"go/token"
	"go/types"
	"math"
	"strings"
	"testing"

	di "github.com/gendi-org/gendi"
)

// testResolver implements TypeResolver for arg_resolver tests.
type testResolver struct {
	vars map[string]types.Object // "pkg.Name" → Object
}

func (r *testResolver) LookupType(typeStr string) (types.Type, error) {
	return types.Typ[types.String], nil
}
func (r *testResolver) LookupFunc(pkgPath, name string) (*types.Func, error) {
	return nil, fmt.Errorf("not found")
}
func (r *testResolver) LookupMethod(recv types.Type, name string) (*types.Func, error) {
	return nil, fmt.Errorf("not found")
}
func (r *testResolver) InstantiateFunc(fn *types.Func, typeArgs []string) (*types.Signature, []types.Type, error) {
	return nil, nil, fmt.Errorf("not supported")
}
func (r *testResolver) LookupVar(pkgPath, name string) (types.Object, error) {
	key := pkgPath + "." + name
	if obj, ok := r.vars[key]; ok {
		return obj, nil
	}
	return nil, fmt.Errorf("symbol %s not found in %s", name, pkgPath)
}

// noResolve is a resolveSvc callback for tests whose services already carry
// resolved types.
func noResolve(string) error { return nil }

// makeStruct creates a named struct type with the given fields.
// Each field is (name string, type types.Type, exported bool).
func makeStruct(pkgName string, fields ...any) *types.Struct {
	pkg := types.NewPackage("test/"+pkgName, pkgName)
	var flds []*types.Var
	for i := 0; i < len(fields); i += 2 {
		name := fields[i].(string)
		typ := fields[i+1].(types.Type)
		flds = append(flds, types.NewField(token.NoPos, pkg, name, typ, false))
	}
	return types.NewStruct(flds, nil)
}

// makePkgVar creates a package-level *types.Var for use with the testResolver.
func makePkgVar(pkgPath, name string, typ types.Type) types.Object {
	pkg := types.NewPackage(pkgPath, pkgPath[strings.LastIndex(pkgPath, "/")+1:])
	return types.NewVar(token.NoPos, pkg, name, typ)
}

func TestTaggedElementTypeAssignable(t *testing.T) {
	container := NewContainer()
	r := &argResolver{}
	arg := di.Argument{Kind: di.ArgTagged, Value: "tag.test"}

	if _, err := r.resolve(container, noResolve, "svc.one", 0, arg, types.NewSlice(types.Typ[types.Int])); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	emptyIface := types.NewInterfaceType(nil, nil)
	emptyIface.Complete()
	if _, err := r.resolve(container, noResolve, "svc.two", 0, arg, types.NewSlice(emptyIface)); err != nil {
		t.Fatalf("expected assignable element type, got %v", err)
	}
}

func TestTaggedElementTypeNotAssignable(t *testing.T) {
	container := NewContainer()
	r := &argResolver{}
	arg := di.Argument{Kind: di.ArgTagged, Value: "tag.test"}

	emptyIface := types.NewInterfaceType(nil, nil)
	emptyIface.Complete()
	if _, err := r.resolve(container, noResolve, "svc.one", 0, arg, types.NewSlice(emptyIface)); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	_, err := r.resolve(container, noResolve, "svc.two", 0, arg, types.NewSlice(types.Typ[types.Int]))
	if err == nil || !strings.Contains(err.Error(), "not assignable") {
		t.Fatalf("expected element type mismatch error, got %v", err)
	}
}

func TestResolve(t *testing.T) {
	stringType := types.Typ[types.String]
	intType := types.Typ[types.Int]

	resolver := &testResolver{
		vars: map[string]types.Object{
			"mypkg.MyVar": makePkgVar("mypkg", "MyVar", stringType),
		},
	}

	container := NewContainer()
	container.Services["dep"] = &Service{ID: "dep", Type: stringType}
	container.Parameters["db.host"] = &Parameter{Name: "db.host"}

	// Also add a tagged service for spread test
	container.Services["handler_svc"] = &Service{
		ID:   "handler_svc",
		Type: types.NewSlice(intType),
	}

	tests := []struct {
		name      string
		arg       di.Argument
		paramType types.Type
		wantKind  ArgumentKind
		wantErr   string
	}{
		{
			name:      "service_ref_ok",
			arg:       di.Argument{Kind: di.ArgServiceRef, Value: "dep"},
			paramType: stringType,
			wantKind:  ServiceRefArg,
		},
		{
			name:      "service_ref_unknown",
			arg:       di.Argument{Kind: di.ArgServiceRef, Value: "missing"},
			paramType: stringType,
			wantErr:   "unknown service",
		},
		{
			name:      "inner_error",
			arg:       di.Argument{Kind: di.ArgInner, Value: "@.inner"},
			paramType: stringType,
			wantErr:   "DecoratorPass",
		},
		{
			name:      "param_found",
			arg:       di.Argument{Kind: di.ArgParam, Value: "db.host"},
			paramType: stringType,
			wantKind:  ParamRefArg,
		},
		{
			name:      "param_not_found_runtime",
			arg:       di.Argument{Kind: di.ArgParam, Value: "unknown.param"},
			paramType: intType,
			wantKind:  ParamRefArg,
		},
		{
			name:      "tagged_not_collection",
			arg:       di.Argument{Kind: di.ArgTagged, Value: "sometag"},
			paramType: stringType,
			wantErr:   "tagged injection requires []T, map[string]T or iter.Seq2[string, T], got string",
		},
		{
			name:      "spread_not_slice",
			arg:       di.Argument{Kind: di.ArgSpread, Value: "@dep"},
			paramType: stringType,
			wantErr:   "variadic parameters",
		},
		{
			name:      "spread_ok",
			arg:       di.Argument{Kind: di.ArgSpread, Value: "@handler_svc"},
			paramType: types.NewSlice(intType),
			wantKind:  SpreadArg,
		},
		{
			name:      "spread_inner_error",
			arg:       di.Argument{Kind: di.ArgSpread, Value: "@missing"},
			paramType: types.NewSlice(intType),
			wantErr:   "unknown service",
		},
		{
			name:      "goref_ok",
			arg:       di.Argument{Kind: di.ArgGoRef, Value: "mypkg.MyVar"},
			paramType: stringType,
			wantKind:  GoRefArg,
		},
		{
			name:      "goref_invalid_name",
			arg:       di.Argument{Kind: di.ArgGoRef, Value: "noDotHere"},
			paramType: stringType,
			wantErr:   "invalid",
		},
		{
			name:      "goref_lookup_error",
			arg:       di.Argument{Kind: di.ArgGoRef, Value: "mypkg.Missing"},
			paramType: stringType,
			wantErr:   "not found",
		},
		{
			name:      "goref_type_mismatch",
			arg:       di.Argument{Kind: di.ArgGoRef, Value: "mypkg.MyVar"},
			paramType: intType,
			wantErr:   "not assignable",
		},
		{
			name:      "literal_string",
			arg:       di.Argument{Kind: di.ArgLiteral, Literal: di.NewStringLiteral("hello")},
			paramType: stringType,
			wantKind:  LiteralArg,
		},
		{
			name:      "unknown_kind",
			arg:       di.Argument{Kind: di.ArgumentKind(99)},
			paramType: stringType,
			wantErr:   "unknown argument kind",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := &argResolver{typeResolver: resolver}
			result, err := r.resolve(container, noResolve, "svc", 0, tt.arg, tt.paramType)
			if tt.wantErr != "" {
				if err == nil {
					t.Fatalf("expected error containing %q, got nil", tt.wantErr)
				}
				if !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("expected error containing %q, got: %v", tt.wantErr, err)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if result.Kind != tt.wantKind {
				t.Fatalf("expected kind %d, got %d", tt.wantKind, result.Kind)
			}
		})
	}
}

func TestResolveTaggedShape(t *testing.T) {
	stringType := types.Typ[types.String]
	intType := types.Typ[types.Int]
	boolType := types.Typ[types.Bool]
	pkg := types.NewPackage("example.com/app", "app")

	// seq2 builds func(yield func(K, V) bool), the underlying type of iter.Seq2.
	seq2 := func(key, value types.Type) *types.Signature {
		yield := types.NewSignatureType(nil, nil, nil,
			types.NewTuple(types.NewParam(0, nil, "", key), types.NewParam(0, nil, "", value)),
			types.NewTuple(types.NewParam(0, nil, "", boolType)), false)
		return types.NewSignatureType(nil, nil, nil,
			types.NewTuple(types.NewParam(0, nil, "yield", yield)), nil, false)
	}
	named := func(name string, underlying types.Type) types.Type {
		return types.NewNamed(types.NewTypeName(0, pkg, name, nil), underlying, nil)
	}
	// iterSeq2 stands for an instance of iter.Seq2: only the package and the
	// name of the type matter to the check.
	iterSeq2 := types.NewNamed(types.NewTypeName(0, types.NewPackage("iter", "iter"), "Seq2", nil),
		seq2(stringType, intType), nil)

	for _, tt := range []struct {
		name      string
		paramType types.Type
		wantShape TagShape
		wantErr   string
	}{
		{name: "slice", paramType: types.NewSlice(intType), wantShape: TagShapeSlice},
		{name: "map", paramType: types.NewMap(stringType, intType), wantShape: TagShapeMap},
		{name: "named map", paramType: named("Registry", types.NewMap(stringType, intType)), wantShape: TagShapeMap},
		{
			name:      "slice of a private alias",
			paramType: types.NewSlice(types.NewAlias(types.NewTypeName(0, pkg, "number", nil), intType)),
			wantShape: TagShapeSlice,
		},
		{name: "sequence", paramType: seq2(stringType, intType), wantShape: TagShapeSeq2},
		{name: "iter.Seq2", paramType: iterSeq2, wantShape: TagShapeSeq2},
		{name: "other named sequence", paramType: named("Seq2", seq2(stringType, intType)), wantShape: TagShapeSeq2},
		{
			name: "sequence with a named callback",
			paramType: types.NewSignatureType(nil, nil, nil,
				types.NewTuple(types.NewParam(0, nil, "yield", named("Yield", seq2(stringType, intType).Params().At(0).Type()))),
				nil, false),
			wantErr: "tagged injection requires []T, map[string]T or iter.Seq2[string, T], got func(yield example.com/app.Yield)",
		},
		{
			name:      "map with int keys",
			paramType: types.NewMap(intType, intType),
			wantErr:   "tagged injection requires string keys, got map[int]int",
		},
		{
			name:      "map with named string keys",
			paramType: types.NewMap(named("Route", stringType), intType),
			wantErr:   "tagged injection requires string keys",
		},
		{
			name:      "sequence with int keys",
			paramType: seq2(intType, intType),
			wantErr:   "tagged injection requires string keys",
		},
		{
			name:      "other func",
			paramType: types.NewSignatureType(nil, nil, nil, nil, nil, false),
			wantErr:   "tagged injection requires []T, map[string]T or iter.Seq2[string, T], got func()",
		},
		{
			name:      "interface",
			paramType: types.NewInterfaceType(nil, nil),
			wantErr:   "tagged injection requires []T, map[string]T or iter.Seq2[string, T]",
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			container := NewContainer()
			r := &argResolver{typeResolver: &testResolver{}}
			arg := di.Argument{Kind: di.ArgTagged, Value: "handler"}
			result, err := r.resolve(container, noResolve, "svc", 0, arg, tt.paramType)
			if tt.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("error = %v, want it to contain %q", err, tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if result.TagShape != tt.wantShape {
				t.Errorf("shape = %d, want %d", result.TagShape, tt.wantShape)
			}
			got := container.tags["handler"].ElementType
			if !types.Identical(got, intType) {
				t.Errorf("element type = %v, want int", got)
			}
			// Identical ignores aliases, so check the alias separately.
			if _, ok := got.(*types.Alias); ok {
				t.Errorf("element type %v is an alias", got)
			}
		})
	}
}

func TestResolveFieldAccess(t *testing.T) {
	stringType := types.Typ[types.String]
	intType := types.Typ[types.Int]

	// Inner struct: type Inner struct { DSN string }
	innerStruct := makeStruct("cfg", "DSN", stringType)
	// Outer struct: type Config struct { Host string; Port int; Database Inner; secret string }
	pkg := types.NewPackage("test/cfg", "cfg")
	outerStruct := types.NewStruct([]*types.Var{
		types.NewField(token.NoPos, pkg, "Host", stringType, false),
		types.NewField(token.NoPos, pkg, "Port", intType, false),
		types.NewField(token.NoPos, pkg, "Database", innerStruct, false),
		types.NewField(token.NoPos, pkg, "secret", stringType, false), // unexported
	}, nil)
	ptrOuter := types.NewPointer(outerStruct)

	resolver := &testResolver{
		vars: map[string]types.Object{
			"mypkg.DefaultCfg": makePkgVar("mypkg", "DefaultCfg", ptrOuter),
		},
	}

	container := NewContainer()
	container.Services["config"] = &Service{ID: "config", Type: ptrOuter}
	container.Services["config.db"] = &Service{ID: "config.db", Type: ptrOuter} // dotted ID

	tests := []struct {
		name      string
		arg       di.Argument
		paramType types.Type
		wantErr   string
	}{
		// Service field access
		{
			name:      "service_field_ok",
			arg:       di.Argument{Kind: di.ArgFieldAccessService, Value: "config.Host"},
			paramType: stringType,
		},
		{
			name:      "service_nested_field",
			arg:       di.Argument{Kind: di.ArgFieldAccessService, Value: "config.Database.DSN"},
			paramType: stringType,
		},
		{
			name:      "service_dotted_id",
			arg:       di.Argument{Kind: di.ArgFieldAccessService, Value: "config.db.Host"},
			paramType: stringType,
		},
		{
			name:      "service_no_field",
			arg:       di.Argument{Kind: di.ArgFieldAccessService, Value: "config"},
			paramType: stringType,
			wantErr:   "requires at least one field",
		},
		{
			name:      "service_not_found",
			arg:       di.Argument{Kind: di.ArgFieldAccessService, Value: "missing.Host"},
			paramType: stringType,
			wantErr:   "no matching service",
		},
		{
			name:      "service_unknown_field",
			arg:       di.Argument{Kind: di.ArgFieldAccessService, Value: "config.NoSuch"},
			paramType: stringType,
			wantErr:   "not found",
		},
		{
			name:      "service_unexported_field",
			arg:       di.Argument{Kind: di.ArgFieldAccessService, Value: "config.secret"},
			paramType: stringType,
			wantErr:   "not found", // unexported fields are not found by LookupFieldOrMethod with nil package
		},
		{
			name:      "service_type_mismatch",
			arg:       di.Argument{Kind: di.ArgFieldAccessService, Value: "config.Host"},
			paramType: intType,
			wantErr:   "not assignable",
		},
		// Go ref field access
		{
			name:      "goref_field_ok",
			arg:       di.Argument{Kind: di.ArgFieldAccessGo, Value: "mypkg.DefaultCfg.Host"},
			paramType: stringType,
		},
		{
			name:      "goref_nested_field",
			arg:       di.Argument{Kind: di.ArgFieldAccessGo, Value: "mypkg.DefaultCfg.Database.DSN"},
			paramType: stringType,
		},
		{
			name:      "goref_too_few_parts",
			arg:       di.Argument{Kind: di.ArgFieldAccessGo, Value: "x.y"},
			paramType: stringType,
			wantErr:   "requires at least one field",
		},
		{
			name:      "goref_symbol_not_found",
			arg:       di.Argument{Kind: di.ArgFieldAccessGo, Value: "mypkg.Missing.Field"},
			paramType: stringType,
			wantErr:   "no matching package-level symbol",
		},
		{
			name:      "goref_unknown_field",
			arg:       di.Argument{Kind: di.ArgFieldAccessGo, Value: "mypkg.DefaultCfg.NoSuch"},
			paramType: stringType,
			wantErr:   "not found",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := &argResolver{typeResolver: resolver}
			result, err := r.resolve(container, noResolve, "svc", 0, tt.arg, tt.paramType)
			if tt.wantErr != "" {
				if err == nil {
					t.Fatalf("expected error containing %q, got nil", tt.wantErr)
				}
				if !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("expected error containing %q, got: %v", tt.wantErr, err)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if result.Kind != FieldAccessArg {
				t.Fatalf("expected FieldAccessArg, got %d", result.Kind)
			}
		})
	}
}

func TestResolveLiteral(t *testing.T) {
	pkg := types.NewPackage("test/lit", "lit")
	namedString := types.NewNamed(types.NewTypeName(token.NoPos, pkg, "Name", nil), types.Typ[types.String], nil)
	timePkg := types.NewPackage("time", "time")
	durationType := types.NewNamed(types.NewTypeName(token.NoPos, timePkg, "Duration", nil), types.Typ[types.Int64], nil)
	emptyIface := types.NewInterfaceType(nil, nil)
	emptyIface.Complete()
	errorType := types.Universe.Lookup("error").Type()

	tests := []struct {
		name        string
		lit         di.Literal
		paramType   types.Type
		wantLitType LiteralType
		wantErr     string
	}{
		// Literal kinds that cannot produce a value of the target type.
		{
			name:      "string_to_int",
			lit:       di.NewStringLiteral("hello world"),
			paramType: types.Typ[types.Int],
			wantErr:   "cannot use",
		},
		{
			name:      "null_to_int",
			lit:       di.NewNullLiteral(),
			paramType: types.Typ[types.Int],
			wantErr:   "not nilable",
		},
		{
			name:      "int_overflows_int8",
			lit:       di.NewIntLiteral(5000),
			paramType: types.Typ[types.Int8],
			wantErr:   "overflows",
		},
		{
			name:      "nan_to_float64",
			lit:       di.NewFloatLiteral(math.NaN()),
			paramType: types.Typ[types.Float64],
			wantErr:   "use a !go: reference",
		},
		{
			name:      "inf_to_float64",
			lit:       di.NewFloatLiteral(math.Inf(1)),
			paramType: types.Typ[types.Float64],
			wantErr:   "use a !go: reference",
		},
		{
			name:      "bool_to_int",
			lit:       di.NewBoolLiteral(true),
			paramType: types.Typ[types.Int],
			wantErr:   "cannot use",
		},
		{
			name:      "int_to_bool",
			lit:       di.NewIntLiteral(1),
			paramType: types.Typ[types.Bool],
			wantErr:   "cannot use",
		},
		{
			name:      "int_to_string",
			lit:       di.NewIntLiteral(42),
			paramType: types.Typ[types.String],
			wantErr:   "cannot use",
		},
		{
			name:      "string_to_bool",
			lit:       di.NewStringLiteral("true"),
			paramType: types.Typ[types.Bool],
			wantErr:   "cannot use",
		},
		{
			name:      "fractional_float_to_int",
			lit:       di.NewFloatLiteral(3.14),
			paramType: types.Typ[types.Int],
			wantErr:   "truncated",
		},
		{
			name:      "negative_int_to_uint",
			lit:       di.NewIntLiteral(-1),
			paramType: types.Typ[types.Uint],
			wantErr:   "overflows",
		},
		{
			name:      "float_overflows_float32",
			lit:       di.NewFloatLiteral(1e39),
			paramType: types.Typ[types.Float32],
			wantErr:   "overflows",
		},
		{
			name:      "string_to_struct",
			lit:       di.NewStringLiteral("hello"),
			paramType: types.NewStruct(nil, nil),
			wantErr:   "cannot use",
		},
		{
			name:      "string_to_error_iface",
			lit:       di.NewStringLiteral("boom"),
			paramType: errorType,
			wantErr:   "cannot use",
		},
		// Cross-kind combinations Go's untyped constants permit must keep working.
		{
			name:        "int_to_float64",
			lit:         di.NewIntLiteral(5),
			paramType:   types.Typ[types.Float64],
			wantLitType: IntLiteral,
		},
		{
			name:        "integral_float_to_int",
			lit:         di.NewFloatLiteral(5.0),
			paramType:   types.Typ[types.Int],
			wantLitType: FloatLiteral,
		},
		{
			name:        "string_to_named_string",
			lit:         di.NewStringLiteral("hello"),
			paramType:   namedString,
			wantLitType: StringLiteral,
		},
		{
			name:        "int_to_duration",
			lit:         di.NewIntLiteral(5000000000),
			paramType:   durationType,
			wantLitType: DurationLiteral,
		},
		{
			name:        "string_to_duration",
			lit:         di.NewStringLiteral("5s"),
			paramType:   durationType,
			wantLitType: DurationLiteral,
		},
		{
			name:        "int_within_int8_range",
			lit:         di.NewIntLiteral(5),
			paramType:   types.Typ[types.Int8],
			wantLitType: IntLiteral,
		},
		{
			name:        "int_to_empty_interface",
			lit:         di.NewIntLiteral(42),
			paramType:   emptyIface,
			wantLitType: IntLiteral,
		},
		{
			name:        "string_to_empty_interface",
			lit:         di.NewStringLiteral("hello"),
			paramType:   emptyIface,
			wantLitType: StringLiteral,
		},
		{
			name:        "null_to_pointer",
			lit:         di.NewNullLiteral(),
			paramType:   types.NewPointer(types.Typ[types.Int]),
			wantLitType: NullLiteral,
		},
		{
			name:        "null_to_slice",
			lit:         di.NewNullLiteral(),
			paramType:   types.NewSlice(types.Typ[types.Int]),
			wantLitType: NullLiteral,
		},
		{
			name:        "null_to_interface",
			lit:         di.NewNullLiteral(),
			paramType:   errorType,
			wantLitType: NullLiteral,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := &argResolver{}
			arg := di.Argument{Kind: di.ArgLiteral, Literal: tt.lit}
			result, err := r.resolve(NewContainer(), noResolve, "svc", 0, arg, tt.paramType)
			if tt.wantErr != "" {
				if err == nil {
					t.Fatalf("expected error containing %q, got nil", tt.wantErr)
				}
				if !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("expected error containing %q, got: %v", tt.wantErr, err)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if result.Kind != LiteralArg {
				t.Fatalf("expected LiteralArg, got %d", result.Kind)
			}
			if result.Literal.Type != tt.wantLitType {
				t.Fatalf("expected literal type %d, got %d", tt.wantLitType, result.Literal.Type)
			}
		})
	}
}

func TestSpreadLiteralInnerRejected(t *testing.T) {
	container := NewContainer()
	r := &argResolver{}
	arg := di.Argument{Kind: di.ArgSpread, Value: "hello"}

	_, err := r.resolve(container, noResolve, "svc", 0, arg, types.NewSlice(types.Typ[types.Int]))
	if err == nil || !strings.Contains(err.Error(), "!spread") {
		t.Fatalf("expected spread inner error, got %v", err)
	}
}

func TestRuntimeParamContextualTypes(t *testing.T) {
	container := NewContainer()
	r := &argResolver{}

	arg := di.Argument{Kind: di.ArgParam, Value: "runtime.param"}
	first, err := r.resolve(container, noResolve, "svc.one", 0, arg, types.Typ[types.String])
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// A different target type at another injection site is allowed: the
	// conversion is contextual. Both usages share one parameter entry.
	second, err := r.resolve(container, noResolve, "svc.two", 0, arg, types.Typ[types.Int])
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if first.Parameter != second.Parameter {
		t.Fatalf("expected repeated usages to share one parameter object")
	}
	if !types.Identical(first.Type, types.Typ[types.String]) || !types.Identical(second.Type, types.Typ[types.Int]) {
		t.Fatalf("expected each usage to keep its own target type, got %s and %s", first.Type, second.Type)
	}

	// A target type outside the scalar set fails generation.
	_, err = r.resolve(container, noResolve, "svc.three", 0, arg, types.NewStruct(nil, nil))
	if err == nil || !strings.Contains(err.Error(), "unsupported target type") {
		t.Fatalf("expected unsupported target type error, got %v", err)
	}
}
