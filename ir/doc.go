// Package ir is the intermediate representation between a loaded
// configuration and generated code: services whose dependencies point at real
// Go types, arguments classified by what they inject, tags desugared into
// collections and decorators expanded into chains.
//
// This is where the generation-time guarantees are enforced. Unknown services,
// arguments that do not match a constructor's signature, circular references,
// service types that are the empty interface, and parameter defaults that
// cannot be converted to their target type are all rejected here, each with
// the source location of the configuration node that caused it.
//
// Builder applies the phases in a fixed order, and several of them only make
// sense after another has run. That order is in builder.go; it is the
// specification, not an implementation detail to be summarised elsewhere.
package ir
