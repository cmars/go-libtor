// +build mage,darwin

package main

import (
	"github.com/magefile/mage/sh"
)

func Sysdeps() error {
	err := sh.Run("brew", "install",
		"pkg-config", "autoconf@2.69", "automake",
		"openssl@1.1")
	if err != nil {
		return err
	}
	return nil
}
