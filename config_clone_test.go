package di

import "testing"

func TestCloneIsolatesMapArgumentEntries(t *testing.T) {
	cfg := NewConfig()
	cfg.Services["router"] = Service{
		Constructor: Constructor{
			Func: "app.NewRouter",
			Args: []Argument{{
				Kind: ArgMap,
				Entries: []ArgEntry{{
					Key:   NewStringLiteral("/"),
					Value: Argument{Kind: ArgServiceRef, Value: "handler.home"},
				}},
			}},
		},
	}

	clone := cfg.Clone()
	clone.Services["router"].Constructor.Args[0].Entries[0].Value.Value = "handler.other"

	got := cfg.Services["router"].Constructor.Args[0].Entries[0].Value.Value
	if got != "handler.home" {
		t.Fatalf("original entry value = %q, want %q: Clone shares the entries slice", got, "handler.home")
	}
}

func TestArgumentChildrenExposesEntryValues(t *testing.T) {
	arg := Argument{
		Kind: ArgMap,
		Entries: []ArgEntry{
			{Key: NewStringLiteral("a"), Value: Argument{Kind: ArgServiceRef, Value: "a"}},
			{Key: NewStringLiteral("b"), Value: Argument{Kind: ArgParam, Value: "b"}},
		},
	}

	children := arg.Children()
	if len(children) != 2 {
		t.Fatalf("Children() = %d items, want 2", len(children))
	}
	children[0].Value = "rewritten"
	if arg.Entries[0].Value.Value != "rewritten" {
		t.Error("Children() must return pointers into Entries so passes can rewrite them")
	}
	if got := (&Argument{Kind: ArgLiteral}).Children(); got != nil {
		t.Errorf("Children() on a literal = %v, want nil", got)
	}
}
