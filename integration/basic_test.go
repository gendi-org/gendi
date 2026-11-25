package integration

import (
	"embed"
	"errors"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/gendi-org/gendi/internal/testmodule"
)

//go:embed testdata/*
var testdata embed.FS

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

	if err := testmodule.Write(tmpDir); err != nil {
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
