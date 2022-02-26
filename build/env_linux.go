// +build mage,linux

package main

import (
	"os"
)

func Setenv() error {
	err := os.Setenv("CGO_CFLAGS", `-g -O2 -DSHARE_DATADIR="./data" -DLOCALSTATEDIR="./state"`)
	if err != nil {
		return err
	}
	return nil
}
