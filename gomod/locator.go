// Package gomod locates Go modules on disk and in the module graph.
//
// It answers the two questions the rest of the generator has about modules:
// which module a directory belongs to, and where the module named by an import
// actually lives once replace directives are taken into account. Both are
// resolved from go.mod rather than from the process working directory, so a
// configuration resolves identically wherever the generator is run from.
package gomod

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// Locator finds Go module directories by module path.
type Locator struct {
	// BaseDir is the directory to start searching from.
	BaseDir string
}

// NewLocator creates a new module locator with the given base directory.
func NewLocator(baseDir string) *Locator {
	return &Locator{BaseDir: baseDir}
}

// FindModuleDir returns the directory for the given module path. It first
// checks whether modulePath is the module containing BaseDir, then falls back
// to "go list" from BaseDir — resolution is a pure function of BaseDir and
// its go.mod graph, never of the process working directory.
func (l *Locator) FindModuleDir(modulePath string) (string, error) {
	if dir, modPath, ok := FindModuleRoot(l.BaseDir); ok && modPath == modulePath {
		return dir, nil
	}
	return l.findModuleDirViaGoList(modulePath)
}

// findModuleDirViaGoList uses "go list" to find the module directory.
func (l *Locator) findModuleDirViaGoList(modulePath string) (string, error) {
	cmd := exec.Command("go", "list", "-m", "-f", "{{.Dir}}", modulePath)
	cmd.Dir = l.BaseDir
	out, err := cmd.Output()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			msg := strings.TrimSpace(string(exitErr.Stderr))
			if msg == "" {
				return "", err
			}
			return "", fmt.Errorf("%s", msg)
		}
		return "", err
	}
	dir := strings.TrimSpace(string(out))
	if dir == "" {
		return "", fmt.Errorf("go list returned empty module dir for %s", modulePath)
	}
	return dir, nil
}

// FindModuleRoot searches upward from startDir to find a go.mod file.
// Returns the directory containing go.mod, the module path, and whether it
// was found. A relative startDir is resolved against the process working
// directory first — an upward walk on the relative spelling would stop at
// the working directory instead of climbing its ancestors.
func FindModuleRoot(startDir string) (dir string, modulePath string, found bool) {
	if abs, err := filepath.Abs(startDir); err == nil {
		startDir = abs
	}
	dir = startDir
	for {
		modFile := filepath.Join(dir, "go.mod")
		if data, err := os.ReadFile(modFile); err == nil {
			if modPath := ParseModulePath(data); modPath != "" {
				return dir, modPath, true
			}
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", "", false
		}
		dir = parent
	}
}

// ParseModulePath extracts the module path from go.mod content, handling
// trailing comments and quoted paths.
func ParseModulePath(data []byte) string {
	for line := range strings.SplitSeq(string(data), "\n") {
		line = strings.TrimSpace(line)
		rest, found := strings.CutPrefix(line, "module")
		if !found || (rest != "" && rest[0] != ' ' && rest[0] != '\t') {
			continue
		}
		if idx := strings.Index(rest, "//"); idx >= 0 {
			rest = rest[:idx]
		}
		path := strings.TrimSpace(rest)
		path = strings.Trim(path, "\"`")
		if path != "" {
			return path
		}
	}
	return ""
}

// LooksLikeModulePath returns true if path appears to be a Go module path
// (i.e., first segment contains a dot, like "github.com/...").
func LooksLikeModulePath(path string) bool {
	parts := strings.Split(path, "/")
	if len(parts) == 0 {
		return false
	}
	return strings.Contains(parts[0], ".")
}
