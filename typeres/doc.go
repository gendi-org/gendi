// Package typeres loads and type-checks the Go packages a configuration
// refers to, and answers the questions the rest of the generator asks about
// them: whether a symbol exists, what a constructor returns, and whether an
// argument is assignable to the parameter it is wired into.
//
// It is the boundary where a configuration stops being text and starts being
// checked against real code. Everything it rejects would otherwise have become
// a compile error in generated code — or, for parameter conversions, a runtime
// failure — which is why the checks live before generation rather than after.
package typeres
