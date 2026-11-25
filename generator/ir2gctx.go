package generator

import (
	"github.com/gendi-org/gendi/ir"
	"github.com/gendi-org/gendi/typeres"
)

// IrConverter builds the generation context using the IR layer.
type IrConverter struct {
	typeResolver *typeres.Resolver
}

func NewIRConverter(typeResolver *typeres.Resolver) *IrConverter {
	return &IrConverter{
		typeResolver: typeResolver,
	}
}

func (b *IrConverter) convertConstructor(irCons *ir.Constructor) constructorDef {
	result := constructorDef{
		argDefs:      irCons.Args,
		typeArgs:     irCons.TypeArgs,
		params:       irCons.Params,
		result:       irCons.ResultType,
		returnsError: irCons.ReturnsError,
	}

	result.funcObj = irCons.Func

	if irCons.Kind == ir.FuncConstructor {
		result.kind = "func"
		return result
	}

	result.kind = "method"
	if irCons.Receiver != nil {
		result.methodRecvID = irCons.Receiver.ID
	}

	return result
}

func (b *IrConverter) convertService(irSvc *ir.Service) *serviceDef {
	svcDef := &serviceDef{
		id:       irSvc.ID,
		typeName: irSvc.Type,
		public:   irSvc.Public,
		shared:   irSvc.Shared,
	}

	if irSvc.IsAlias() {
		svcDef.aliasTarget = irSvc.Alias.ID
	}

	if irSvc.Constructor != nil {
		svcDef.constructor = b.convertConstructor(irSvc.Constructor)
	}

	return svcDef
}

func (b *IrConverter) Convert(irContainer *ir.Container) (*GenContext, error) {
	services := make(map[string]*serviceDef)
	// Convert IR services to serviceDef
	for id, irSvc := range irContainer.Services {
		svcDef := b.convertService(irSvc)
		services[id] = svcDef
	}

	ctx := &GenContext{
		services:          services,
		orderedServiceIDs: irContainer.ServiceIDsPostOrder(),
	}

	return ctx, nil
}
