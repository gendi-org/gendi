// Package srcloc carries the position of a configuration node from the parser
// into the errors reported about it, and renders those errors with the
// offending line and a caret under the token.
//
// Configuration errors are expected to arrive here with a Location. One raised
// without a location still reports its message, but silently loses the context
// that makes a generation failure obvious to read — which is why the location
// is threaded through the configuration model rather than looked up after the
// fact.
package srcloc
