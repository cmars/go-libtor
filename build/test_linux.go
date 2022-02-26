// +build mage,linux

package main

import (
	"os"
	"strings"

	"github.com/magefile/mage/sh"
)

func TestBuild() error {
	var err error
	err = TestBuildMatrix("sta", "sta", "sta")
	if err != nil {
		return err
	}
	err = TestBuildMatrix("sta", "sta", "dyn")
	if err != nil {
		return err
	}
	err = TestBuildMatrix("sta", "dyn", "sta")
	if err != nil {
		return err
	}
	err = TestBuildMatrix("sta", "dyn", "dyn")
	if err != nil {
		return err
	}
	err = TestBuildMatrix("dyn", "sta", "sta")
	if err != nil {
		return err
	}
	err = TestBuildMatrix("dyn", "sta", "dyn")
	if err != nil {
		return err
	}
	err = TestBuildMatrix("dyn", "dyn", "sta")
	if err != nil {
		return err
	}
	err = TestBuildMatrix("dyn", "dyn", "dyn")
	if err != nil {
		return err
	}
	return nil
}

func TestBuildMatrix(zlibType, opensslType, libeventType string) error {
	var installPackages, staticTags []string

	if zlibType == "dyn" {
		installPackages = append(installPackages, "zlib1g-dev")
	} else {
		staticTags = append(staticTags, "staticZlib")
	}

	if opensslType == "dyn" {
		installPackages = append(installPackages, "libssl-dev")
	} else {
		staticTags = append(staticTags, "staticOpenssl")
	}

	if libeventType == "dyn" {
		installPackages = append(installPackages, "libevent-dev")
	} else {
		staticTags = append(staticTags, "staticLibevent")
	}

	if len(installPackages) > 0 {
		err := sh.Run("sudo", "apt-get", "update", "-qq")
		if err != nil {
			return err
		}
		installCmd := append([]string{"sudo", "apt-get", "install", "-y"}, installPackages...)
		err = sh.Run("sudo", installCmd...)
		if err != nil {
			return err
		}
	}

	err := os.Chdir("..")
	if err != nil {
		return err
	}

	return sh.Run("go", "build", "-v", "-x", `-tags="`+strings.Join(staticTags, ",")+`"`, ".")
}
