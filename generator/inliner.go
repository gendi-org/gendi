package generator

import (
	"fmt"
	"strings"

	"github.com/gendi-org/gendi/ir"
)

type Inliner interface {
	TryInline(cons constructorDef, args []string) (bool, string)
}

type InlinerComposite struct {
	inliners []Inliner
}

func NewInlinerComposite(inliners ...Inliner) *InlinerComposite {
	return &InlinerComposite{inliners: inliners}
}

func (i *InlinerComposite) TryInline(cons constructorDef, args []string) (bool, string) {
	for _, inliner := range i.inliners {
		ok, result := inliner.TryInline(cons, args)
		if ok {
			return true, result
		}
	}

	return false, ""
}

func isStdlibCall(cons constructorDef, funcName string) bool {
	if cons.kind != "func" || cons.funcObj == nil || cons.funcObj.Name() != funcName {
		return false
	}

	pkg := cons.funcObj.Pkg()
	return pkg != nil && pkg.Path() == ir.StdlibPkgPath
}

type InlinerMakeSlice struct {
	importManager *ImportManager
}

func NewInlinerMakeSlice(importManager *ImportManager) *InlinerMakeSlice {
	return &InlinerMakeSlice{importManager: importManager}
}

func (i *InlinerMakeSlice) TryInline(cons constructorDef, args []string) (bool, string) {
	if !isStdlibCall(cons, "MakeSlice") {
		return false, ""
	}

	resultType := i.importManager.typeString(cons.result)

	return true, fmt.Sprintf("%s{%s}", resultType, strings.Join(args, ", "))
}

type InlinerMakeChan struct {
	importManager *ImportManager
}

func NewInlinerMakeChan(importManager *ImportManager) *InlinerMakeChan {
	return &InlinerMakeChan{importManager: importManager}
}

func (i *InlinerMakeChan) TryInline(cons constructorDef, args []string) (bool, string) {
	if !isStdlibCall(cons, "NewChan") || len(args) != 1 {
		return false, ""
	}

	resultType := i.importManager.typeString(cons.result)

	return true, fmt.Sprintf("make(%s, %s)", resultType, args[0])
}
