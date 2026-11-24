package integration

import (
	"embed"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"testing"

	"golang.org/x/mod/modfile"
)

//go:embed testdata/*
var testdata embed.FS

// getModuleRoot returns the absolute path to the gendi module root
func getModuleRoot() string {
	// Find go.mod in current or parent directories
	dir, err := os.Getwd()
	if err != nil {
		return "../"
	}

	for {
		goModPath := filepath.Join(dir, "go.mod")
		if _, err := os.Stat(goModPath); err == nil {
			return dir
		}

		parent := filepath.Dir(dir)
		if parent == dir {
			// Reached filesystem root
			return "../"
		}
		dir = parent
	}
}

func copyFile(src, dst string) error {
	srcF, err := testdata.Open(src)
	if err != nil {
		return err
	}
	defer srcF.Close()

	dstF, err := os.OpenFile(dst, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0644)
	if err != nil {
		return err
	}
	defer dstF.Close()

	_, err = io.Copy(dstF, srcF)
	return err
}

func copyDir(src, dst string, exclude []string) error {
	return fs.WalkDir(testdata, src, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		relPath, _ := filepath.Rel(src, path)
		targetPath := filepath.Join(dst, relPath)

		if slices.Contains(exclude, relPath) {
			if d.IsDir() {
				return filepath.SkipDir
			}

			return nil
		}

		if d.IsDir() {
			return os.MkdirAll(targetPath, 0755)
		}

		return copyFile(path, targetPath)
	})
}

// createGoMod writes the go.mod of a fixture with the go version of the gendi
// module. The go command does not build a module that requires a module with
// a later go version than its own.
func createGoMod(goModPath string) error {
	root := getModuleRoot()
	data, err := os.ReadFile(filepath.Join(root, "go.mod"))
	if err != nil {
		return err
	}
	gendiMod, err := modfile.ParseLax("go.mod", data, nil)
	if err != nil {
		return err
	}

	goModContent := fmt.Sprintf(`module test

go %s

require github.com/gendi-org/gendi v0.0.0

replace github.com/gendi-org/gendi => %s
`, gendiMod.Go.Version, root)

	return os.WriteFile(goModPath, []byte(goModContent), 0644)
}

func createEmptyMainGo(mainGoPath string) error {
	const goModContent = "package main\nfunc main() {}\n"

	return os.WriteFile(mainGoPath, []byte(goModContent), 0644)
}

func prepareTestDir(t *testing.T, src string) string {
	tmpDir := t.TempDir()
	if err := copyDir(src, tmpDir, []string{"main.go"}); err != nil {
		t.Fatal(err)
	}

	goModPath := filepath.Join(tmpDir, "go.mod")
	if _, err := os.Stat(goModPath); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("go.mod error: %v", err)
	}

	if err := createGoMod(goModPath); err != nil {
		t.Fatal(err)
	}

	if err := createEmptyMainGo(filepath.Join(tmpDir, "main.go")); err != nil {
		t.Fatal(err)
	}

	return tmpDir
}

func prepareMainGo(srcDir, dstDir string) error {
	mainPath := filepath.Join(srcDir, "main.go")
	_, err := os.Stat(mainPath)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}

	if err != nil {
		return err
	}

	return copyFile(mainPath, filepath.Join(dstDir, "main.go"))
}
