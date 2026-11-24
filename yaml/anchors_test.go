package yaml

import (
	"errors"
	"fmt"
	"testing"

	"github.com/gendi-org/gendi/srcloc"
)

func TestAnchorExpander(t *testing.T) {
	tests := []struct {
		name string
		yaml string
		want string // summary of service "b"
	}{
		{
			name: "merge alias into service",
			yaml: `
services:
  a: &s {constructor: {func: pkg.New}, public: true}
  b: {<<: *s, shared: false}
`,
			want: "pkg.New public=true shared=false args=[]",
		},
		{
			name: "explicit key overrides merged key",
			yaml: `
services:
  a: &s {constructor: {func: pkg.New}, public: true}
  b: {public: false, <<: *s}
`,
			want: "pkg.New public=false shared=true args=[]",
		},
		{
			name: "first merged mapping wins",
			yaml: `
services:
  one: &one {constructor: {func: pkg.One}, public: true}
  two: &two {constructor: {func: pkg.Two}, shared: false}
  b: {<<: [*one, *two]}
`,
			want: "pkg.One public=true shared=false args=[]",
		},
		{
			name: "inline merge",
			yaml: `
services:
  b: {<<: {constructor: {func: pkg.New}}, public: true}
`,
			want: "pkg.New public=true shared=true args=[]",
		},
		{
			name: "alias as service",
			yaml: `
services:
  a: &s {constructor: {func: pkg.New}, public: true}
  b: *s
`,
			want: "pkg.New public=true shared=true args=[]",
		},
		{
			name: "alias as constructor argument",
			yaml: `
services:
  dep: {constructor: {func: pkg.Dep}}
  a: {constructor: {func: pkg.New, args: [&arg "@dep"]}}
  b: {constructor: {func: pkg.New, args: [*arg]}}
`,
			want: "pkg.New public=false shared=true args=[dep]",
		},
		{
			name: "alias refers to the last anchor before it",
			yaml: `
services:
  a: {constructor: &c {func: pkg.Old}}
  b: {constructor: *c}
  x: {constructor: &c {func: pkg.New}}
`,
			want: "pkg.Old public=false shared=true args=[]",
		},
		{
			name: "anchor value refers to an earlier anchor with the same name",
			yaml: `
services:
  a: &s {constructor: {func: pkg.New}}
  x: &s {<<: *s, public: true}
  b: *s
`,
			want: "pkg.New public=true shared=true args=[]",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path := writeYAML(t, tt.yaml)
			boundary := boundaryFor(t, path)
			cfg, err := LoadConfig(path, boundary, boundary)
			if err != nil {
				t.Fatal(err)
			}
			svc, ok := cfg.Services["b"]
			if !ok {
				t.Fatal(`service "b" not found`)
			}
			args := make([]string, len(svc.Constructor.Args))
			for i, arg := range svc.Constructor.Args {
				args[i] = arg.Value
			}
			got := fmt.Sprintf("%s public=%t shared=%t args=%v", svc.Constructor.Func, svc.Public, svc.Shared, args)
			if got != tt.want {
				t.Errorf("service b = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestAnchorExpander_Errors(t *testing.T) {
	tests := []struct {
		name   string
		yaml   string
		line   int
		column int
		msg    string
	}{
		{
			name:   "merge of a scalar",
			yaml:   "services:\n  b: {<<: pkg.New}\n",
			line:   2,
			column: 11,
			msg:    "merge value must be a mapping or a sequence of mappings",
		},
		{
			name:   "merge of a scalar in a sequence",
			yaml:   "services:\n  a: &s {constructor: {func: pkg.New}}\n  b: {<<: [*s, pkg.New]}\n",
			line:   3,
			column: 16,
			msg:    "merge value must be a mapping or a sequence of mappings",
		},
		{
			name:   "anchor value refers to itself",
			yaml:   "services:\n  b: &s {<<: *s}\n",
			line:   2,
			column: 14,
			msg:    `unknown anchor "s"`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path := writeYAML(t, tt.yaml)
			boundary := boundaryFor(t, path)
			_, err := LoadConfig(path, boundary, boundary)
			var locErr *srcloc.Error
			if !errors.As(err, &locErr) || locErr.Loc == nil {
				t.Fatalf("expected located error, got %v", err)
			}
			want := *srcloc.NewLocation(path, tt.line, tt.column)
			if *locErr.Loc != want {
				t.Errorf("location = %v, want %v", locErr.Loc, &want)
			}
			if locErr.Message != tt.msg {
				t.Errorf("message = %q, want %q", locErr.Message, tt.msg)
			}
		})
	}
}
