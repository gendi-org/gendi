package stdlib

import (
	"slices"
	"testing"
)

func TestMakeSeq2(t *testing.T) {
	for _, tt := range []struct {
		name     string
		keys     []string
		values   []int
		stopAt   int // stop the iteration after this many pairs; 0 means never
		wantKeys []string
		wantVals []int
	}{
		{name: "empty"},
		{
			name:     "order",
			keys:     []string{"b", "a", "c"},
			values:   []int{2, 1, 3},
			wantKeys: []string{"b", "a", "c"},
			wantVals: []int{2, 1, 3},
		},
		{
			name:     "early stop",
			keys:     []string{"b", "a", "c"},
			values:   []int{2, 1, 3},
			stopAt:   1,
			wantKeys: []string{"b"},
			wantVals: []int{2},
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			seq := MakeSeq2(tt.keys, tt.values...)
			// Iterate two times: the sequence must be reusable.
			for range 2 {
				var gotKeys []string
				var gotVals []int
				for k, v := range seq {
					gotKeys = append(gotKeys, k)
					gotVals = append(gotVals, v)
					if tt.stopAt != 0 && len(gotKeys) == tt.stopAt {
						break
					}
				}
				if !slices.Equal(gotKeys, tt.wantKeys) || !slices.Equal(gotVals, tt.wantVals) {
					t.Errorf("MakeSeq2() yields %v %v, want %v %v", gotKeys, gotVals, tt.wantKeys, tt.wantVals)
				}
			}
		})
	}
}
