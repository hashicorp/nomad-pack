package deps

import (
	"context"
	"fmt"
	"path"

	gg "github.com/hashicorp/go-getter"
	"github.com/hashicorp/hcl/v2/hclsimple"

	"github.com/hashicorp/nomad-pack/internal/pkg/cache"
	"github.com/hashicorp/nomad-pack/sdk/pack"
	"github.com/hashicorp/nomad-pack/terminal"
)

// Vendor reads the metadata.hcl from the provided directory, downloads
// dependencies, and adds them to a "vendor" registry if copyToCache flag is
// set.
func Vendor(ctx context.Context, ui terminal.UI, globalCache *cache.Cache, targetPath string, copyToCache bool) error {
	// attempt to read metadata.hcl
	metadata := &pack.Metadata{}
	err := hclsimple.DecodeFile(path.Join(targetPath, "metadata.hcl"), nil, metadata)
	if err != nil {
		return err
	}

	if len(metadata.Dependencies) == 0 {
		return fmt.Errorf("metadata.hcl file does not contain any dependencies")
	}

	for _, d := range metadata.Dependencies {
		targetDir := path.Join(targetPath, "deps", d.Name)

		// download each dependency
		ui.Info(fmt.Sprintf("downloading %v pack to %v...", d.Name, targetDir))
		if err := gg.Get(targetDir, fmt.Sprintf(d.Source), gg.WithContext(ctx)); err != nil {
			return fmt.Errorf("error downloading dependency: %v", err)
		}
		ui.Success("...success!")

		if copyToCache {
			// and add them to a "vendor" registry in the cache
			if err := globalCache.AddVendoredPack(&cache.AddOpts{
				Source:          d.Source,
				PackName:        d.Name,
				CurrentLocation: targetDir,
			}); err != nil {
				return err
			}
		}
	}
	return nil
}
