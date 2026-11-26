package ir

// TagShape is the form in which a consumer receives a tagged collection. The
// type of the target parameter selects it.
type TagShape int

const (
	// TagShapeSlice is []T, in collection order.
	TagShapeSlice TagShape = iota
	// TagShapeMap is map[string]T, from the key of each service.
	TagShapeMap
	// TagShapeSeq2 is iter.Seq2[string, T]: key and service, in collection
	// order.
	TagShapeSeq2
)

// Keyed reports whether the shape exposes the keys of the services.
func (s TagShape) Keyed() bool {
	return s != TagShapeSlice
}
