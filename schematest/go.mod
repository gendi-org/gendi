// The tests of gendi.schema.json are a module of their own so that the JSON
// Schema validator never reaches the go.mod of gendi itself: gendi is often
// embedded, and its dependency graph stays clean.
module github.com/gendi-org/gendi/schematest

go 1.26.0

require (
	github.com/gendi-org/gendi v0.0.0-00010101000000-000000000000
	github.com/goccy/go-yaml v1.19.2
	github.com/santhosh-tekuri/jsonschema/v6 v6.0.3
)

require (
	github.com/bmatcuk/doublestar/v4 v4.10.7 // indirect
	golang.org/x/mod v0.43.0 // indirect
	golang.org/x/sync v0.25.0 // indirect
	golang.org/x/text v0.14.0 // indirect
	golang.org/x/tools v0.53.0 // indirect
)

replace github.com/gendi-org/gendi => ../
