// +build mage

package main

import (
	"os"
)

func prependEnv(key, value string) error {
	newValue := os.Getenv(key)
	if key == "" {
		newValue = value
	} else {
		newValue = value + ":" + newValue
	}
	return os.Setenv(key, newValue)
}
