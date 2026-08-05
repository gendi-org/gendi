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
			name:      "tagged_not_slice",
			arg:       di.Argument{Kind: di.ArgTagged, Value: "sometag"},
			paramType: stringType,
			wantErr:   "requires slice type",
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

func TestResolveMapArgument(t *testing.T) {
	strMap := types.NewMap(types.Typ[types.String], types.Typ[types.Int])
	container := NewContainer()
	container.Services["dep"] = &Service{ID: "dep", Type: types.Typ[types.Int]}
	r := &argResolver{typeResolver: &testResolver{}}

	arg := di.Argument{
		Kind: di.ArgMap,
		Entries: []di.ArgEntry{
			{Key: di.NewStringLiteral("a"), Value: di.Argument{Kind: di.ArgLiteral, Literal: di.NewIntLiteral(1)}},
			{Key: di.NewStringLiteral("b"), Value: di.Argument{Kind: di.ArgServiceRef, Value: "dep"}},
		},
	}

	resolved, err := r.resolve(container, noResolve, "svc", 0, arg, strMap)
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	if resolved.Kind != MapArg {
		t.Fatalf("kind = %d, want MapArg (%d)", resolved.Kind, MapArg)
	}
	if !types.Identical(resolved.Type, strMap) {
		t.Errorf("type = %s, want %s", resolved.Type, strMap)
	}
	if len(resolved.Entries) != 2 {
		t.Fatalf("entries = %d, want 2", len(resolved.Entries))
	}
	if resolved.Entries[0].Key.Type != StringLiteral || resolved.Entries[0].Key.Value != "a" {
		t.Errorf("entry[0] key = %#v, want string \"a\"", resolved.Entries[0].Key)
	}
	if resolved.Entries[0].Value.Kind != LiteralArg {
		t.Errorf("entry[0] value kind = %d, want LiteralArg", resolved.Entries[0].Value.Kind)
	}
	if !types.Identical(resolved.Entries[0].Value.Type, types.Typ[types.Int]) {
		t.Errorf("entry[0] value type = %s, want int", resolved.Entries[0].Value.Type)
	}
	if resolved.Entries[1].Value.Kind != ServiceRefArg || resolved.Entries[1].Value.Service == nil {
		t.Errorf("entry[1] must resolve to the dep service, got %#v", resolved.Entries[1].Value)
	}
}

func TestResolveMapArgumentRejects(t *testing.T) {
	strMap := types.NewMap(types.Typ[types.String], types.Typ[types.Int])
	intKeyMap := types.NewMap(types.Typ[types.Int], types.Typ[types.String])
	floatKeyMap := types.NewMap(types.Typ[types.Float64], types.Typ[types.Int])
	timePkg := types.NewPackage("time", "time")
	durationType := types.NewNamed(types.NewTypeName(token.NoPos, timePkg, "Duration", nil), types.Typ[types.Int64], nil)
	durationKeyMap := types.NewMap(durationType, types.Typ[types.Int])
	r := &argResolver{typeResolver: &testResolver{}}

	for _, tt := range []struct {
		name       string
		paramType  types.Type
		entries    []di.ArgEntry
		wantErrHas string
	}{
		{
			name:       "non-map parameter",
			paramType:  types.Typ[types.String],
			entries:    []di.ArgEntry{{Key: di.NewStringLiteral("a"), Value: di.Argument{Kind: di.ArgLiteral, Literal: di.NewIntLiteral(1)}}},
			wantErrHas: "map argument requires map type, got string",
		},
		{
			name:       "key type mismatch",
			paramType:  intKeyMap,
			entries:    []di.ArgEntry{{Key: di.NewStringLiteral("a"), Value: di.Argument{Kind: di.ArgLiteral, Literal: di.NewStringLiteral("v")}}},
			wantErrHas: "map key: cannot use string literal \"a\" as int",
		},
		{
			name:       "value type mismatch",
			paramType:  strMap,
			entries:    []di.ArgEntry{{Key: di.NewStringLiteral("a"), Value: di.Argument{Kind: di.ArgLiteral, Literal: di.NewStringLiteral("v")}}},
			wantErrHas: "cannot use string literal",
		},
		{
			name:      "duplicate key",
			paramType: strMap,
			entries: []di.ArgEntry{
				{Key: di.NewStringLiteral("a"), Value: di.Argument{Kind: di.ArgLiteral, Literal: di.NewIntLiteral(1)}},
				{Key: di.NewStringLiteral("a"), Value: di.Argument{Kind: di.ArgLiteral, Literal: di.NewIntLiteral(2)}},
			},
			wantErrHas: "duplicate map key",
		},
		{
			name:      "duplicate key across int/float literal forms for a float64 key",
			paramType: floatKeyMap,
			entries: []di.ArgEntry{
				{Key: di.NewIntLiteral(100), Value: di.Argument{Kind: di.ArgLiteral, Literal: di.NewIntLiteral(1)}},
				{Key: di.NewFloatLiteral(1e2), Value: di.Argument{Kind: di.ArgLiteral, Literal: di.NewIntLiteral(2)}},
			},
			wantErrHas: "duplicate map key",
		},
		{
			name:      "duplicate key across string/int literal forms for a time.Duration key",
			paramType: durationKeyMap,
			entries: []di.ArgEntry{
				{Key: di.NewStringLiteral("30s"), Value: di.Argument{Kind: di.ArgLiteral, Literal: di.NewIntLiteral(1)}},
				{Key: di.NewIntLiteral(30000000000), Value: di.Argument{Kind: di.ArgLiteral, Literal: di.NewIntLiteral(2)}},
			},
			wantErrHas: "duplicate map key",
		},
		{
			// 1.0000000000000002 is the float64 value nearest to 1 other than
			// 1 itself; a map[float32]X key narrows both to the same float32
			// constant, so the generated composite literal would carry a
			// duplicate key if identity stayed at float64 precision.
			name:      "duplicate key across literals that only collide at float32 precision",
			paramType: types.NewMap(types.Typ[types.Float32], types.Typ[types.Int]),
			entries: []di.ArgEntry{
				{Key: di.NewFloatLiteral(1.0000000000000002), Value: di.Argument{Kind: di.ArgLiteral, Literal: di.NewIntLiteral(1)}},
				{Key: di.NewIntLiteral(1), Value: di.Argument{Kind: di.ArgLiteral, Literal: di.NewIntLiteral(2)}},
			},
			wantErrHas: "duplicate map key",
		},
		{
			name:       "null key",
			paramType:  strMap,
			entries:    []di.ArgEntry{{Key: di.NewNullLiteral(), Value: di.Argument{Kind: di.ArgLiteral, Literal: di.NewIntLiteral(1)}}},
			wantErrHas: "map argument key cannot be null",
		},
		{
			name:       "tagged value",
			paramType:  strMap,
			entries:    []di.ArgEntry{{Key: di.NewStringLiteral("a"), Value: di.Argument{Kind: di.ArgTagged, Value: "handler"}}},
			wantErrHas: "!tagged: is not allowed as a map value",
		},
		{
			name:       "nested map value",
			paramType:  strMap,
			entries:    []di.ArgEntry{{Key: di.NewStringLiteral("a"), Value: di.Argument{Kind: di.ArgMap}}},
			wantErrHas: "a nested map is not allowed as a map value",
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			_, err := r.resolve(NewContainer(), noResolve, "svc", 0, di.Argument{Kind: di.ArgMap, Entries: tt.entries}, tt.paramType)
			if err == nil {
				t.Fatal("expected an error")
			}
			if !strings.Contains(err.Error(), tt.wantErrHas) {
				t.Fatalf("error = %v, want it to contain %q", err, tt.wantErrHas)
			}
		})
	}
}

// TestResolveMapArgumentInterfaceKeysNotCollapsed guards against
// over-normalizing key identity: for an interface{} key type, int(100) and
// float64(100) are distinct Go map keys, so a config using both must resolve
// both entries rather than being rejected as a duplicate.
func TestResolveMapArgumentInterfaceKeysNotCollapsed(t *testing.T) {
	emptyIface := types.NewInterfaceType(nil, nil)
	emptyIface.Complete()
	anyMap := types.NewMap(emptyIface, types.Typ[types.Int])
	r := &argResolver{typeResolver: &testResolver{}}

	arg := di.Argument{
		Kind: di.ArgMap,
		Entries: []di.ArgEntry{
			{Key: di.NewIntLiteral(100), Value: di.Argument{Kind: di.ArgLiteral, Literal: di.NewIntLiteral(1)}},
			{Key: di.NewFloatLiteral(100), Value: di.Argument{Kind: di.ArgLiteral, Literal: di.NewIntLiteral(2)}},
		},
	}

	resolved, err := r.resolve(NewContainer(), noResolve, "svc", 0, arg, anyMap)
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	if len(resolved.Entries) != 2 {
		t.Fatalf("entries = %d, want 2 (int(100) and float64(100) are distinct interface keys)", len(resolved.Entries))
	}
}

// TestResolveMapArgumentFloat32DistinctKeysNotCollapsed guards against an
// over-eager float32 fix: two keys that remain distinct once narrowed to
// float32 precision must both resolve, not be rejected as duplicates.
func TestResolveMapArgumentFloat32DistinctKeysNotCollapsed(t *testing.T) {
	float32KeyMap := types.NewMap(types.Typ[types.Float32], types.Typ[types.Int])
	r := &argResolver{typeResolver: &testResolver{}}

	arg := di.Argument{
		Kind: di.ArgMap,
		Entries: []di.ArgEntry{
			{Key: di.NewFloatLiteral(1.0), Value: di.Argument{Kind: di.ArgLiteral, Literal: di.NewIntLiteral(1)}},
			{Key: di.NewFloatLiteral(1.0001), Value: di.Argument{Kind: di.ArgLiteral, Literal: di.NewIntLiteral(2)}},
		},
	}

	resolved, err := r.resolve(NewContainer(), noResolve, "svc", 0, arg, float32KeyMap)
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	if len(resolved.Entries) != 2 {
		t.Fatalf("entries = %d, want 2 (1.0 and 1.0001 are still distinct at float32 precision)", len(resolved.Entries))
	}
}
