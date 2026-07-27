// Package cmd is the command-line front end of the generator: flag
// definitions, pass selection, and the load-passes-emit-write sequence that a
// generator binary performs.
//
// It is a library rather than an internal detail so that a project can build
// its own generator with its own compiler passes registered, and still get the
// same flags, the same validation of pass names before generation starts, and
// the same error rendering as the stock binary.
//
// See https://gendi.dev/docs/embedding/ for what such a binary looks like.
package cmd
