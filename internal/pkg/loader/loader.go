// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package loader

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/hashicorp/hcl/v2/hclsimple"
	"github.com/hashicorp/nomad-pack/sdk/pack"
)

func Load(name string) (*pack.Pack, error) {
	fi, err := os.Stat(name)
	if err != nil {
		return nil, err
	}
	if !fi.IsDir() {
		return nil, errors.New("unable to load non-directory pack")
	}
	return loadDir(name)
}

func loadDir(dir string) (*pack.Pack, error) {
	abs, err := filepath.Abs(dir)
	if err != nil {
		return nil, err
	}

	var files []*pack.File
	abs += string(filepath.Separator)

	walkFn := func(name string, fi os.FileInfo, err error) error {

		if fi.IsDir() {
			return nil
		}

		if err != nil {
			return err
		}

		n := strings.TrimPrefix(name, abs)
		if n == "" {
			return nil
		}

		// Normalize to / since it will also work on Windows
		n = filepath.ToSlash(n)

		if !fi.Mode().IsRegular() {
			return fmt.Errorf("cannot load irregular file %q", name)
		}

		content, err := os.ReadFile(name)
		if err != nil {
			return fmt.Errorf("failed to read %s: %v", n, err)
		}

		content = bytes.TrimPrefix(content, []byte{0xEF, 0xBB, 0xBF})

		files = append(files, &pack.File{Name: n, Path: name, Content: content})
		return nil
	}

	if err = walk(abs, walkFn); err != nil {
		return nil, err
	}
	return loadFiles(files)
}

func loadFiles(files []*pack.File) (*pack.Pack, error) {

	p := new(pack.Pack)

	for _, f := range files {
		switch {
		case f.Name == "metadata.hcl":

			// Decode the metadata file into the pack. There shouldn't be more
			// than one per pack, but if there is, last found wins.
			if p.Metadata == nil {
				p.Metadata = new(pack.Metadata)
			}
			if err := hclsimple.Decode(f.Name, f.Content, nil, p.Metadata); err != nil {
				return p, fmt.Errorf("failed to decode %s: %v", f.Name, err)
			}

		case f.Name == "variables.hcl":
			p.RootVariableFile = f

		case f.Name == "outputs.tpl":
			// This sets the default output template file. It can be overridden
			// from the CLI.
			p.OutputTemplateFile = f

		case strings.HasPrefix(f.Name, "templates/") &&
			strings.HasSuffix(f.Name, ".nomad.tpl") ||
			strings.Contains(f.Name, "templates/_"):
			// The file is a pack template file. This catches both full Nomad
			// object templates and helpers.
			p.TemplateFiles = append(p.TemplateFiles, f)

		case strings.HasPrefix(f.Name, "templates/") &&
			strings.HasSuffix(f.Name, ".tpl"):
			// if there are any other files inside the "templates/" directory,
			// add them to the aux files array
			p.AuxiliaryFiles = append(p.AuxiliaryFiles, f)

		default:
			// Do nothing with other files; this might change in the future.
			continue
		}
	}

	// Validate the metadata.
	if p.Metadata == nil {
		return p, errors.New("metadata.hcl file not found")
	}
	return p, p.Metadata.Validate()
}
