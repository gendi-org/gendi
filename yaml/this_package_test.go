package yaml

import (
	"errors"
	"strings"
	"testing"

	"github.com/gendi-org/gendi/srcloc"
)

// TestThisWithoutGoModule uses loadConfigString, which writes the config to a
// temporary directory without a go.mod. $this then has no package.
func TestThisWithoutGoModule(t *testing.T) {
	const reason = "resolve $this: no go.mod found in "

	tests := []struct {
		name     string
		yaml     string
		wantLine int
		wantMsg  string
	}{
		{
			name: "service type",
			yaml: `services:
  svc:
    type: "*$this.T"
    constructor:
      func: "strings.NewReplacer"
`,
			wantLine: 3,
			wantMsg:  `service "svc": type: ` + reason,
		},
		{
			name: "constructor func",
			yaml: `services:
  svc:
    constructor:
      func: "$this.New"
`,
			wantLine: 4,
			wantMsg:  `service "svc": constructor.func: ` + reason,
		},
		{
			name: "go reference argument",
			yaml: `services:
  svc:
    constructor:
      func: "strings.NewReplacer"
      args:
        - "!go:$this.Default"
`,
			wantLine: 6,
			wantMsg:  `service "svc": arg[0]: ` + reason,
		},
		{
			name: "field access argument",
			yaml: `services:
  svc:
    constructor:
      func: "strings.NewReplacer"
      args:
        - "x"
        - "!field:!go:$this.Config.Host"
`,
			wantLine: 7,
			wantMsg:  `service "svc": arg[1]: ` + reason,
		},
		{
			name: "tag element type",
			yaml: `tags:
  h:
    element_type: "$this.Handler"
`,
			wantLine: 3,
			wantMsg:  `tag "h": element_type: ` + reason,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := loadConfigString(t, tt.yaml)
			if err == nil {
				t.Fatal("expected error, got nil")
			}
			if !strings.Contains(err.Error(), tt.wantMsg) {
				t.Fatalf("error = %q, want it to contain %q", err.Error(), tt.wantMsg)
			}
			var locErr *srcloc.Error
			if !errors.As(err, &locErr) || locErr.Loc == nil {
				t.Fatalf("error has no location: %v", err)
			}
			if locErr.Loc.Line != tt.wantLine {
				t.Errorf("line = %d, want %d", locErr.Loc.Line, tt.wantLine)
			}
		})
	}
}

func TestNoThisWithoutGoModule(t *testing.T) {
	_, err := loadConfigString(t, `tags:
  h:
    element_type: "io.Reader"
services:
  svc:
    type: "*strings.Replacer"
    constructor:
      func: "strings.NewReplacer"
      args:
        - "!go:os.Stdout"
`)
	if err != nil {
		t.Fatalf("a config without $this needs no go.mod: %v", err)
	}
}
