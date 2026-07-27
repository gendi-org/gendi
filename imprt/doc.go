// Package imprt expands the entries of an imports: list into the concrete
// files they name.
//
// It covers the three addressing forms — a path relative to the importing
// file, a glob, and a path inside another Go module located through the
// go.mod graph — and applies the exclusions declared alongside them.
//
// Every resolver is created with a boundary, and an empty one is an error
// rather than a permissive default: candidates are resolved through symlinks
// and checked against that boundary so an import cannot quietly reach outside
// the module it belongs to. Resolution never depends on the process working
// directory, so the same configuration resolves the same way wherever the
// generator is invoked from.
package imprt
