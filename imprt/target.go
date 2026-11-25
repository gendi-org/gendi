package imprt

import (
	"fmt"
	"path"
	"path/filepath"
	"strings"

	"github.com/bmatcuk/doublestar/v4"
)

// kind classifies an import or exclusion pattern by its syntactic form.
type kind int

const (
	// kindLocal is a path resolved against the importing file's directory.
	kindLocal kind = iota
	// kindModule is a path resolved against the Go module it names.
	kindModule
)

// target is an addressed import: what to resolve, against which directory,
// and which root the resolved files may not escape.
type target struct {
	kind       kind
	anchorDir  string // directory the pattern resolves against
	boundary   string // confinement root for resolved files
	pattern    string // for modules, the remainder after the module path
	modulePath string // set for kindModule, used for exclusion masks and errors
}

// excludeMask converts one exclusion pattern into a slash mask relative to the
// import's anchor.
func (t target) excludeMask(exclude string) (string, error) {
	k, err := classify(exclude)
	if err != nil {
		return "", fmt.Errorf("exclusion %q: %w", exclude, err)
	}
	if k != t.kind {
		return "", fmt.Errorf("exclusion %q does not match the addressing of the import: exclude a module import with a module pattern and a local import with a local pattern", exclude)
	}
	mask := exclude
	if t.kind == kindModule {
		remainder, ok := strings.CutPrefix(exclude, t.modulePath+"/")
		if !ok {
			return "", fmt.Errorf("exclusion %q must name a path inside module %s of the import", exclude, t.modulePath)
		}
		mask = remainder
	}
	mask = path.Clean(filepath.ToSlash(mask))
	if !doublestar.ValidatePattern(mask) {
		return "", fmt.Errorf("invalid exclusion pattern %q", exclude)
	}
	return mask, nil
}

// excludeMasks converts exclusion patterns into slash masks relative to the
// import's anchor. An exclusion is addressed exactly like its import: a local
// import takes local patterns, a module import takes patterns inside the same
// module. Masks are pure filters over the files the import found — they never
// touch the filesystem, so a mask that matches nothing is a no-op. Masks keep
// the order of excludes.
func (t target) excludeMasks(excludes []string) ([]string, error) {
	masks := make([]string, 0, len(excludes))
	for i, exclude := range excludes {
		mask, err := t.excludeMask(exclude)
		if err != nil {
			return nil, &ExclusionError{Index: i, Err: err}
		}
		masks = append(masks, mask)
	}
	return masks, nil
}

// excludedBy reports whether file — or any directory on its path relative to
// the import's anchor — matches one of the exclusion masks; a mask matching a
// directory therefore excludes its whole subtree.
func (t target) excludedBy(masks []string, file string) (bool, error) {
	if len(masks) == 0 {
		return false, nil
	}
	rel, err := filepath.Rel(t.anchorDir, file)
	if err != nil {
		return false, err
	}
	prefix := ""
	for segment := range strings.SplitSeq(filepath.ToSlash(rel), "/") {
		prefix = path.Join(prefix, segment)
		for i, mask := range masks {
			matched, err := doublestar.Match(mask, prefix)
			if err != nil {
				return false, &ExclusionError{Index: i, Err: fmt.Errorf("exclusion %q: %w", mask, err)}
			}
			if matched {
				return true, nil
			}
		}
	}
	return false, nil
}

func (t target) notFoundError(full string) error {
	if t.kind == kindModule {
		return fmt.Errorf("module %s does not contain %s", t.modulePath, t.pattern)
	}
	return fmt.Errorf("not found at %s", full)
}
