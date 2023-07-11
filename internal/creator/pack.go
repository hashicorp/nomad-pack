// Copyright (c) HashiCorp, Inc.
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

type packCreator struct {
	name    string
	path    string
	tplPath string
}

// CreatePack build a skeleton pack containing:
// - A README.md file containing a human-readable description of the pack,
//   often including any dependency information.
// - A metadata.hcl file containing information about the pack.
// - A variables.hcl file that defines the variables in a pack.
// - A CHANGELOG.md file that lists changes for each version of the pack.
// - An outputs.tpl file that defines an output to be printed when a pack is
//   deployed.
// - A templates subdirectory containing the HCL templates used to render the
//   jobspec.
// - A jobspec template for the hello-world-service container.

func CreatePack(c config.PackConfig) error {
	ui := c.GetUI()

	if c.OutPath == "" {
		// This should never happen
		return newCreatePackError(fmt.Errorf("failed to parse output path for pack"))
	}

	_, err := os.Stat(c.OutPath)
	if err == nil && !c.Overwrite {
		return newCreatePackError(
			fmt.Errorf("%s appears to be non-empty, re-run the command with --overwrite to overwrite", c.OutPath),
		)
	}
	ui.Output("Creating %q Pack in %q...\n", c.PackName, c.OutPath)

	pc := packCreator{
		name:    c.PackName,
		path:    path.Join(c.OutPath, c.PackName),
		tplPath: path.Join(c.OutPath, c.PackName, "templates"),
	}

	err = os.MkdirAll(pc.tplPath, cache.DefaultDirPerms)
	if err != nil {
		return newCreatePackError(err)
	}

	err = pc.createReadmeFile()
	if err != nil {
		return newCreatePackError(err)
	}

	err = pc.createMetadataFile()
	if err != nil {
		return newCreatePackError(err)
	}

	err = pc.createVariablesFile()
	if err != nil {
		return newCreatePackError(err)
	}

	err = pc.createChangelogFile()
	if err != nil {
		return newCreatePackError(err)
	}

	err = pc.createOutputTemplateFile()
	if err != nil {
		return newCreatePackError(err)
	}

	err = pc.createJobTemplateFile()
	if err != nil {
		return newCreatePackError(err)
	}

	err = pc.createJobTemplateHelpersFile()
	if err != nil {
		return newCreatePackError(err)
	}
	ui.Output("Done.")
	return nil
}

func (pc packCreator) createMetadataFile() error {
	return pc.createPackFile(config.FileNameMetadata, "pack_metadata.hcl")
}

func (pc packCreator) createReadmeFile() error {
	return pc.createPackFile(config.FileNameReadme, "pack_readme.md")
}

func (pc packCreator) createChangelogFile() error {
	return pc.createPackFile(config.FileNameChangelog, "changelog.md")
}

func (pc packCreator) createVariablesFile() error {
	return pc.createPackFile(config.FileNameVariables, "pack_variables.hcl")
}

func (pc packCreator) createOutputTemplateFile() error {
	return pc.createPackFile(config.FileNameOutputs, "pack_output.tpl")
}

func (pc packCreator) createJobTemplateFile() error {
	return pc.createPackTemplateFile(fmt.Sprintf("%s.nomad.tpl", pc.name), "pack_jobspec.tpl")
}

func (pc packCreator) createJobTemplateHelpersFile() error {
	return pc.createPackTemplateFile("_helpers.tpl", "pack_helpers.tpl")
}

func (pc packCreator) createPackTemplateFile(filename, template string, fixups ...func(string) string) error {
	return pc.createPackFile(path.Join("templates", filename), template, fixups...)
}
func (pc packCreator) createPackFile(filename, template string, fixups ...func(string) string) error {
	dest := path.Join(pc.path, filename)
	f, err := os.Create(dest)
	defer func() {
		_ = f.Close()
	}()

	if err != nil {
		out := &fs.PathError{
			Op:   "createPackFile.create",
			Err:  err,
			Path: dest,
		}
		return out
	}

	err = tpl.ExecuteTemplate(f, template, packDataFromCreator(pc))
	if err != nil {
		out := &fs.PathError{
			Op:   "createPackFile.executeTemplate",
			Err:  err,
			Path: dest,
		}
		return out
	}
	return nil
}

func packDataFromCreator(pc packCreator) map[string]string {
	return map[string]string{
		"PackName": pc.name,
	}
}

// newCreatePackError makes error handling for the method consistent.
func newCreatePackError(err error) error {
	return fmt.Errorf("create pack error: %w", err)
}
