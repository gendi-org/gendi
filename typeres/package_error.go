package typeres

import "fmt"

// PackageError reports a package that the resolver cannot load. Path is the
// import path that the caller requested, so the caller can find the place in
// its config that references the package.
type PackageError struct {
	Path string
	Err  error
}

func (e *PackageError) Error() string {
	return fmt.Sprintf("load package %s: %v", e.Path, e.Err)
}

func (e *PackageError) Unwrap() error {
	return e.Err
}
