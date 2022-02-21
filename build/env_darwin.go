// +build mage,darwin

package main

import (
	"os"
)

func Setenv() error {
	// Use autoconf 2.69 to build Tor. 2.71 doesn't work.
	err := prependEnv("PATH", "/usr/local/opt/autoconf@2.69/bin")
	if err != nil { return err }

	// Configure Tor to find OpenSSL includes and libraries on macos
	err = prependEnv("LD_LIBRARY_PATH", "/usr/local/opt/openssl@1.1/lib")
	if err != nil { return err }
	err = prependEnv("CPATH", "/usr/local/opt/openssl@1.1/include")
	if err != nil { return err }
	err = prependEnv("PKG_CONFIG_PATH", "/usr/local/opt/openssl@1.1/lib/pkgconfig")
	if err != nil { return err }
	err = os.Setenv( "LDFLAGS","-g -O2 -L/usr/local/opt/openssl@1.1/lib")
	if err != nil { return err }
	err = os.Setenv( "CFLAGS","-g -O2 -I/usr/local/opt/openssl@1.1/include")
	if err != nil { return err }
	err = os.Setenv( "CGO_LDFLAGS","-g -O2 -L/usr/local/opt/openssl@1.1/lib")
	if err != nil { return err }
	err = os.Setenv( "CGO_CFLAGS","-g -O2 -I/usr/local/opt/openssl@1.1/include")
	if err != nil { return err }
	return nil
}
