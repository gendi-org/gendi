// Package parameters is the only package a generated container depends on at
// run time.
//
// A container holds no parameter values of its own. It reads them through a
// Provider and converts them through a Caster, both supplied by the
// application, which is what makes the values a program runs with independent
// of the defaults written in the configuration file.
//
// Provider and Caster are the extension points and are kept small on purpose.
// Resolver is deliberately a concrete facade over the two rather than an
// interface: generated code should have exactly one way to ask for a typed
// value, and no reason to substitute the mechanism that answers.
package parameters
