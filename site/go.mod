// The documentation site is a module of its own so that the Hugo theme never
// reaches the go.mod of gendi itself — people install gendi with `go get -tool`
// and its dependency graph stays clean.
module github.com/gendi-org/gendi-docs

go 1.26.0

require github.com/imfing/hextra v0.12.3 // indirect
