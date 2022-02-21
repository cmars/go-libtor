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
	"text/template"

	"github.com/magefile/mage/sh"
)

const (
	LibeventURL = "https://github.com/libevent/libevent"
	LibeventTag = "release-2.1.12-stable"
)

// wrapLibevent clones the libevent library into the local repository and wraps
// it into a Go package.
//
// Libevent is a fairly straightforward C library, however it heavily relies on
// makefiles to mix-and-match the correct sources for the correct platforms. It
// also relies on autoconf and family to generate platform specific configs.
//
// Since it's not meaningfully feasible to build libevent without the make tools,
// yet that approach cannot create a portable Go library, we're going to hook
// into the original build mechanism and use the emitted events as a driver for
// the Go wrapping.
func WrapLibevent(root string) error {
	libeventDir := filepath.Join(root, runtime.GOOS, "libevent")
	err := sh.Rm(libeventDir)
	if err != nil {
		return err
	}
	err = sh.Run("git", "clone", "--depth", "1", "-b", LibeventTag, LibeventURL, libeventDir)
	if err != nil {
		return fmt.Errorf("git clone failed: %w", err)
	}

	cwd, err := os.Getwd()
	if err != nil {
		return err
	}
	defer os.Chdir(cwd)

	fmt.Println("entering directory:", libeventDir)
	err = os.Chdir(libeventDir)
	if err != nil {
		return err
	}
	defer fmt.Println("leaving directory:", libeventDir)

	err = sh.Run("./autogen.sh")
	if err != nil {
		return fmt.Errorf("autogen failed: %w")
	}

	err = sh.Run("./configure", "--disable-shared", "--enable-static")
	if err != nil {
		return fmt.Errorf("configure failed: %w")
	}

	// Retrieve the version of the current commit
	conf, err := ioutil.ReadFile("configure.ac")
	if err != nil {
		return err
	}
	numver := regexp.MustCompile("AC_DEFINE\\(NUMERIC_VERSION, (0x[0-9a-f]{8}),").FindSubmatch(conf)[1]
	strver := regexp.MustCompile("AC_INIT\\(libevent,(.+)\\)").FindSubmatch(conf)[1]

	// Hook the make system and gather the needed sources
	makeOutput, err := sh.Output("make", "--dry-run", "libevent.la")
	if err != nil {
		return fmt.Errorf("failed to run make to get dependencies: %w", err)
	}
	deps := regexp.MustCompile(" ([a-z_]+)\\.lo;").FindAllStringSubmatch(string(makeOutput), -1)

	// Wipe everything from the library that's non-essential
	files, err := ioutil.ReadDir(".")
	if err != nil {
		return err
	}
	for _, file := range files {
		// Remove all folders apart from the headers
		if file.IsDir() {
			if file.Name() == "include" || file.Name() == "compat" {
				continue
			}
			err := os.RemoveAll(file.Name())
			if err != nil {
				return err
			}
			continue
		}
		// Remove all files apart from the sources and license
		if file.Name() == "LICENSE" {
			continue
		}
		if ext := filepath.Ext(file.Name()); ext != ".h" && ext != ".c" {
			err := os.Remove(file.Name())
			if err != nil {
				return err
			}
		}
	}

	targetFilter := targetFilters[runtime.GOOS]

	// Generate Go wrappers for each C source individually
	tmpl, err := template.New("").Parse(libeventTemplate)
	if err != nil {
		return err
	}
	for _, dep := range deps {
		sourceFilename := filepath.Join(root, "libtor", runtime.GOOS+"_libevent_"+dep[1]+".go")
		err := func() error {
			sourceFile, err := os.Create(sourceFilename)
			if err != nil {
				return fmt.Errorf("failed to create source file %q: %w", sourceFilename, err)
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
	preambleFilename := filepath.Join(root, "libtor", runtime.GOOS+"_libevent_preamble.go")
	tmpl, err = template.New("").Parse(libeventPreamble)
	if err != nil {
		return err
	}
	preambleFile, err := os.Create(preambleFilename)
	if err != nil {
		return fmt.Errorf("failed to create source file %q: %w", preambleFilename, err)
	}
	defer preambleFile.Close()
	if err := tmpl.Execute(preambleFile, map[string]string{
		"TargetFilter": targetFilter,
		"Target":       runtime.GOOS,
	}); err != nil {
		return err
	}

	// Inject the configuration headers and ensure everything builds
	err = os.MkdirAll(filepath.Join(root, "libevent_config", "event2"), 0755)
	if err != nil {
		return err
	}

	for _, arch := range []string{"", ".linux64", ".linux32", ".android64", ".android32", ".macos64", ".ios64"} {
		blob, err := ioutil.ReadFile(filepath.Join(root, "config", "libevent", fmt.Sprintf("event-config%s.h", arch)))
		if err != nil {
			return err
		}
		tmpl, err := template.New("").Parse(string(blob))
		if err != nil {
			return err
		}
		buff := new(bytes.Buffer)
		if err := tmpl.Execute(buff, struct{ NumVer, StrVer string }{string(numver), string(strver)}); err != nil {
			return err
		}
		err = ioutil.WriteFile(filepath.Join(root, "libevent_config", "event2", fmt.Sprintf("event-config%s.h", arch)), buff.Bytes(), 0644)
		if err != nil {
			return err
		}
	}
	return nil
}

// libeventPreamble is the CGO preamble injected to configure the C compiler.
var libeventPreamble = `// go-libtor - Self-contained Tor from Go
// Copyright (c) 2018 Péter Szilágyi. All rights reserved.
// +build {{.TargetFilter}}
// +build staticLibevent

package libtor

/*
#cgo CFLAGS: -I${SRCDIR}/../libevent_config
#cgo CFLAGS: -I${SRCDIR}/../{{.Target}}/libevent
#cgo CFLAGS: -I${SRCDIR}/../{{.Target}}/libevent/compat
#cgo CFLAGS: -I${SRCDIR}/../{{.Target}}/libevent/include
*/
import "C"
`

// libeventTemplate is the source file template used in libevent Go wrappers.
var libeventTemplate = `// go-libtor - Self-contained Tor from Go
// Copyright (c) 2018 Péter Szilágyi. All rights reserved.
// +build {{.TargetFilter}}
// +build staticLibevent

package libtor

/*
#include <compat/sys/queue.h>
#include <../{{.File}}.c>
*/
import "C"
`
