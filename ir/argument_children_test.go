package ir

import "testing"

func TestArgumentChildren(t *testing.T) {
	inner := &Argument{Kind: ServiceRefArg}
	second := &Argument{Kind: ParamRefArg}

	for _, tt := range []struct {
		name string
		arg  *Argument
		want []*Argument
	}{
		{name: "nil argument", arg: nil, want: nil},
		{name: "literal has no children", arg: &Argument{Kind: LiteralArg}, want: nil},
		{name: "service ref has no children", arg: &Argument{Kind: ServiceRefArg}, want: nil},
		{name: "spread yields inner", arg: &Argument{Kind: SpreadArg, Inner: inner}, want: []*Argument{inner}},
		{name: "spread without inner", arg: &Argument{Kind: SpreadArg}, want: nil},
		{
			name: "map yields entry values in order",
			arg: &Argument{Kind: MapArg, Entries: []MapEntry{
				{Key: LiteralValue{Type: StringLiteral, Value: "a"}, Value: inner},
				{Key: LiteralValue{Type: StringLiteral, Value: "b"}, Value: second},
			}},
			want: []*Argument{inner, second},
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.arg.children()
			if len(got) != len(tt.want) {
				t.Fatalf("children() = %d items, want %d", len(got), len(tt.want))
			}
			for i := range tt.want {
				if got[i] != tt.want[i] {
					t.Errorf("children()[%d] = %p, want %p", i, got[i], tt.want[i])
				}
			}
		})
	}
}
