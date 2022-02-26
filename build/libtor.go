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
	TorURL = "https://git.torproject.org/tor.git"
	TorTag = "release-0.4.6"
)

// wrapTor clones the tor library into the local repository and wraps it into
// a Go package.
//
// Tor is a small and simple C library which can be wrapped by inserting an empty
// Go file among the C sources, causing the Go compiler to pick up all the loose
// sources and build them together into a static library.
func WrapTor(root string) error {
	torDir := filepath.Join(root, runtime.GOOS, "tor")
	err := sh.Rm(torDir)
	if err != nil {
		return err
	}
	err = sh.Run("git", "clone", "--depth", "1", "-b", TorTag, TorURL, torDir)
	if err != nil {
		return fmt.Errorf("git clone failed: %w", err)
	}

	popd := mustPushd(torDir)
	defer popd()

	err = sh.Run("./autogen.sh")
	if err != nil {
		return err
	}

	err = sh.Run("./configure", "--disable-asciidoc")
	if err != nil {
		return err
	}

	// Retrieve the version of the current commit
	winconf, err := ioutil.ReadFile(filepath.Join("src", "win32", "orconfig.h"))
	if err != nil {
		return err
	}
	strver := regexp.MustCompile("define VERSION \"(.+)\"").FindSubmatch(winconf)[1]

	makeOutput, err := sh.Output("make", "--dry-run")
	if err != nil {
		return err
	}
	deps := regexp.MustCompile("(?m)([a-z0-9_/-]+)\\.c").FindAllStringSubmatch(string(makeOutput), -1)

	// Wipe everything from the library that's non-essential
	files, err := ioutil.ReadDir(".")
	if err != nil {
		return err
	}
	for _, file := range files {
		// Remove all folders apart from the sources
		if file.IsDir() {
			if file.Name() == "src" {
				continue
			}
			err := os.RemoveAll(file.Name())
			if err != nil {
				return err
			}
			continue
		}
		// Remove all files apart from the license
		if file.Name() == "LICENSE" || file.Name() == "orconfig.h" {
			continue
		}
		err := os.Remove(file.Name())
		if err != nil {
			return err
		}
	}
	// Wipe all the sources from the library that are non-essential
	files, err = ioutil.ReadDir("src")
	if err != nil {
		return err
	}
	for _, file := range files {
		if file.IsDir() {
			switch file.Name() {
			case "app", "core", "ext", "feature", "lib", "trunnel", "win32":
				continue
			}
			err := os.RemoveAll(filepath.Join("src", file.Name()))
			if err != nil {
				return err
			}
			continue
		}
		err := os.Remove(filepath.Join("src", file.Name()))
		if err != nil {
			return err
		}
	}
	// Wipe all the weird .Po files containing dummies
	if err := filepath.Walk("src",
		func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return err
			}
			if filepath.Base(path) == ".deps" {
				err := os.RemoveAll(path)
				if err != nil {
					return err
				}
				return filepath.SkipDir
			}
			return nil
		},
	); err != nil {
		return err
	}

	targetFilter := targetFilters[runtime.GOOS]

	tmpl, err := template.New("").Parse(torTemplate)
	if err != nil {
		return err
	}
	for _, dep := range deps {
		// Skip any files not needed for the library
		if strings.HasPrefix(dep[1], "src/ext/tinytest") {
			continue
		}
		if strings.HasPrefix(dep[1], "src/test/") {
			continue
		}
		if strings.HasPrefix(dep[1], "src/tools/") {
			continue
		}
		// Skip the main tor entry point, we're wrapping a lib
		if strings.HasSuffix(dep[1], "tor_main") {
			continue
		}
		// The donna crypto library needs architecture specific linking
		if strings.HasSuffix(dep[1], "-c64") {
			for _, arch := range []string{"amd64", "arm64"} {
				gofile := strings.Replace(dep[1], "/", "_", -1) + "_" + arch + ".go"
				buff := new(bytes.Buffer)
				if err := tmpl.Execute(buff, map[string]string{
					"TargetFilter": targetFilter,
					"File":         dep[1],
				}); err != nil {
					return err
				}
				err = ioutil.WriteFile(filepath.Join(root, "libtor", runtime.GOOS+"_tor_"+gofile), buff.Bytes(), 0644)
				if err != nil {
					return err
				}
			}
			for _, arch := range []string{"386", "arm"} {
				gofile := strings.Replace(dep[1], "/", "_", -1) + "_" + arch + ".go"
				buff := new(bytes.Buffer)
				if err := tmpl.Execute(buff, map[string]string{
					"TargetFilter": targetFilter,
					"File":         strings.Replace(dep[1], "-c64", "", -1),
				}); err != nil {
					return err
				}
				err = ioutil.WriteFile(filepath.Join(root, "libtor", runtime.GOOS+"_tor_"+gofile), buff.Bytes(), 0644)
				if err != nil {
					return err
				}
			}
			continue
		}
		// Anything else gets wrapped directly
		gofile := strings.Replace(dep[1], "/", "_", -1) + ".go"
		buff := new(bytes.Buffer)
		if err := tmpl.Execute(buff, map[string]string{
			"TargetFilter": targetFilter,
			"File":         dep[1],
		}); err != nil {
			return err
		}
		err = ioutil.WriteFile(filepath.Join(root, "libtor", runtime.GOOS+"_tor_"+gofile), buff.Bytes(), 0644)
		if err != nil {
			return err
		}
	}
	tmpl, err = template.New("").Parse(torPreamble)
	if err != nil {
		return err
	}
	buff := new(bytes.Buffer)
	if err := tmpl.Execute(buff, map[string]string{
		"TargetFilter": targetFilter,
		"Target":       runtime.GOOS,
	}); err != nil {
		return err
	}
	err = ioutil.WriteFile(filepath.Join(root, "libtor", runtime.GOOS+"_tor_preamble.go"), buff.Bytes(), 0644)
	if err != nil {
		return err
	}

	// Inject the configuration headers and ensure everything builds
	os.MkdirAll(filepath.Join(root, "tor_config"), 0755)

	for _, arch := range []string{"", ".linux64", ".linux32", ".android64", ".android32", ".macos64", ".ios64"} {
		blob, err := ioutil.ReadFile(filepath.Join(root, "config", "tor", fmt.Sprintf("orconfig%s.h", arch)))
		if err != nil {
			return err
		}
		tmpl, err := template.New("").Parse(string(blob))
		if err != nil {
			return err
		}
		buff := new(bytes.Buffer)
		if err := tmpl.Execute(buff, struct{ StrVer string }{string(strver)}); err != nil {
			return err
		}
		err = ioutil.WriteFile(filepath.Join(root, "tor_config", fmt.Sprintf("orconfig%s.h", arch)), buff.Bytes(), 0644)
		if err != nil {
			return err
		}
	}
	blob, err := ioutil.ReadFile(filepath.Join(root, "config", "tor", "micro-revision.i"))
	if err != nil {
		return err
	}
	err = ioutil.WriteFile(filepath.Join(root, "tor_config", "micro-revision.i"), blob, 0644)
	if err != nil {
		return err
	}

	// Copy and fill out the libtor entrypoint wrappers and the readme template.
	blob, err = ioutil.ReadFile(filepath.Join(root, "build", "libtor_external.go.in"))
	if err != nil {
		return err
	}
	err = ioutil.WriteFile(filepath.Join(root, "libtor.go"), blob, 0644)
	if err != nil {
		return err
	}
	blob, err = ioutil.ReadFile(filepath.Join(root, "build", "libtor_internal.go.in"))
	if err != nil {
		return err
	}
	err = ioutil.WriteFile(filepath.Join(root, "libtor", "libtor.go"), blob, 0644)
	if err != nil {
		return err
	}
	return nil
}

// torPreamble is the CGO preamble injected to configure the C compiler.
var torPreamble = `// go-libtor - Self-contained Tor from Go
// Copyright (c) 2018 Péter Szilágyi. All rights reserved.
// +build {{.TargetFilter}}

package libtor

/*
#cgo CFLAGS: -I${SRCDIR}/../tor_config
#cgo CFLAGS: -I${SRCDIR}/../{{.Target}}/tor
#cgo CFLAGS: -I${SRCDIR}/../{{.Target}}/tor/src
#cgo CFLAGS: -I${SRCDIR}/../{{.Target}}/tor/src/core/or
#cgo CFLAGS: -I${SRCDIR}/../{{.Target}}/tor/src/ext
#cgo CFLAGS: -I${SRCDIR}/../{{.Target}}/tor/src/ext/trunnel
#cgo CFLAGS: -I${SRCDIR}/../{{.Target}}/tor/src/feature/api

#cgo CFLAGS: -DED25519_CUSTOMRANDOM -DED25519_CUSTOMHASH -DED25519_SUFFIX=_donna

#cgo LDFLAGS: -lm
*/
import "C"
`

// torTemplate is the source file template used in Tor Go wrappers.
var torTemplate = `// go-libtor - Self-contained Tor from Go
// Copyright (c) 2018 Péter Szilágyi. All rights reserved.
// +build {{.TargetFilter}}

package libtor

/*
#include <../{{.File}}.c>
*/
import "C"
`
