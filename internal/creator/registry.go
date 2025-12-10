// Copyright IBM Corp. 2021, 2025
// SPDX-License-Identifier: MPL-2.0

package creator

import (
	"fmt"
	"io/fs"
	"os"
	"path"

	"github.com/hashicorp/nomad-pack/internal/config"
	"github.com/hashicorp/nomad-pack/internal/pkg/cache"
)

type registryCreator struct {
	name             string
	path             string
	packsPath        string
	createSamplePack bool
}

// CreateRegistry build a skeleton registry containing:
//   - A README.md file containing a human-readable description of the registry,
//     often including any dependency information.
//   - A CHANGELOG.md file that lists changes for each version of the pack.
//   - A packs folder that contains all of the packs present in the registry
//   - Optionally, a sample pack to get started with
func CreateRegistry(c config.PackConfig) error {
	ui := c.GetUI()
	outPath := c.OutPath
	if outPath == "" {
		outPath = "."
	}
	ui.Output("Creating %q Registry in %q...\n", c.RegistryName, outPath)

	rc := registryCreator{
		name:             c.RegistryName,
		path:             path.Join(outPath, c.RegistryName),
		packsPath:        path.Join(outPath, c.RegistryName, "packs"),
		createSamplePack: c.CreateSamplePack,
	}

	// TODO: Make this optional
	// TODO: Make this interactive

	err := os.MkdirAll(rc.packsPath, cache.DefaultDirPerms)
	if err != nil {
		return newCreateRegistryError(err)
	}

	err = rc.createReadmeFile()
	if err != nil {
		return newCreateRegistryError(err)
	}
	err = rc.createChangelogFile()
	if err != nil {
		return newCreateRegistryError(err)
	}

	if rc.createSamplePack {
		c.PackName = "hello_world"
		c.OutPath = rc.packsPath
		err = CreatePack(c)
		if err != nil {
			return err
		}
	}
	ui.Output("Done.\n")
	return nil
}

func regDataFromCreator(rc registryCreator) map[string]string {
	return map[string]string{
		"RegistryName": rc.name,
	}
}

// newCreatePackError makes error handling for the method consistent.
func newCreateRegistryError(err error) error {
	return fmt.Errorf("create registry error: %w", err)
}

func (rc registryCreator) createReadmeFile() error {
	return rc.createRegistryFile(config.FileNameReadme, "registry_readme.md")
}

func (rc registryCreator) createChangelogFile() error {
	return rc.createRegistryFile(config.FileNameChangelog, "changelog.md")
}

func (rc registryCreator) createRegistryFile(filename, template string, fixups ...func(string) string) error {
	dest := path.Join(rc.path, filename)
	f, err := os.Create(dest)
	defer func() {
		_ = f.Close()
	}()

	if err != nil {
		out := &fs.PathError{
			Op:   "createRegistryFile.create",
			Err:  err,
			Path: dest,
		}
		return out
	}

	err = tpl.ExecuteTemplate(f, template, regDataFromCreator(rc))
	if err != nil {
		out := &fs.PathError{
			Op:   "createRegistryFile.executeTemplate",
			Err:  err,
			Path: dest,
		}
		return out
	}
	return nil
}
