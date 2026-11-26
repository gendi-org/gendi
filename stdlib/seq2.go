package stdlib

import (
	"fmt"
	"iter"
)

// MakeSeq2 returns a sequence that yields the pairs (keys[i], values[i]) in
// order. It is the constructor of a tagged collection in the sequence shape.
//
// The sequence holds the values that it got, so it can be iterated more than
// one time. MakeSeq2 panics when the lengths differ: the generator makes both
// slices from one list, so a difference is a generator defect.
func MakeSeq2[V any](keys []string, values ...V) iter.Seq2[string, V] {
	if len(keys) != len(values) {
		panic(fmt.Sprintf("stdlib.MakeSeq2: %d keys, %d values", len(keys), len(values)))
	}
	return func(yield func(string, V) bool) {
		for i, key := range keys {
			if !yield(key, values[i]) {
				return
			}
		}
	}
}
