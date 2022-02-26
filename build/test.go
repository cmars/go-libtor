// +build mage

package main

import (
	"github.com/magefile/mage/mg"
	"github.com/magefile/mage/sh"
)

func TestBuildStatic() error {
	popd := mustPushd("..")
	defer popd()

	return sh.Run("go", "build", "-v", "-x",
		`-tags="staticZlib,staticOpenssl,staticLibevent"`, ".")
}

func TestBuildDynamic() error {
	mg.Deps(
		mg.F(Sysdeps),
		mg.F(Setenv),
	)

	popd := mustPushd("..")
	defer popd()

	return sh.Run("go", "build", "-v", "-x", ".")
}
