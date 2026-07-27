// Package pipeline runs a configuration through to generated Go source:
// internal passes, loading of the Go packages it refers to, construction of
// the intermediate representation, then rendering.
//
// Build stops at the intermediate representation and Emit continues to source.
// Neither modifies the caller's configuration — passes operate on a clone —
// and both require Options.Finalize to have been called first.
//
// The phase order lives in build.go as an ordered sequence of calls. It is the
// specification of what happens when; read it rather than a summary of it.
package pipeline
