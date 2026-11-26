package pipeline

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	di "github.com/gendi-org/gendi"
	"github.com/gendi-org/gendi/internal/testmodule"
	"github.com/gendi-org/gendi/srcloc"
)

func TestBuildLocatesPackageErrors(t *testing.T) {
	loc := func(line int) *srcloc.Location {
		return &srcloc.Location{File: "gendi.yaml", Line: line, Column: 5}
	}
	missing := func(path string) string {
		return fmt.Sprintf("load package %s: no required module provides package %s;", path, path)
	}
	replacer := di.Constructor{Func: "strings.NewReplacer", SourceLoc: loc(2)}

	tests := []struct {
		name string
		cfg  *di.Config
		// noGendi makes a module that does not require gendi, so the
		// stdlib package of tagged collections does not load.
		noGendi bool
		want    []string
	}{
		{
			name: "service type",
			cfg: &di.Config{Services: map[string]di.Service{
				"svc": {Type: "*example.com/nope.T", Constructor: replacer, Public: true, SourceLoc: loc(1)},
			}},
			want: []string{`gendi.yaml:1:5: service "svc" type: ` + missing("example.com/nope")},
		},
		{
			name: "constructor func",
			cfg: &di.Config{Services: map[string]di.Service{
				"svc": {Constructor: di.Constructor{Func: "example.com/nope.New", SourceLoc: loc(2)}, Public: true, SourceLoc: loc(1)},
			}},
			want: []string{`gendi.yaml:2:5: service "svc" constructor.func: ` + missing("example.com/nope")},
		},
		{
			name: "go reference argument",
			cfg: &di.Config{Services: map[string]di.Service{
				"svc": {Constructor: di.Constructor{
					Func:      "strings.NewReplacer",
					Args:      []di.Argument{{Kind: di.ArgGoRef, Value: "example.com/nope.X", SourceLoc: loc(3)}},
					SourceLoc: loc(2),
				}, Public: true, SourceLoc: loc(1)},
			}},
			want: []string{`gendi.yaml:3:5: service "svc" arg[0]: ` + missing("example.com/nope")},
		},
		{
			name: "tag element type",
			cfg: &di.Config{
				Tags: map[string]di.Tag{"h": {ElementType: "example.com/nope.H", SourceLoc: loc(4)}},
				Services: map[string]di.Service{
					"svc": {Constructor: replacer, Public: true, SourceLoc: loc(1)},
				},
			},
			want: []string{`gendi.yaml:4:5: tag "h" element_type: ` + missing("example.com/nope")},
		},
		{
			name: "first reference wins",
			cfg: &di.Config{Services: map[string]di.Service{
				"bbb": {Constructor: di.Constructor{Func: "example.com/nope.New", SourceLoc: loc(6)}, Public: true, SourceLoc: loc(5)},
				"aaa": {Constructor: di.Constructor{Func: "example.com/nope.New", SourceLoc: loc(2)}, Public: true, SourceLoc: loc(1)},
			}},
			want: []string{`gendi.yaml:2:5: service "aaa" constructor.func: ` + missing("example.com/nope")},
		},
		{
			name: "one error for each package",
			cfg: &di.Config{Services: map[string]di.Service{
				"svc": {
					Type:        "*example.com/zzz.T",
					Constructor: di.Constructor{Func: "example.com/aaa.New", SourceLoc: loc(2)},
					Public:      true,
					SourceLoc:   loc(1),
				},
			}},
			want: []string{
				`gendi.yaml:2:5: service "svc" constructor.func: ` + missing("example.com/aaa"),
				`gendi.yaml:1:5: service "svc" type: ` + missing("example.com/zzz"),
			},
		},
		{
			name: "config without locations",
			cfg: &di.Config{Services: map[string]di.Service{
				"svc": {Constructor: di.Constructor{Func: "example.com/nope.New"}, Public: true},
			}},
			want: []string{`service "svc" constructor.func: ` + missing("example.com/nope")},
		},
		{
			name: "stdlib for tags",
			cfg: &di.Config{Services: map[string]di.Service{
				"svc": {
					Constructor: replacer,
					Tags:        []di.ServiceTag{{Name: "h", SourceLoc: loc(7)}},
					Public:      true,
					SourceLoc:   loc(1),
				},
			}},
			noGendi: true,
			want:    []string{`gendi.yaml:7:5: service "svc" tag "h": tags need the gendi module: ` + missing("github.com/gendi-org/gendi/stdlib")},
		},
		{
			// The tags of an alias are an error by themselves. They must
			// not need the stdlib package, or the error about the module
			// hides that error.
			name: "alias tags do not need stdlib",
			cfg: &di.Config{Services: map[string]di.Service{
				"svc": {Constructor: replacer, SourceLoc: loc(1)},
				"alias": {
					Alias:     "svc",
					Tags:      []di.ServiceTag{{Name: "h", SourceLoc: loc(7)}},
					Public:    true,
					SourceLoc: loc(6),
				},
			}},
			noGendi: true,
			want:    []string{`phase *ir.servicePhase apply: gendi.yaml:7:5: service "alias": alias cannot define tags; tag the target service instead`},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			dir := t.TempDir()
			if tt.noGendi {
				if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module test\n\ngo 1.26.0\n"), 0o644); err != nil {
					t.Fatal(err)
				}
			} else if err := testmodule.Write(dir); err != nil {
				t.Fatal(err)
			}
			opts := Options{Package: "di", Out: dir, ModuleRoot: dir, ModulePath: "test"}
			if err := opts.Finalize(); err != nil {
				t.Fatal(err)
			}

			_, err := Build(tt.cfg, opts)
			if err == nil {
				t.Fatal("expected error, got nil")
			}
			got := err.Error()
			last := -1
			for _, want := range tt.want {
				i := strings.Index(got, want)
				if i < 0 {
					t.Fatalf("error does not contain %q:\n%s", want, got)
				}
				if i < last {
					t.Fatalf("error has %q in the wrong order:\n%s", want, got)
				}
				last = i
			}
			if !strings.HasPrefix(got, tt.want[0]) {
				t.Fatalf("error does not start with %q:\n%s", tt.want[0], got)
			}
		})
	}
}
