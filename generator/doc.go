// Package generator renders the Go source of a container from the resolved
// intermediate representation.
//
// Everything here is about output: managing and aliasing imports, choosing
// identifiers that cannot collide with the user's own packages, emitting the
// build and getter functions, and inlining the few standard-library factories
// the generator knows by name so the generated file does not import them.
//
// It performs no analysis. By the time it runs, every type has been resolved
// and every configuration error already reported, so a failure here is a bug
// in the generator rather than a problem with the configuration.
//
// Output is deterministic: two runs over the same input produce byte-identical
// source, which is what makes a generated file reviewable as an ordinary diff.
package generator
