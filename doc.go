// Package di is the configuration model GenDI compiles: services, parameters,
// tags, and the arguments that connect them, as they exist after a
// configuration file has been loaded and before anything Go-specific has been
// resolved.
//
// The model is deliberately inert — it reads no files, resolves no types and
// generates nothing — so that a compiler pass can rewrite an entire
// configuration with ordinary Go code and hand it back. Pass is that extension
// point.
//
// Every node carries an optional source location. Preserving it when a pass
// rewrites an entry is what keeps the offending configuration line in the
// errors reported later; dropping it silently degrades that reporting to a
// bare message.
package di
