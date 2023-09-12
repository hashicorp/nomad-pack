// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package deps

import (
	"context"
	"errors"
	"fmt"
	"path"

	gg "github.com/hashicorp/go-getter"
	"github.com/hashicorp/hcl/v2/hclsimple"

	"github.com/hashicorp/nomad-pack/sdk/pack"
	"github.com/hashicorp/nomad-pack/terminal"
)

// Vendor reads the metadata.hcl from the provided directory and downloads
// dependencies
func Vendor(ctx context.Context, ui terminal.UI, targetPath string) error {
	// attempt to read metadata.hcl
	metadata := &pack.Metadata{}
	err := hclsimple.DecodeFile(path.Join(targetPath, "metadata.hcl"), nil, metadata)
	if err != nil {
		return err
	}

	if len(metadata.Dependencies) == 0 {
		return errors.New("metadata.hcl file does not contain any dependencies")
	}

	for _, d := range metadata.Dependencies {
		targetDir := path.Join(targetPath, "deps", d.Name)

		// download each dependency
		ui.Info(fmt.Sprintf("downloading %v pack to %v...", d.Name, targetDir))
		if err := gg.Get(targetDir, fmt.Sprintf(d.Source), gg.WithContext(ctx)); err != nil {
			return fmt.Errorf("error downloading dependency: %v", err)
		}
		ui.Success("...success!")
	}
	return nil
}
