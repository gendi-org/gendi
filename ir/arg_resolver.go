package ir

import (
	"fmt"
	"go/constant"
	"go/types"
	"math"
	"strconv"
	"strings"
	"time"

	di "github.com/gendi-org/gendi"
	"github.com/gendi-org/gendi/srcloc"
	"github.com/gendi-org/gendi/typeres"
	"github.com/gendi-org/gendi/yaml"
)

// argResolver resolves constructor arguments
type argResolver struct {
	typeResolver TypeResolver
}

// resolve resolves a single constructor argument. resolveSvc forces
// resolution of another service before its type is inspected.
func (r *argResolver) resolve(container *Container, resolveSvc func(string) error, svcID string, idx int, arg di.Argument, paramType types.Type) (*Argument, error) {
	switch arg.Kind {
	case di.ArgServiceRef:
		return r.resolveServiceRef(container, svcID, idx, arg, paramType)
	case di.ArgInner:
		return nil, srcloc.Errorf(arg.SourceLoc, "service %q arg[%d]: @.inner should have been resolved by DecoratorPass", svcID, idx)
	case di.ArgParam:
		return r.resolveParam(container, svcID, idx, arg, paramType)
	case di.ArgTagged:
		return r.resolveTagged(container, svcID, idx, arg, paramType)
	case di.ArgSpread:
		return r.resolveSpread(container, resolveSvc, svcID, idx, arg, paramType)
	case di.ArgFieldAccessService:
		return r.resolveFieldAccessServiceArg(container, resolveSvc, svcID, idx, arg, paramType)
	case di.ArgFieldAccessGo:
		return r.resolveFieldAccessGoArg(svcID, idx, arg, paramType)
	case di.ArgGoRef:
		return r.resolveGoRef(svcID, idx, arg, paramType)
	case di.ArgMap:
		return r.resolveMap(container, resolveSvc, svcID, idx, arg, paramType)
	case di.ArgLiteral:
		return r.resolveLiteral(svcID, idx, arg, paramType)
	default:
		return nil, srcloc.Errorf(arg.SourceLoc, "service %q arg[%d]: unknown argument kind %d", svcID, idx, arg.Kind)
	}
}

func (r *argResolver) resolveServiceRef(container *Container, svcID string, idx int, arg di.Argument, paramType types.Type) (*Argument, error) {
	dep, ok := container.Services[arg.Value]
	if !ok {
		return nil, srcloc.Errorf(arg.SourceLoc, "service %q arg[%d]: unknown service %q", svcID, idx, arg.Value)
	}
	return &Argument{Type: paramType, Kind: ServiceRefArg, Service: dep}, nil
}

func (r *argResolver) resolveParam(container *Container, svcID string, idx int, arg di.Argument, paramType types.Type) (*Argument, error) {
	if _, _, err := ParamScalarKind(paramType); err != nil {
		return nil, srcloc.Errorf(arg.SourceLoc, "service %q arg[%d]: parameter %q: %v", svcID, idx, arg.Value, err)
	}
	param, ok := container.Parameters[arg.Value]
	if !ok {
		// The parameter might be provided at runtime. Register it alongside
		// declared parameters so every usage resolves to the same IR entry.
		param = &Parameter{Name: arg.Value}
		container.Parameters[arg.Value] = param
	}
	return &Argument{Type: paramType, Kind: ParamRefArg, Parameter: param}, nil
}

func (r *argResolver) resolveTagged(container *Container, svcID string, idx int, arg di.Argument, paramType types.Type) (*Argument, error) {
	tag, ok := container.tags[arg.Value]
	if !ok {
		// Create tag on-demand - infer ElementType from parameter
		tag = &Tag{
			Name:     arg.Value,
			Services: []*Service{},
		}
		container.tags[arg.Value] = tag
	}

	// Infer or validate ElementType from parameter type
	sliceType, ok := paramType.Underlying().(*types.Slice)
	if !ok {
		return nil, srcloc.Errorf(arg.SourceLoc, "service %q arg[%d]: tagged injection requires slice type, got %s", svcID, idx, paramType)
	}
	elemType := sliceType.Elem()

	if tag.ElementType == nil {
		// Infer ElementType from parameter
		tag.ElementType = elemType
	} else if !types.AssignableTo(tag.ElementType, elemType) {
		// Validate consistency if ElementType was declared or inferred earlier
		return nil, srcloc.Errorf(arg.SourceLoc, "service %q arg[%d]: tag %q element type mismatch: %s is not assignable to %s",
			svcID, idx, arg.Value, tag.ElementType, elemType)
	}

	return &Argument{Type: paramType, Kind: TaggedArg, Tag: tag}, nil
}

func (r *argResolver) resolveSpread(container *Container, resolveSvc func(string) error, svcID string, idx int, arg di.Argument, paramType types.Type) (*Argument, error) {
	// Validate that parameter is variadic (represented as slice in go/types)
	if _, ok := paramType.Underlying().(*types.Slice); !ok {
		return nil, srcloc.Errorf(arg.SourceLoc, "service %q arg[%d]: !spread: can only be used with variadic parameters, got %s", svcID, idx, paramType)
	}

	// Parse inner argument string
	innerKind, innerValue, err := yaml.ParseArgumentString(arg.Value)
	if err != nil {
		return nil, srcloc.WrapError(arg.SourceLoc, fmt.Sprintf("service %q arg[%d]: !spread", svcID, idx), err)
	}
	// A literal cannot be spread: the inner expression must evaluate to a slice.
	if innerKind == di.ArgLiteral {
		return nil, srcloc.Errorf(arg.SourceLoc, "service %q arg[%d]: !spread: inner value %q must be a service reference or tagged collection", svcID, idx, arg.Value)
	}
	innerArg := di.Argument{
		Kind:  innerKind,
		Value: innerValue,
	}

	// Resolve inner argument with the slice type (not element type)
	// The inner expression should evaluate to []T
	innerResolved, err := r.resolve(container, resolveSvc, svcID, idx, innerArg, paramType)
	if err != nil {
		return nil, srcloc.WrapError(arg.SourceLoc, fmt.Sprintf("service %q arg[%d]: !spread", svcID, idx), err)
	}

	// Validate that inner resolved to a slice type
	if _, ok := innerResolved.Type.Underlying().(*types.Slice); !ok {
		return nil, srcloc.Errorf(arg.SourceLoc, "service %q arg[%d]: !spread: requires slice type, got %s", svcID, idx, innerResolved.Type)
	}

	return &Argument{Type: paramType, Kind: SpreadArg, Inner: innerResolved}, nil
}

func (r *argResolver) resolveFieldAccessServiceArg(container *Container, resolveSvc func(string) error, svcID string, idx int, arg di.Argument, paramType types.Type) (*Argument, error) {
	fa, err := r.resolveFieldAccessOnService(container, resolveSvc, svcID, idx, arg, arg.Value)
	if err != nil {
		return nil, err
	}
	if !types.AssignableTo(fa.ResultType, paramType) {
		return nil, srcloc.Errorf(arg.SourceLoc, "service %q arg[%d]: field access result type %s is not assignable to %s",
			svcID, idx, fa.ResultType, paramType)
	}
	return &Argument{Type: paramType, Kind: FieldAccessArg, FieldAccess: fa}, nil
}

func (r *argResolver) resolveFieldAccessGoArg(svcID string, idx int, arg di.Argument, paramType types.Type) (*Argument, error) {
	fa, err := r.resolveFieldAccessOnGoRef(svcID, idx, arg, arg.Value)
	if err != nil {
		return nil, err
	}
	if !types.AssignableTo(fa.ResultType, paramType) {
		return nil, srcloc.Errorf(arg.SourceLoc, "service %q arg[%d]: field access result type %s is not assignable to %s",
			svcID, idx, fa.ResultType, paramType)
	}
	return &Argument{Type: paramType, Kind: FieldAccessArg, FieldAccess: fa}, nil
}

func (r *argResolver) resolveGoRef(svcID string, idx int, arg di.Argument, paramType types.Type) (*Argument, error) {
	pkgPath, name, _, err := typeres.SplitQualifiedNameWithTypeParams(arg.Value)
	if err != nil {
		return nil, srcloc.WrapError(arg.SourceLoc, fmt.Sprintf("service %q arg[%d]: invalid go reference %q", svcID, idx, arg.Value), err)
	}
	obj, err := r.typeResolver.LookupVar(pkgPath, name)
	if err != nil {
		return nil, srcloc.WrapError(arg.SourceLoc, fmt.Sprintf("service %q arg[%d]", svcID, idx), err)
	}
	if !types.AssignableTo(obj.Type(), paramType) {
		return nil, srcloc.Errorf(arg.SourceLoc, "service %q arg[%d]: go reference %q type %s is not assignable to %s",
			svcID, idx, arg.Value, obj.Type(), paramType)
	}
	return &Argument{Type: paramType, Kind: GoRefArg, GoRef: &GoRef{Object: obj}}, nil
}

func (r *argResolver) resolveLiteral(svcID string, idx int, arg di.Argument, paramType types.Type) (*Argument, error) {
	litVal, err := r.convertLiteral(arg.Literal, paramType)
	if err != nil {
		return nil, srcloc.WrapError(arg.SourceLoc, fmt.Sprintf("service %q arg[%d]", svcID, idx), err)
	}
	return &Argument{Type: paramType, Kind: LiteralArg, Literal: litVal}, nil
}

// resolveMap resolves a map argument: literal keys, each with its own argument
// value. The forms that only make sense as a whole argument are rejected here
// as well as in the loader, because a di.Config can be built programmatically.
func (r *argResolver) resolveMap(container *Container, resolveSvc func(string) error, svcID string, idx int, arg di.Argument, paramType types.Type) (*Argument, error) {
	mapType, ok := paramType.Underlying().(*types.Map)
	if !ok {
		return nil, srcloc.Errorf(arg.SourceLoc, "service %q arg[%d]: map argument requires map type, got %s", svcID, idx, paramType)
	}
	// The parameter type comes from a compiled signature, where Go already
	// guarantees a comparable key; the check keeps a pathological IR producer
	// from turning into an obscure failure downstream.
	if !types.Comparable(mapType.Key()) {
		return nil, srcloc.Errorf(arg.SourceLoc, "service %q arg[%d]: map key type %s is not comparable", svcID, idx, mapType.Key())
	}

	entries := make([]MapEntry, 0, len(arg.Entries))
	seen := make(map[string]bool, len(arg.Entries))

	for _, entry := range arg.Entries {
		loc := entry.SourceLoc
		if loc == nil {
			loc = arg.SourceLoc
		}

		if entry.Key.IsNull() {
			return nil, srcloc.Errorf(loc, "service %q arg[%d]: map argument key cannot be null", svcID, idx)
		}
		key, err := r.convertLiteral(entry.Key, mapType.Key())
		if err != nil {
			return nil, srcloc.WrapError(loc, fmt.Sprintf("service %q arg[%d]: map key", svcID, idx), err)
		}
		dedupKey, err := r.mapKeyIdentity(key, mapType.Key())
		if err != nil {
			return nil, srcloc.WrapError(loc, fmt.Sprintf("service %q arg[%d]: map key", svcID, idx), err)
		}
		if seen[dedupKey] {
			return nil, srcloc.Errorf(loc, "service %q arg[%d]: duplicate map key %s", svcID, idx, r.describeLiteral(entry.Key))
		}
		seen[dedupKey] = true

		switch entry.Value.Kind {
		case di.ArgMap, di.ArgSpread, di.ArgTagged, di.ArgInner:
			return nil, srcloc.Errorf(loc, "service %q arg[%d]: %s is not allowed as a map value",
				svcID, idx, r.argKindToken(entry.Value.Kind))
		}

		value, err := r.resolve(container, resolveSvc, svcID, idx, entry.Value, mapType.Elem())
		if err != nil {
			return nil, err
		}
		entries = append(entries, MapEntry{Key: key, Value: value})
	}

	return &Argument{Type: paramType, Kind: MapArg, Entries: entries}, nil
}

// mapKeyIdentity renders a key the way the Go compiler will see it in the
// generated composite literal, so keys that differ as literals but denote the
// same constant collide here instead of producing a map literal that does not
// compile: 100 and 1e2 for a float64 key, "30s" and 30000000000 for a
// time.Duration key, 0.0 and -0.0 for any numeric key (Go constants have no
// signed zero). For an interface key type the literal's kind stays part of
// the identity on top of that — int(100) and float64(100) are distinct
// interface keys, and collapsing them would reject a valid config.
//
// A key type narrower than float64 loses precision the check above does not:
// two literals distinct at float64 (e.g. 1 and 1.0000000000000002) can denote
// the same float32 or complex64 key once the Go compiler converts them, so
// for those key kinds the identity is computed at the key type's own
// precision instead.
func (r *argResolver) mapKeyIdentity(key LiteralValue, keyType types.Type) (string, error) {
	narrowToFloat32 := false
	if basic, ok := keyType.Underlying().(*types.Basic); ok {
		narrowToFloat32 = basic.Kind() == types.Float32 || basic.Kind() == types.Complex64
	}

	value, err := r.mapKeyValueIdentity(key, narrowToFloat32)
	if err != nil {
		return "", err
	}

	if _, ok := keyType.Underlying().(*types.Interface); ok {
		// The kind stays part of the identity here: int(100) and float64(100)
		// are distinct interface keys, and collapsing them would reject a
		// valid config. The value has already been canonicalized above, so
		// this only needs to keep kinds apart, not also normalize within one.
		return fmt.Sprintf("%d:%s", key.Type, value), nil
	}
	return value, nil
}

// mapKeyValueIdentity renders a literal's value the way the Go compiler's
// constant representation would, so two literals of the same kind that
// denote the same value collide instead of surviving deduplication: Go
// constants have no signed zero, so 0.0 and -0.0 must render identically or
// a map[any]V argument using both would produce a composite literal with a
// duplicate key that fails to compile.
func (r *argResolver) mapKeyValueIdentity(key LiteralValue, narrowToFloat32 bool) (string, error) {
	switch key.Type {
	case StringLiteral:
		v, ok := key.Value.(string)
		if !ok {
			return "", fmt.Errorf("string literal must be string")
		}
		return strconv.Quote(v), nil
	case BoolLiteral:
		v, ok := key.Value.(bool)
		if !ok {
			return "", fmt.Errorf("bool literal must be bool")
		}
		return strconv.FormatBool(v), nil
	case IntLiteral:
		v, ok := key.Value.(int64)
		if !ok {
			return "", fmt.Errorf("int literal must be int64")
		}
		if narrowToFloat32 {
			return constant.MakeFloat64(float64(float32(v))).ExactString(), nil
		}
		return constant.MakeInt64(v).ExactString(), nil
	case FloatLiteral:
		v, ok := key.Value.(float64)
		if !ok {
			return "", fmt.Errorf("float literal must be float64")
		}
		if narrowToFloat32 {
			v = float64(float32(v))
		}
		return constant.MakeFloat64(v).ExactString(), nil
	case DurationLiteral:
		switch v := key.Value.(type) {
		case string:
			d, err := time.ParseDuration(v)
			if err != nil {
				return "", fmt.Errorf("invalid duration %q: %w", v, err)
			}
			return strconv.FormatInt(int64(d), 10), nil
		case int64:
			return strconv.FormatInt(v, 10), nil
		}
		return "", fmt.Errorf("duration literal must be string or int")
	}
	return "", fmt.Errorf("unsupported key literal kind %d", key.Type)
}

// argKindToken names an argument form the way it is spelled in YAML, for
// errors about forms that are not allowed inside a map.
func (r *argResolver) argKindToken(kind di.ArgumentKind) string {
	switch kind {
	case di.ArgMap:
		return "a nested map"
	case di.ArgSpread:
		return "!spread:"
	case di.ArgTagged:
		return "!tagged:"
	case di.ArgInner:
		return "@.inner"
	default:
		return "this value"
	}
}

// convertLiteral converts a di.Literal to an IR LiteralValue, validating that
// the literal can produce a value of targetType so mismatches fail at
// generation time instead of breaking compilation of the generated code.
func (r *argResolver) convertLiteral(lit di.Literal, targetType types.Type) (LiteralValue, error) {
	if typeres.IsDuration(targetType) {
		return r.convertDurationLiteral(lit)
	}
	if err := r.checkLiteralAssignable(lit, targetType); err != nil {
		return LiteralValue{}, err
	}

	switch lit.Kind {
	case di.LiteralString:
		return LiteralValue{Type: StringLiteral, Value: lit.String()}, nil
	case di.LiteralInt:
		return LiteralValue{Type: IntLiteral, Value: lit.Int()}, nil
	case di.LiteralFloat:
		return LiteralValue{Type: FloatLiteral, Value: lit.Float()}, nil
	case di.LiteralBool:
		return LiteralValue{Type: BoolLiteral, Value: lit.Bool()}, nil
	case di.LiteralNull:
		return LiteralValue{Type: NullLiteral, Value: nil}, nil
	default:
		return LiteralValue{}, fmt.Errorf("unsupported literal kind %d", lit.Kind)
	}
}

// convertDurationLiteral converts a duration literal (string "1s" or int nanoseconds)
func (r *argResolver) convertDurationLiteral(lit di.Literal) (LiteralValue, error) {
	switch lit.Kind {
	case di.LiteralString:
		// Parse as duration string - will be handled by generator
		return LiteralValue{Type: DurationLiteral, Value: lit.String()}, nil
	case di.LiteralInt:
		return LiteralValue{Type: DurationLiteral, Value: lit.Int()}, nil
	default:
		return LiteralValue{}, fmt.Errorf("cannot use %s as time.Duration: duration must be string or int", r.describeLiteral(lit))
	}
}

// checkLiteralAssignable validates that the literal, emitted into the
// generated source as an untyped Go constant (or nil), can be assigned to
// targetType. It mirrors Go's untyped constant conversion rules: cross-kind
// combinations the compiler accepts (int literals for float targets, integral
// float literals for integer targets, literals for interface targets their
// default type satisfies) stay valid, while kind mismatches, out-of-range
// constants, and null for non-nilable targets are rejected.
func (r *argResolver) checkLiteralAssignable(lit di.Literal, targetType types.Type) error {
	if lit.Kind == di.LiteralNull {
		switch under := targetType.Underlying().(type) {
		case *types.Pointer, *types.Interface, *types.Slice, *types.Map, *types.Chan, *types.Signature:
			return nil
		case *types.Basic:
			if under.Kind() == types.UnsafePointer {
				return nil
			}
		}
		return fmt.Errorf("cannot use %s as %s: target type is not nilable", r.describeLiteral(lit), targetType)
	}

	switch under := targetType.Underlying().(type) {
	case *types.Basic:
		return r.checkLiteralBasic(lit, under, targetType)
	case *types.Interface:
		var defaultType types.Type
		switch lit.Kind {
		case di.LiteralString:
			defaultType = types.Typ[types.String]
		case di.LiteralInt:
			defaultType = types.Typ[types.Int]
		case di.LiteralFloat:
			defaultType = types.Typ[types.Float64]
		case di.LiteralBool:
			defaultType = types.Typ[types.Bool]
		}
		if defaultType != nil && types.AssignableTo(defaultType, targetType) {
			return nil
		}
	}
	return fmt.Errorf("cannot use %s as %s", r.describeLiteral(lit), targetType)
}

// checkLiteralBasic validates a non-null literal against a target whose
// underlying type is a basic kind.
func (r *argResolver) checkLiteralBasic(lit di.Literal, basic *types.Basic, targetType types.Type) error {
	info := basic.Info()
	switch lit.Kind {
	case di.LiteralString:
		if info&types.IsString != 0 {
			return nil
		}
	case di.LiteralBool:
		if info&types.IsBoolean != 0 {
			return nil
		}
	case di.LiteralInt:
		switch {
		case info&types.IsInteger != 0:
			return r.checkIntegerConstant(lit, lit.Int(), basic, targetType)
		case info&(types.IsFloat|types.IsComplex) != 0:
			// Every int64 value is representable as a float or complex constant.
			return nil
		}
	case di.LiteralFloat:
		v := lit.Float()
		if math.IsNaN(v) || math.IsInf(v, 0) {
			return fmt.Errorf("cannot use %s as %s: NaN and infinities have no Go constant representation; use a !go: reference to a package-level variable instead", r.describeLiteral(lit), targetType)
		}
		switch {
		case info&types.IsInteger != 0:
			if v != math.Trunc(v) {
				return fmt.Errorf("%s truncated to %s", r.describeLiteral(lit), targetType)
			}
			if v < -9223372036854775808.0 || v >= 9223372036854775808.0 {
				return fmt.Errorf("%s overflows %s", r.describeLiteral(lit), targetType)
			}
			return r.checkIntegerConstant(lit, int64(v), basic, targetType)
		case info&(types.IsFloat|types.IsComplex) != 0:
			if (basic.Kind() == types.Float32 || basic.Kind() == types.Complex64) && math.Abs(v) > math.MaxFloat32 {
				return fmt.Errorf("%s overflows %s", r.describeLiteral(lit), targetType)
			}
			return nil
		}
	}
	return fmt.Errorf("cannot use %s as %s", r.describeLiteral(lit), targetType)
}

// checkIntegerConstant validates that an integral constant value fits the
// integer target's range, matching the compile-time constant overflow checks
// the generated code would otherwise hit.
func (r *argResolver) checkIntegerConstant(lit di.Literal, v int64, basic *types.Basic, targetType types.Type) error {
	var ok bool
	switch basic.Kind() {
	case types.Int:
		ok = v >= math.MinInt && v <= math.MaxInt
	case types.Int8:
		ok = v >= math.MinInt8 && v <= math.MaxInt8
	case types.Int16:
		ok = v >= math.MinInt16 && v <= math.MaxInt16
	case types.Int32:
		ok = v >= math.MinInt32 && v <= math.MaxInt32
	case types.Int64:
		ok = true
	case types.Uint, types.Uintptr:
		ok = v >= 0 && uint64(v) <= math.MaxUint
	case types.Uint8:
		ok = v >= 0 && v <= math.MaxUint8
	case types.Uint16:
		ok = v >= 0 && v <= math.MaxUint16
	case types.Uint32:
		ok = v >= 0 && v <= math.MaxUint32
	case types.Uint64:
		ok = v >= 0
	default:
		return fmt.Errorf("cannot use %s as %s", r.describeLiteral(lit), targetType)
	}
	if !ok {
		return fmt.Errorf("%s overflows %s", r.describeLiteral(lit), targetType)
	}
	return nil
}

// describeLiteral renders a literal for error messages.
func (r *argResolver) describeLiteral(lit di.Literal) string {
	switch lit.Kind {
	case di.LiteralString:
		return fmt.Sprintf("string literal %q", lit.String())
	case di.LiteralInt:
		return fmt.Sprintf("int literal %d", lit.Int())
	case di.LiteralFloat:
		return fmt.Sprintf("float literal %v", lit.Float())
	case di.LiteralBool:
		return fmt.Sprintf("bool literal %t", lit.Bool())
	case di.LiteralNull:
		return "null literal"
	default:
		return fmt.Sprintf("literal of kind %d", lit.Kind)
	}
}

// resolveFieldAccessOnService resolves !field:@service.Field.Chain.
// Uses longest prefix match to find the service ID, since service IDs can contain dots.
func (r *argResolver) resolveFieldAccessOnService(container *Container, resolveSvc func(string) error, svcID string, idx int, arg di.Argument, value string) (*FieldAccess, error) {
	parts := strings.Split(value, ".")
	if len(parts) < 2 {
		return nil, srcloc.Errorf(arg.SourceLoc, "service %q arg[%d]: !field:@%s requires at least one field name", svcID, idx, value)
	}

	// Longest prefix match: try progressively shorter prefixes
	var service *Service
	var fieldParts []string
	for i := len(parts) - 1; i >= 1; i-- {
		candidate := strings.Join(parts[:i], ".")
		if svc, ok := container.Services[candidate]; ok {
			service = svc
			fieldParts = parts[i:]
			break
		}
	}
	if service == nil {
		return nil, srcloc.Errorf(arg.SourceLoc, "service %q arg[%d]: !field:@%s: no matching service found", svcID, idx, value)
	}

	// The target may not be resolved yet (resolution order is not
	// dependency-aware); force it so service.Type is populated.
	if err := resolveSvc(service.ID); err != nil {
		return nil, srcloc.WrapError(arg.SourceLoc, fmt.Sprintf("service %q arg[%d]: !field:@%s", svcID, idx, value), err)
	}

	baseType := service.Type
	resultType, err := walkFieldChain(baseType, fieldParts)
	if err != nil {
		return nil, srcloc.WrapError(arg.SourceLoc, fmt.Sprintf("service %q arg[%d]: !field:@%s", svcID, idx, value), err)
	}

	return &FieldAccess{
		Service:    service,
		FieldNames: fieldParts,
		ResultType: resultType,
	}, nil
}

// resolveFieldAccessOnGoRef resolves !field:!go:pkg.Symbol.Field.Chain.
// Iteratively strips trailing dot-separated parts and tries LookupVar until
// the package-level symbol is found. The remainder becomes the field chain.
func (r *argResolver) resolveFieldAccessOnGoRef(svcID string, idx int, arg di.Argument, value string) (*FieldAccess, error) {
	parts := strings.Split(value, ".")
	if len(parts) < 3 {
		// Need at least pkg.Symbol.Field
		return nil, srcloc.Errorf(arg.SourceLoc, "service %q arg[%d]: !field:!go:%s requires at least one field name after the symbol", svcID, idx, value)
	}

	// Try progressively shorter prefixes to find the package-level symbol.
	// Minimum 2 parts for a valid qualified name (pkg.Name).
	var obj types.Object
	var fieldParts []string
	for i := len(parts) - 1; i >= 2; i-- {
		qualName := strings.Join(parts[:i], ".")
		pkgPath, name, _, err := typeres.SplitQualifiedNameWithTypeParams(qualName)
		if err != nil {
			continue
		}
		obj, err = r.typeResolver.LookupVar(pkgPath, name)
		if err != nil {
			continue
		}
		fieldParts = parts[i:]
		break
	}
	if obj == nil {
		return nil, srcloc.Errorf(arg.SourceLoc, "service %q arg[%d]: !field:!go:%s: no matching package-level symbol found", svcID, idx, value)
	}

	baseType := obj.Type()
	resultType, err := walkFieldChain(baseType, fieldParts)
	if err != nil {
		return nil, srcloc.WrapError(arg.SourceLoc, fmt.Sprintf("service %q arg[%d]: !field:!go:%s", svcID, idx, value), err)
	}

	return &FieldAccess{
		GoRef:      &GoRef{Object: obj},
		FieldNames: fieldParts,
		ResultType: resultType,
	}, nil
}

// walkFieldChain walks a chain of field names on a type, returning the final field's type.
// Pointer types are automatically dereferenced at each level.
func walkFieldChain(baseType types.Type, fieldNames []string) (types.Type, error) {
	if len(fieldNames) == 0 {
		return nil, fmt.Errorf("empty field chain")
	}

	currentType := baseType
	for _, fieldName := range fieldNames {
		// Dereference pointer types
		for {
			ptr, ok := currentType.(*types.Pointer)
			if !ok {
				break
			}
			currentType = ptr.Elem()
		}

		obj, _, _ := types.LookupFieldOrMethod(currentType, true, nil, fieldName)
		if obj == nil {
			return nil, fmt.Errorf("field %q not found on type %s", fieldName, currentType)
		}
		field, ok := obj.(*types.Var)
		if !ok || !field.IsField() {
			return nil, fmt.Errorf("%q is not a field on type %s", fieldName, currentType)
		}
		if !field.Exported() {
			return nil, fmt.Errorf("field %q on type %s is not exported", fieldName, currentType)
		}
		currentType = field.Type()
	}
	return currentType, nil
}
