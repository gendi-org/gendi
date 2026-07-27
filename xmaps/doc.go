// Package xmaps holds the map helpers that keep generation deterministic.
//
// Go randomises map iteration order, so iterating a configuration map directly
// would let that randomness reach generated code, error messages and test
// assertions, and two runs over the same configuration would stop producing
// identical files. OrderedKeys is how this repository iterates a map whose
// order can be observed anywhere downstream.
package xmaps
