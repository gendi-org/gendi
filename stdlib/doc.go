// Package stdlib provides constructors for standard library types that are
// tedious to wire by hand — HTTP clients and transports, slog handlers and
// writers — together with the gendi.yaml that declares them as services.
//
// That file, not this package, is the surface a configuration uses: importing
// it is what makes @stdlib.http.client and @stdlib.logger available. It is
// also GenDI's own instance of a library shipping its wiring as data, and the
// worked example behind https://gendi.dev/docs/library-wiring/.
//
// MakeSlice and NewChan are different in kind. The generator recognises them
// and emits the equivalent literal or make(...) expression instead of a call,
// so a container that uses tagged collections or channels does not import this
// package at run time.
//
// SLogPass lives here rather than with the other passes because it rewrites
// services in terms of these factories.
package stdlib
