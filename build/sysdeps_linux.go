// +build mage,linux

package main

import (
	"github.com/magefile/mage/sh"
)

func Sysdeps() error {
	err := sh.Run("sudo", "apt-get", "update", "-qq")
	if err != nil {
		return err
	}

	err = sh.Run("sudo", "apt-get", "install", "-y",
		"zlib1g-dev", "libssl-dev", "libevent-dev")
	if err != nil {
		return err
	}
	return nil
}
