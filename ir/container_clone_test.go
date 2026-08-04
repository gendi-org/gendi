package ir

import "testing"

func TestConstructorCloneIsolatesMapEntries(t *testing.T) {
	inner := &Argument{Kind: LiteralArg}
	constructor := Constructor{
		Args: []*Argument{{
			Kind: MapArg,
			Entries: []MapEntry{{
				Key:   LiteralValue{Type: StringLiteral, Value: "/"},
				Value: inner,
			}},
		}},
	}

	clone := constructor.Clone()

	// Mutate through the clone's entry pointer
	clone.Args[0].Entries[0].Value.Kind = ServiceRefArg

	// Original's entry Value should be unaffected
	got := constructor.Args[0].Entries[0].Value.Kind
	if got != LiteralArg {
		t.Fatalf("original entry value Kind = %v, want %v: Clone shares the entry Value pointer", got, LiteralArg)
	}

	// Also verify the entries slice itself is cloned
	clone.Args[0].Entries[0].Key.Value = "different"
	original := constructor.Args[0].Entries[0].Key.Value
	if original != "/" {
		t.Fatalf("original entry key mutated, Clone shares the entries slice")
	}
}
