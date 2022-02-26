// +build mage

package main

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"text/template"

	"github.com/magefile/mage/sh"
)

const (
	OpensslURL = "https://github.com/openssl/openssl"
	OpensslTag = "OpenSSL_1_1_1-stable"
)

// wrapOpenSSL clones the OpenSSL library into the local repository and wraps
// it into a Go package.
//
// OpenSSL is a fairly complex C library, heavily relying on makefiles to mix-
// and-match the correct sources for the correct platforms and it also relies on
// platform specific assembly sources for more performant builds.
//
// Since it's not meaningfully feasible to build OpenSSL without the make tools,
// yet that approach cannot create a portable Go library, we're going to hook
// into the original build mechanism and use the emitted events as a driver for
// the Go wrapping.
//
// In addition, assembly is disabled altogether to retain Go's portability. This
// is a downside we unfortunately have to live with for now.
func WrapOpenssl(root string) error {
	opensslDir := filepath.Join(root, runtime.GOOS, "openssl")
	err := sh.Rm(opensslDir)
	if err != nil {
		return err
	}
	err = sh.Run("git", "clone", "--depth", "1", "-b", OpensslTag, OpensslURL, opensslDir)
	if err != nil {
		return fmt.Errorf("git clone failed: %w", err)
	}

	date, err := sh.Output("git", "show", "-s", "--format=%cd")
	if err != nil {
		return err
	}

	popd := mustPushd(opensslDir)
	defer popd()

	// Configure the library for compilation
	err = sh.Run("./config", "no-shared", "no-zlib", "no-asm", "no-async", "no-sctp")
	if err != nil {
		return fmt.Errorf("OpenSSL config failed: %w", err)
	}

	// Hook the make system and gather the needed sources
	makeOutput, err := sh.Output("make", "--dry-run")
	if err != nil {
		fmt.Println(makeOutput)
		return fmt.Errorf("make failed: %w", err)
	}
	deps := regexp.MustCompile("(?m)([a-z0-9_/-]+)\\.c$").FindAllStringSubmatch(makeOutput, -1)

	// Wipe everything from the library that's non-essential
	files, err := ioutil.ReadDir(".")
	if err != nil {
		return fmt.Errorf("failed to read directory: %w", err)
	}
	for _, file := range files {
		// Remove all folders apart from the headers
		if file.IsDir() {
			if file.Name() == "crypto" || file.Name() == "engines" || file.Name() == "include" || file.Name() == "ssl" {
				continue
			}
			err := sh.Rm(file.Name())
			if err != nil {
				return err
			}
		} else if file.Name() == "LICENSE" {
			// Remove all files apart from the license and sources
			continue
		} else if ext := filepath.Ext(file.Name()); ext != ".h" && ext != ".c" {
			err := sh.Rm(file.Name())
			if err != nil {
				return err
			}
		}
	}

	targetFilter := targetFilters[runtime.GOOS]

	// Generate Go wrappers for each C source individually
	tmpl, err := template.New("").Parse(opensslTemplate)
	if err != nil {
		return err
	}
	for _, dep := range deps {
		// Skip any files not needed for the library
		if strings.HasPrefix(dep[1], "apps/") {
			continue
		}
		if strings.HasPrefix(dep[1], "fuzz/") {
			continue
		}
		if strings.HasPrefix(dep[1], "test/") {
			continue
		}
		// Anything else is wrapped directly with Go
		gofile := strings.Replace(dep[1], "/", "_", -1) + ".go"
		err = func() error {
			sourceFile, err := os.Create(filepath.Join(root, "libtor", runtime.GOOS+"_openssl_"+gofile))
			if err != nil {
				return fmt.Errorf("failed to create wrapped %q: %w", gofile, err)
			}
			defer sourceFile.Close()
			if err := tmpl.Execute(sourceFile, map[string]string{
				"TargetFilter": targetFilter,
				"File":         dep[1],
			}); err != nil {
				return err
			}
			return nil
		}()
		if err != nil {
			return err
		}
	}
	preambleFilename := filepath.Join(root, "libtor", runtime.GOOS+"_openssl_preamble.go")
	preambleFile, err := os.Create(preambleFilename)
	if err != nil {
		return fmt.Errorf("failed to create %q: %w", preambleFilename, err)
	}
	defer preambleFile.Close()
	tmpl, err = template.New("").Parse(opensslPreamble)
	if err != nil {
		return err
	}
	if err := tmpl.Execute(preambleFile, map[string]string{
		"TargetFilter": targetFilter,
		"Target":       runtime.GOOS,
	}); err != nil {
		return err
	}

	// Inject the configuration headers and ensure everything builds
	err = os.MkdirAll(filepath.Join(root, "openssl_config", "crypto"), 0755)
	if err != nil {
		return err
	}

	for _, arch := range []string{"", ".linux", ".darwin"} {
		blob, err := ioutil.ReadFile(filepath.Join(root, "config", "openssl", fmt.Sprintf("dso_conf%s.h", arch)))
		if err != nil {
			return err
		}
		err = ioutil.WriteFile(filepath.Join(root, "openssl_config", "crypto", fmt.Sprintf("dso_conf%s.h", arch)), blob, 0644)
		if err != nil {
			return err
		}
	}

	for _, arch := range []string{"", ".x64", ".x86"} {
		blob, err := ioutil.ReadFile(filepath.Join(root, "config", "openssl", fmt.Sprintf("bn_conf%s.h", arch)))
		if err != nil {
			return err
		}
		err = ioutil.WriteFile(filepath.Join(root, "openssl_config", "crypto", fmt.Sprintf("bn_conf%s.h", arch)), blob, 0644)
		if err != nil {
			return err
		}
	}
	for _, arch := range []string{"", ".x64", ".x86", ".macos64", ".ios64"} {
		blob, err := ioutil.ReadFile(filepath.Join(root, "config", "openssl", fmt.Sprintf("buildinf%s.h", arch)))
		if err != nil {
			return err
		}
		tmpl, err := template.New("").Parse(string(blob))
		if err != nil {
			return err
		}
		buff := new(bytes.Buffer)
		if err := tmpl.Execute(buff, struct{ Date string }{string(date)}); err != nil {
			return err
		}
		err = ioutil.WriteFile(filepath.Join(root, "openssl_config", fmt.Sprintf("buildinf%s.h", arch)), buff.Bytes(), 0644)
		if err != nil {
			return err
		}
	}
	os.MkdirAll(filepath.Join(root, "openssl_config", "openssl"), 0755)

	for _, arch := range []string{"", ".x64", ".x86", ".macos64", ".ios64"} {
		blob, err := ioutil.ReadFile(filepath.Join(root, "config", "openssl", fmt.Sprintf("opensslconf%s.h", arch)))
		if err != nil {
			return err
		}
		err = ioutil.WriteFile(filepath.Join(root, "openssl_config", "openssl", fmt.Sprintf("opensslconf%s.h", arch)), blob, 0644)
		if err != nil {
			return err
		}
	}
	return nil
}

// opensslPreamble is the CGO preamble injected to configure the C compiler.
var opensslPreamble = `// go-libtor - Self-contained Tor from Go
// Copyright (c) 2018 Péter Szilágyi. All rights reserved.
// +build {{.TargetFilter}}
// +build staticOpenssl

package libtor

/*
#cgo CFLAGS: -I${SRCDIR}/../openssl_config
#cgo CFLAGS: -I${SRCDIR}/../{{.Target}}/openssl
#cgo CFLAGS: -I${SRCDIR}/../{{.Target}}/openssl/include
#cgo CFLAGS: -I${SRCDIR}/../{{.Target}}/openssl/crypto/ec/curve448
#cgo CFLAGS: -I${SRCDIR}/../{{.Target}}/openssl/crypto/ec/curve448/arch_32
#cgo CFLAGS: -I${SRCDIR}/../{{.Target}}/openssl/crypto/modes
*/
import "C"
`

// opensslTemplate is the source file template used in OpenSSL Go wrappers.
var opensslTemplate = `// go-libtor - Self-contained Tor from Go
// Copyright (c) 2018 Péter Szilágyi. All rights reserved.
// +build {{.TargetFilter}}
// +build staticOpenssl

package libtor

/*
#define DSO_NONE
#define OPENSSLDIR "/usr/local/ssl"
#define ENGINESDIR "/usr/local/lib/engines"

#include <../{{.File}}.c>
*/
import "C"
`
