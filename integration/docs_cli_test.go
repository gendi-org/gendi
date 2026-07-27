package integration

import (
	"flag"
	"os"
	"regexp"
	"strings"
	"testing"

	"github.com/gendi-org/gendi/cmd"
)

const docsCLIPage = "../site/content/docs/cli.md"

// A table row: | `--name type` | description | notes |
var flagRowRe = regexp.MustCompile("(?m)^\\| `--([a-z-]+)[^`]*` \\| ([^|]*?) \\|")

// TestDocsCLI keeps the CLI page's flag table in step with the flags the
// generator actually registers — the table is the only place the flags are
// documented, and nothing else would notice a renamed or forgotten one.
//
// The description column must be the flag's usage string verbatim; anything
// the page wants to add goes in the third column.
func TestDocsCLI(t *testing.T) {
	page, err := os.ReadFile(docsCLIPage)
	if err != nil {
		t.Fatalf("read %s: %v", docsCLIPage, err)
	}

	// Only the flag table, which precedes the first subheading. The page has a
	// second table further down, keyed by pass name rather than flag.
	table, _, _ := strings.Cut(string(page), "\n## ")

	documented := map[string]string{}
	for _, m := range flagRowRe.FindAllStringSubmatch(table, -1) {
		documented[m[1]] = strings.TrimSpace(m[2])
	}

	fs := flag.NewFlagSet("gendi", flag.ContinueOnError)
	(&cmd.Config{}).RegisterFlags(fs)

	registered := map[string]*flag.Flag{}
	fs.VisitAll(func(f *flag.Flag) { registered[f.Name] = f })

	for name, f := range registered {
		desc, ok := documented[name]
		if !ok {
			t.Errorf("flag --%s is registered but missing from %s", name, docsCLIPage)

			continue
		}

		if desc != f.Usage {
			t.Errorf("flag --%s is described as %q, but its usage string is %q",
				name, desc, f.Usage)
		}

		// A meaningful default belongs in the row, so a reader does not have to
		// run the binary to find it. Booleans default to false and say nothing.
		if f.DefValue != "" && f.DefValue != "false" &&
			!strings.Contains(rowFor(string(page), name), f.DefValue) {
			t.Errorf("flag --%s defaults to %q, which its row does not mention",
				name, f.DefValue)
		}
	}

	for name := range documented {
		if _, ok := registered[name]; !ok {
			t.Errorf("%s documents --%s, which the generator does not register",
				docsCLIPage, name)
		}
	}

	if len(registered) == 0 || len(documented) == 0 {
		t.Fatalf("nothing checked (%d registered, %d documented) — has the table "+
			"or RegisterFlags been restructured?", len(registered), len(documented))
	}
}

// rowFor returns the whole table row documenting a flag.
func rowFor(page, name string) string {
	for line := range strings.SplitSeq(page, "\n") {
		if strings.HasPrefix(line, "| `--"+name) {
			return line
		}
	}

	return ""
}
