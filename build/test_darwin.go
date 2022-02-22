// +build mage,darwin

package main

import (
	"strings"

	"github.com/magefile/mage/mg"
	"github.com/magefile/mage/sh"
)

func TestBuildStatic() error {
	err := os.Chdir("..")
	if err != nil {
		return err
	}

	return sh.Run("go", "build", "-v", "-x",
		`-tags="staticZlib,staticOpenssl,staticLibevent"`, ".")
}

func TestBuildDynamic() error {
	mg.Deps(mg.F(Sysdeps))

	err := os.Chdir("..")
	if err != nil {
		return err
	}

	return sh.Run("go", "build", "-v", "-x", ".")
}
