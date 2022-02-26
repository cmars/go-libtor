// +build mage

package main

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"

	"github.com/magefile/mage/mg"
	"github.com/magefile/mage/sh"
)

func projectRoot() (string, error) {
	return filepath.Abs("..")
}

func mustPushd(path string) func() {
	path, err := filepath.Abs(path)
	if err != nil {
		panic(err)
	}
	cwd, err := os.Getwd()
	if err != nil {
		panic(err)
	}
	err = os.Chdir(path)
	if err != nil {
		panic(err)
	}
	fmt.Printf("entering directory: %s\n", path)
	return func() {
		err := os.Chdir(cwd)
		if err != nil {
			panic(err)
		}
		fmt.Printf("leaving directory: %s\n", path)
	}
}

// targetFilters maps a build target to the builds tags to apply to it
var targetFilters = map[string]string{
	"linux":  "linux android",
	"darwin": "darwin,amd64 darwin,arm64 ios,amd64 ios,arm64",
}

func All() error {
	var err error
	err = Sysdeps()
	if err != nil {
		return err
	}
	err = Wrap()
	if err != nil {
		return err
	}
	err = TestBuildDynamic()
	if err != nil {
		return err
	}
	err = TestBuildStatic()
	if err != nil {
		return err
	}
	return nil
}

func Wrap() error {
	root, err := projectRoot()
	if err != nil {
		return fmt.Errorf("failed to resolve project root: %w", err)
	}
	mg.Deps(
		mg.F(Clean, root),
		mg.F(Setenv),
	)

	for _, dir := range []string{"libtor", runtime.GOOS} {
		dpath := filepath.Join(root, dir)
		err := os.MkdirAll(dpath, 0777)
		if err != nil {
			return fmt.Errorf("failed to mkdir %q: %w", dir, err)
		}
	}

	err = WrapZlib(root)
	if err != nil {
		return err
	}
	err = WrapOpenssl(root)
	if err != nil {
		return err
	}
	err = WrapLibevent(root)
	if err != nil {
		return err
	}
	err = WrapTor(root)
	if err != nil {
		return err
	}

	return nil
}

func Clean(root string) error {
	wrappedFiles, err := filepath.Glob(filepath.Join(root, "libtor", runtime.GOOS+"_*"))
	if err != nil {
		return fmt.Errorf("failed to match source files to rebuild: %w", err)
	}
	for _, f := range wrappedFiles {
		err := sh.Rm(f)
		if err != nil {
			return fmt.Errorf("failed to remove %q: %w", f, err)
		}
	}
	return nil
}

func Archive() error {
	root, err := projectRoot()
	if err != nil {
		return err
	}

	popd := mustPushd(root + "/..")
	defer popd()
	err = sh.Run("tar", "cvf", "/tmp/go-libtor.tar", filepath.Base(root))
	if err != nil {
		return err
	}
	return nil
}
