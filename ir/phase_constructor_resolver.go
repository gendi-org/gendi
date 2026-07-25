package ir

import (
	"fmt"
	"go/types"
	"strings"

	di "github.com/gendi-org/gendi"
	"github.com/gendi-org/gendi/srcloc"
	"github.com/gendi-org/gendi/typeres"
)

// constructorResolverPhase resolves service constructors (functions, methods, and aliases)
type constructorResolverPhase struct {
	typeResolver TypeResolver
	argResolver  *argResolver
}

// Apply resolves all service constructors with circular reference detection
func (r *constructorResolverPhase) Apply(cfg *di.Config, container *Container) error {
	resolvingSvcs := make(map[string]bool)
	resolvedSvcs := make(map[string]bool)

	var resolveService func(id string) error
	resolveService = func(id string) error {
		if resolvedSvcs[id] {
			return nil
		}
		if resolvingSvcs[id] {
			cfgSvc := cfg.Services[id]
			return srcloc.Errorf(cfgSvc.SourceLoc, "circular constructor reference at %q", id)
		}
		resolvingSvcs[id] = true
		defer func() {
			resolvingSvcs[id] = false
			resolvedSvcs[id] = true
		}()

		svc := container.Services[id]
		cfgService, exists := cfg.Services[id]
		if !exists {
			return fmt.Errorf("service %q not found", id)
		}

		if cfgService.Alias != "" {
			return r.resolveAlias(container, svc, &cfgService, resolveService)
		}
		return r.resolveConstructor(container, svc, &cfgService, resolveService)
	}

	for _, id := range container.ServiceIDsPostOrder() {
		if err := resolveService(id); err != nil {
			return err
		}
	}
	return nil
}

// resolveAlias resolves an alias service
func (r *constructorResolverPhase) resolveAlias(container *Container, svc *Service, cfg *di.Service, resolve func(string) error) error {
	if cfg.Constructor.Func != "" || cfg.Constructor.Method != "" || len(cfg.Constructor.Args) > 0 {
		return srcloc.Errorf(cfg.SourceLoc, "service %q alias cannot define constructor", svc.ID)
	}
	if cfg.Decorates != "" {
		return srcloc.Errorf(cfg.SourceLoc, "service %q alias cannot be a decorator", svc.ID)
	}

	if err := resolve(cfg.Alias); err != nil {
		return srcloc.WrapError(cfg.SourceLoc, fmt.Sprintf("service %q alias target %q", svc.ID, cfg.Alias), err)
	}

	target, ok := container.Services[cfg.Alias]
	if !ok {
		return srcloc.Errorf(cfg.SourceLoc, "service %q alias target %q not found", svc.ID, cfg.Alias)
	}

	svc.Alias = target
	svc.Type = target.Type

	if cfg.Type != "" {
		declType, err := r.typeResolver.LookupType(cfg.Type)
		if err != nil {
			return srcloc.WrapError(cfg.SourceLoc, fmt.Sprintf("service %q type", svc.ID), err)
		}
		if !types.AssignableTo(svc.Type, declType) {
			return srcloc.Errorf(cfg.SourceLoc, "service %q type mismatch", svc.ID)
		}
	}
	return nil
}

// resolveConstructor resolves a function or method constructor
func (r *constructorResolverPhase) resolveConstructor(container *Container, svc *Service, cfg *di.Service, resolve func(string) error) error {
	cons := cfg.Constructor
	if cons.Func == "" && cons.Method == "" {
		return srcloc.Errorf(cfg.SourceLoc, "service %q missing constructor", svc.ID)
	}
	if cons.Func != "" && cons.Method != "" {
		return srcloc.Errorf(cfg.SourceLoc, "service %q has both func and method constructors", svc.ID)
	}

	var irCons *Constructor
	var err error

	if cons.Func != "" {
		irCons, err = r.resolveFuncConstructor(svc.ID, cons)
	} else {
		irCons, err = r.resolveMethodConstructor(container, svc.ID, cons, resolve)
	}
	if err != nil {
		return err
	}

	// Resolve arguments
	expectedMin := len(irCons.Params)
	if irCons.Variadic {
		expectedMin-- // Variadic parameter can accept 0 or more args
	}

	if !irCons.Variadic && len(cons.Args) != len(irCons.Params) {
		return srcloc.Errorf(cfg.SourceLoc, "service %q constructor args count mismatch: expected %d got %d",
			svc.ID, len(irCons.Params), len(cons.Args))
	}

	if irCons.Variadic && len(cons.Args) < expectedMin {
		return srcloc.Errorf(cfg.SourceLoc, "service %q constructor requires at least %d args, got %d",
			svc.ID, expectedMin, len(cons.Args))
	}

	irCons.Args = make([]*Argument, len(cons.Args))
	for i, arg := range cons.Args {
		// For variadic functions, all args after the last non-variadic param
		// use the variadic param's element type
		paramIdx := i
		if irCons.Variadic && i >= len(irCons.Params)-1 {
			paramIdx = len(irCons.Params) - 1
		}

		paramType := irCons.Params[paramIdx]
		// If this is a variadic parameter, get the slice element type
		// Exception: for spread arguments, keep the slice type
		if irCons.Variadic && paramIdx == len(irCons.Params)-1 && arg.Kind != di.ArgSpread {
			if sliceType, ok := paramType.(*types.Slice); ok {
				paramType = sliceType.Elem()
			}
		}

		irArg, err := r.argResolver.resolve(container, resolve, svc.ID, i, arg, paramType)
		if err != nil {
			return err
		}
		irCons.Args[i] = irArg
	}

	// Validate spread position
	if err := r.validateSpreadPosition(svc.ID, irCons, cons.Args); err != nil {
		return err
	}

	svc.Constructor = irCons
	svc.Type = irCons.ResultType

	if cfg.Type != "" {
		declType, err := r.typeResolver.LookupType(cfg.Type)
		if err != nil {
			return srcloc.WrapError(cfg.SourceLoc, fmt.Sprintf("service %q type", svc.ID), err)
		}
		if !types.AssignableTo(svc.Type, declType) {
			return srcloc.Errorf(cfg.SourceLoc, "service %q type mismatch", svc.ID)
		}
	}

	return nil
}

// validateConstructorSignature validates that a signature returns T or (T, error)
func (r *constructorResolverPhase) validateConstructorSignature(sig *types.Signature) (types.Type, bool, error) {
	res := sig.Results()
	if res.Len() == 0 || res.Len() > 2 {
		return nil, false, fmt.Errorf("constructor must return T or (T, error)")
	}
	resType := res.At(0).Type()
	// The empty interface is not a service type: it makes every argument
	// assignable, so nothing downstream could be validated statically.
	// Interfaces with methods stay valid — they are the usual contract type.
	if iface, ok := resType.Underlying().(*types.Interface); ok && iface.Empty() {
		return nil, false, fmt.Errorf("constructor must not return the empty interface (%s); a service needs a type the container can check statically", resType)
	}
	returnsErr := false
	if res.Len() == 2 {
		errType := res.At(1).Type()
		if !types.Identical(errType, types.Universe.Lookup("error").Type()) {
			return nil, false, fmt.Errorf("second return value must be error")
		}
		returnsErr = true
	}
	return resType, returnsErr, nil
}

// signatureParams extracts parameter types from a function signature
func (r *constructorResolverPhase) signatureParams(sig *types.Signature) []types.Type {
	params := make([]types.Type, sig.Params().Len())
	for i := 0; i < sig.Params().Len(); i++ {
		params[i] = sig.Params().At(i).Type()
	}
	return params
}

// resolveFuncConstructor resolves a function constructor
func (r *constructorResolverPhase) resolveFuncConstructor(id string, cons di.Constructor) (*Constructor, error) {
	pkgPath, name, typeParamStrs, err := typeres.SplitQualifiedNameWithTypeParams(cons.Func)
	if err != nil {
		return nil, srcloc.WrapError(cons.SourceLoc, fmt.Sprintf("service %q constructor.func", id), err)
	}

	fn, err := r.typeResolver.LookupFunc(pkgPath, name)
	if err != nil {
		return nil, srcloc.WrapError(cons.SourceLoc, fmt.Sprintf("service %q constructor.func", id), err)
	}

	var sig *types.Signature
	var typeArgs []types.Type

	rawSig := fn.Type().(*types.Signature)

	if len(typeParamStrs) > 0 {
		// Generic function - instantiate with type arguments
		sig, typeArgs, err = r.typeResolver.InstantiateFunc(fn, typeParamStrs)
		if err != nil {
			return nil, srcloc.WrapError(cons.SourceLoc, fmt.Sprintf("service %q constructor.func", id), err)
		}
	} else {
		// Check if function is generic but no type arguments provided
		if tp := rawSig.TypeParams(); tp != nil && tp.Len() > 0 {
			return nil, srcloc.Errorf(cons.SourceLoc, "service %q constructor.func: generic function %s requires %d type argument(s)",
				id, name, tp.Len())
		}
		sig = rawSig
	}

	resultType, returnsErr, err := r.validateConstructorSignature(sig)
	if err != nil {
		return nil, srcloc.WrapError(cons.SourceLoc, fmt.Sprintf("service %q constructor.func", id), err)
	}

	return &Constructor{
		Kind:         FuncConstructor,
		Func:         fn,
		TypeArgs:     typeArgs,
		Params:       r.signatureParams(sig),
		ResultType:   resultType,
		ReturnsError: returnsErr,
		Variadic:     sig.Variadic(),
	}, nil
}

// resolveMethodConstructor resolves a method constructor
func (r *constructorResolverPhase) resolveMethodConstructor(container *Container, id string, cons di.Constructor, resolve func(string) error) (*Constructor, error) {
	methodRef := cons.Method
	if !strings.HasPrefix(methodRef, "@") {
		return nil, srcloc.Errorf(cons.SourceLoc, "service %q constructor.method must start with @", id)
	}

	methodRef = methodRef[1:]
	parts := strings.Split(methodRef, ".")
	if len(parts) < 2 {
		return nil, srcloc.Errorf(cons.SourceLoc, "service %q constructor.method invalid format", id)
	}

	methodName := parts[len(parts)-1]
	recvID := strings.Join(parts[:len(parts)-1], ".")
	if recvID == "" || methodName == "" {
		return nil, srcloc.Errorf(cons.SourceLoc, "service %q constructor.method invalid format", id)
	}

	if err := resolve(recvID); err != nil {
		return nil, err
	}

	recvSvc, ok := container.Services[recvID]
	if !ok {
		return nil, srcloc.Errorf(cons.SourceLoc, "service %q constructor.method unknown receiver service %q", id, recvID)
	}

	meth, err := r.typeResolver.LookupMethod(recvSvc.Type, methodName)
	if err != nil {
		return nil, srcloc.WrapError(cons.SourceLoc, fmt.Sprintf("service %q constructor.method", id), err)
	}

	sig := meth.Type().(*types.Signature)
	resultType, returnsErr, err := r.validateConstructorSignature(sig)
	if err != nil {
		return nil, srcloc.WrapError(cons.SourceLoc, fmt.Sprintf("service %q constructor.method", id), err)
	}

	return &Constructor{
		Kind:         MethodConstructor,
		Func:         meth,
		Receiver:     recvSvc,
		Params:       r.signatureParams(sig),
		ResultType:   resultType,
		ReturnsError: returnsErr,
		Variadic:     sig.Variadic(),
	}, nil
}

// validateSpreadPosition validates that spread arguments follow the rules:
// 1. Spread requires a variadic constructor
// 2. Only one spread is allowed per constructor call
// 3. Spread must be the last argument and fill the variadic slot alone
func (r *constructorResolverPhase) validateSpreadPosition(svcID string, cons *Constructor, diArgs []di.Argument) error {
	// Find all spread arguments
	spreadCount := 0
	lastSpreadIdx := -1
	for i, arg := range cons.Args {
		if arg.Kind == SpreadArg {
			spreadCount++
			lastSpreadIdx = i
		}
	}

	if spreadCount == 0 {
		return nil // No spread, nothing to check
	}

	// A spread into a plain slice parameter would generate invalid Go.
	if !cons.Variadic {
		return srcloc.Errorf(diArgs[lastSpreadIdx].SourceLoc,
			"service %q: !spread: can only be used with variadic parameters", svcID)
	}

	// Check that only one spread is present
	if spreadCount > 1 {
		// Find the second spread arg's location for error reporting from diArgs
		var loc *srcloc.Location
		count := 0
		for i, arg := range cons.Args {
			if arg.Kind == SpreadArg {
				count++
				if count == 2 {
					loc = diArgs[i].SourceLoc
					break
				}
			}
		}
		return srcloc.Errorf(loc, "service %q: !spread: only one spread allowed per constructor call", svcID)
	}

	// Check that spread is the last argument
	if lastSpreadIdx != len(cons.Args)-1 {
		return srcloc.Errorf(diArgs[lastSpreadIdx].SourceLoc, "service %q: !spread: must be the last argument", svcID)
	}

	// Go does not allow mixing positional variadic values with a spread, so
	// the spread must occupy the variadic slot alone.
	if lastSpreadIdx != len(cons.Params)-1 {
		return srcloc.Errorf(diArgs[lastSpreadIdx].SourceLoc,
			"service %q: !spread: cannot be mixed with positional variadic values", svcID)
	}

	return nil
}
