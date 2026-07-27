// Command gendi generates a dependency injection container in Go from a YAML
// configuration.
//
// It is the stock generator: the compiler passes shipped with GenDI and
// nothing else. A project that needs passes of its own builds an equivalent
// binary around [github.com/gendi-org/gendi/cmd.Run] and keeps the same flags
// and diagnostics.
//
// Install it as a tool dependency and run it through go generate:
//
//	go get -tool github.com/gendi-org/gendi/cmd/gendi
//	go tool gendi --config=gendi.yaml --out=./di --pkg=di
//
// The flags are documented at https://gendi.dev/docs/cli/
package main
