// Package testmodule writes a temporary Go module for tests. The module uses
// the gendi module of this repository through a replace directive.
package testmodule

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"

	"golang.org/x/mod/modfile"
)

// Root returns the absolute path of the gendi module root.
func Root() string {
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		panic("testmodule: no caller information")
	}
	return filepath.Join(filepath.Dir(file), "..", "..")
}

// Write writes go.mod and go.sum of the module "test" to dir.
//
// The go.mod takes the go version and all requirements of gendi. The go
// command refuses a module with an earlier go version than a dependency, and
// from go 1.17 the main module must require every module that provides a
// package in its build. The go.sum is a copy of the gendi go.sum. Without
// these two files, a package that imports a gendi package with an external
// dependency does not build.
func Write(dir string) error {
	root := Root()
	data, err := os.ReadFile(filepath.Join(root, "go.mod"))
	if err != nil {
		return err
	}
	gendiMod, err := modfile.ParseLax("go.mod", data, nil)
	if err != nil {
		return fmt.Errorf("parse gendi go.mod: %w", err)
	}

	mod := &modfile.File{Syntax: &modfile.FileSyntax{}}
	if err := mod.AddModuleStmt("test"); err != nil {
		return err
	}
	if err := mod.AddGoStmt(gendiMod.Go.Version); err != nil {
		return err
	}
	gendiPath := gendiMod.Module.Mod.Path
	mod.AddNewRequire(gendiPath, "v0.0.0", false)
	for _, req := range gendiMod.Require {
		mod.AddNewRequire(req.Mod.Path, req.Mod.Version, true)
	}
	if err := mod.AddReplace(gendiPath, "", root, ""); err != nil {
		return err
	}
	mod.Cleanup()
	out, err := mod.Format()
	if err != nil {
		return fmt.Errorf("format go.mod: %w", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "go.mod"), out, 0o644); err != nil {
		return err
	}

	sum, err := os.ReadFile(filepath.Join(root, "go.sum"))
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(dir, "go.sum"), sum, 0o644)
}
