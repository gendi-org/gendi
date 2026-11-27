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

// InlinerMakeSeq2 emits a function literal for stdlib.MakeSeq2. The values
// are variables that the build function assigned before the call, so the
// iteration does not build services and cannot fail.
type InlinerMakeSeq2 struct {
	importManager *ImportManager
}

func NewInlinerMakeSeq2(importManager *ImportManager) *InlinerMakeSeq2 {
	return &InlinerMakeSeq2{importManager: importManager}
}

func (i *InlinerMakeSeq2) TryInline(cons constructorDef, args []string) (bool, string) {
	if !isStdlibCall(cons, "MakeSeq2") || len(cons.typeArgs) != 1 {
		return false, ""
	}
	keys, values, ok := i.keyedArgs(cons, args)
	if !ok {
		return false, ""
	}

	var b strings.Builder
	fmt.Fprintf(&b, "func(yield func(string, %s) bool) {", i.importManager.typeString(cons.typeArgs[0]))
	for idx, key := range keys {
		fmt.Fprintf(&b, "\nif !yield(%q, %s) {\nreturn\n}", key, values[idx])
	}
	if len(keys) > 0 {
		b.WriteString("\n")
	}
	b.WriteString("}")

	return true, b.String()
}

// keyedArgs splits the arguments of MakeSeq2 into the keys and the value
// expressions.
func (i *InlinerMakeSeq2) keyedArgs(cons constructorDef, args []string) ([]string, []string, bool) {
	if len(cons.argDefs) == 0 || len(args) != len(cons.argDefs) {
		return nil, nil, false
	}
	keys, ok := cons.argDefs[0].Literal.Value.([]string)
	if !ok || cons.argDefs[0].Literal.Type != ir.StringsLiteral || len(keys) != len(args)-1 {
		return nil, nil, false
	}
	return keys, args[1:], true
}
