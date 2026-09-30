package generator

import (
	"bytes"
	"fmt"
	"slices"

	"github.com/gendi-org/gendi/ir"
)

// buildFunctionRenderer renders a build function for a service.
type buildFunctionRenderer interface {
	buildSignature(rnd *ContainerRenderer, svc *serviceDef) (signature string)
	render(b *bytes.Buffer, rnd *ContainerRenderer, ctx *GenContext, svc *serviceDef) error
}

// selectBuildRenderer chooses the appropriate renderer based on service properties.
func selectBuildRenderer(svc *serviceDef) buildFunctionRenderer {
	return &regularBuildRenderer{}
}

func argNeedsErrorHandling(arg *ir.Argument) bool {
	if arg == nil {
		return false
	}

	switch arg.Kind {
	case ir.ServiceRefArg, ir.TaggedArg, ir.ParamRefArg:
		return true
	case ir.FieldAccessArg:
		if arg.FieldAccess != nil && arg.FieldAccess.Service != nil {
			return true
		}
	}
	for _, child := range arg.Children() {
		if argNeedsErrorHandling(child) {
			return true
		}
	}
	return false
}

func buildNeedsErrorHandling(svc *serviceDef) bool {
	if svc.constructor.returnsError || svc.constructor.kind == "method" {
		return true
	}
	return slices.ContainsFunc(svc.constructor.argDefs, argNeedsErrorHandling)
}

// regularBuildRenderer renders a standard build function.
type regularBuildRenderer struct{}

func (r *regularBuildRenderer) buildSignature(rnd *ContainerRenderer, svc *serviceDef) string {
	name := rnd.identGenerator.Build(svc.id)
	retType := rnd.importManager.typeString(svc.typeName)
	signature := fmt.Sprintf("func (c *%s) %s() (%s, error)", rnd.containerName, name, retType)
	return signature
}

func (r *regularBuildRenderer) render(b *bytes.Buffer, rnd *ContainerRenderer, ctx *GenContext, svc *serviceDef) error {
	signature := r.buildSignature(rnd, svc)
	retType := rnd.importManager.typeString(svc.typeName)
	returnsErr := buildNeedsErrorHandling(svc)

	fmt.Fprintf(b, "%s {\n", signature)
	if returnsErr {
		fmt.Fprintf(b, "\tvar zero %s\n", retType)
	}

	stmts, callExpr, err := rnd.constructorCall(ctx, svc, returnsErr)
	if err != nil {
		return err
	}
	for _, stmt := range stmts {
		fmt.Fprintf(b, "\t%s\n", stmt)
	}
	if svc.constructor.returnsError {
		fmt.Fprintf(b, "\tres, err := %s\n", callExpr)
		fmt.Fprintf(b, "\t%s\n", serviceConstructorError(rnd.importManager, svc.id))
		fmt.Fprintf(b, "\treturn res, nil\n")
	} else {
		fmt.Fprintf(b, "\treturn %s, nil\n", callExpr)
	}
	b.WriteString("}\n\n")
	return nil
}
