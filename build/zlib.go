// +build mage

package main

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"text/template"

	"github.com/magefile/mage/sh"
)

const (
	ZlibURL = "https://github.com/madler/zlib"
	ZlibTag = "v1.2.11"
)

// wrapZlib clones the zlib library into the local repository and wraps it into
// a Go package.
//
// Zlib is a small and simple C library which can be wrapped by inserting an empty
// Go file among the C sources, causing the Go compiler to pick up all the loose
// sources and build them together into a static library.
func WrapZlib(root string) error {
	zlibDir := filepath.Join(root, runtime.GOOS, "zlib")
	err := sh.Rm(zlibDir)
	if err != nil {
		return err
	}
	err = sh.Run("git", "clone", "--depth", "1", "-b", ZlibTag, ZlibURL, zlibDir)
	if err != nil {
		return fmt.Errorf("git clone failed: %w", err)
	}

	// Wipe everything from the library that's non-essential
	files, err := ioutil.ReadDir(zlibDir)
	if err != nil {
		return fmt.Errorf("failed to readdir %q: %w", zlibDir, err)
	}
	for _, file := range files {
		if file.IsDir() {
			err = os.RemoveAll(filepath.Join(zlibDir, file.Name()))
			if err != nil {
				return fmt.Errorf("failed to remove %q: %w", file.Name(), err)
			}
		} else if ext := filepath.Ext(file.Name()); ext != ".h" && ext != ".c" {
			err = os.Remove(filepath.Join(zlibDir, file.Name()))
			if err != nil {
				return fmt.Errorf("failed to remove %q: %w", file.Name(), err)
			}
		}
	}

	targetFilter := targetFilters[runtime.GOOS]

	// Generate Go wrappers for each C source individually
	tmpl, err := template.New("").Parse(zlibTemplate)
	if err != nil {
		return fmt.Errorf("failed to parse zlib source template: %w", err)
	}
	for _, file := range files {
		if file.IsDir() {
			continue
		}
		if ext := filepath.Ext(file.Name()); ext == ".c" {
			name := strings.TrimSuffix(file.Name(), ext)
			sourceFilename := filepath.Join(root, "libtor", runtime.GOOS+"_zlib_"+name+".go")
			fmt.Println(sourceFilename)
			err := func() error {
				sourceFile, err := os.Create(sourceFilename)
				if err != nil {
					return fmt.Errorf("failed to create %q: %w", sourceFilename, err)
				}
				if err := tmpl.Execute(sourceFile, map[string]string{
					"TargetFilter": targetFilter,
					"File":         name,
				}); err != nil {
					return fmt.Errorf("failed to execute zlib source template: %w", err)
				}
				return nil
			}()
			if err != nil {
				return err
			}
		}
	}

	tmpl, err = template.New("").Parse(zlibPreamble)
	if err != nil {
		return fmt.Errorf("failed to parse zlib preamble template: %w", err)
	}
	preambleFilename := filepath.Join(root, "libtor", runtime.GOOS+"_zlib_preamble.go")
	preambleFile, err := os.Create(preambleFilename)
	if err != nil {
		return fmt.Errorf("failed to create %q: %w", preambleFilename, err)
	}
	defer preambleFile.Close()
	err = tmpl.Execute(preambleFile, map[string]string{
		"TargetFilter": targetFilter,
		"Target":       runtime.GOOS,
	})
	if err != nil {
		return fmt.Errorf("failed to write %q: %w", preambleFilename, err)
	}

	return nil
}

// zlibPreamble is the CGO preamble injected to configure the C compiler.
var zlibPreamble = `// go-libtor - Self-contained Tor from Go
// Copyright (c) 2018 Péter Szilágyi. All rights reserved.
// +build {{.TargetFilter}}
// +build staticZlib

package libtor


/*
#cgo CFLAGS: -I${SRCDIR}/../{{.Target}}/zlib
#cgo CFLAGS: -DHAVE_UNISTD_H -DHAVE_STDARG_H
*/
import "C"
`

// zlibTemplate is the source file template used in zlib Go wrappers.
var zlibTemplate = `// go-libtor - Self-contained Tor from Go
// Copyright (c) 2018 Péter Szilágyi. All rights reserved.
// +build {{.TargetFilter}}
// +build staticZlib

package libtor

/*
#include <../zlib/{{.File}}.c>
*/
import "C"
`
