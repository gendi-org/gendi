package imprt

// ExclusionError is an error of one exclusion pattern of an import. Index is
// the position of the pattern in the excludes that ResolveImport received, so
// that the caller can point at the pattern in its config.
type ExclusionError struct {
	Index int
	Err   error
}

func (e *ExclusionError) Error() string {
	return e.Err.Error()
}

func (e *ExclusionError) Unwrap() error {
	return e.Err
}
