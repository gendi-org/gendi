package yaml

import (
	"strings"

	"github.com/gendi-org/gendi/srcloc"
)

// thisPackage is the Go package of the directory of a config file. The $this
// token in the config resolves to it.
type thisPackage struct {
	path string
	// err tells why path is unknown.
	err error
}

// check returns an error when value uses $this and the package is unknown.
// Without this error, $this stays in the value and the package loader later
// fails on the import path "$this" without a location.
func (t thisPackage) check(value string, loc *srcloc.Location, field string) error {
	if t.err == nil || !strings.Contains(value, "$this.") {
		return nil
	}
	return srcloc.Errorf(loc, "%s: resolve $this: %v", field, t.err)
}
