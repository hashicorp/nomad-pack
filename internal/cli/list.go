// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package cli

import (
	"github.com/posener/complete"

	"github.com/hashicorp/nomad-pack/internal/pkg/cache"
	"github.com/hashicorp/nomad-pack/internal/pkg/flag"
)

// ListCommand lists all registries and pack that have been downloaded
// to the current machine.
type ListCommand struct {
	*baseCommand
	registry string
	ref      string
}

func (c *ListCommand) Run(args []string) int {
	c.cmdKey = "list"
	// Initialize. If we fail, we just exit since Init handles the UI.
	if err := c.Init(
		WithNoArgs(args),
		WithFlags(c.Flags()),
		WithNoConfig(),
		WithClient(false),
	); err != nil {
		c.ui.ErrorWithUsageAndContext(err, ErrParsingArgsOrFlags, c)
		return 1
	}

	// Get the global cache dir - may be configurable in the future, so using this
	// helper function rather than a direct reference to the CONST.
	globalCache, err := cache.NewCache(&cache.CacheConfig{
		Path:   cache.DefaultCachePath(),
		Logger: c.ui,
	})
	if err != nil {
		return 1
	}

	// Load the list of registries.
	err = globalCache.Load()
	if err != nil {
		return 1
	}

	// Iterate over the registries and build a table row for each cachedRegistry/pack
	// entry at each ref. Hierarchically, this should equate to the default
	// cachedRegistry and all its peers.
	table := packTable()
	if len(globalCache.Registries()) > 0 {
		for _, cachedRegistry := range globalCache.Registries() {
			// filter by registry name if provided...
			if c.registry != "" && cachedRegistry.Name != c.registry {
				continue
			}
			// ...and filter by registry ref, too
			if c.ref != "" && cachedRegistry.LocalRef != c.ref {
				continue
			}
			for _, registryPack := range cachedRegistry.Packs {
				tableRow := packRow(cachedRegistry, registryPack)
				table.Rows = append(table.Rows, tableRow)
			}
		}
	}

	// Display output table if any entries present
	if len(table.Rows) > 0 {
		c.ui.Table(table)
	} else {
		c.ui.Output("No packs present in the cache.")
	}

	return 0
}

func (c *ListCommand) Flags() *flag.Sets {
	return c.flagSet(flagSetOperation|flagSetNomadClient, func(set *flag.Sets) {
		f := set.NewSet("List Options")

		f.StringVar(&flag.StringVar{
			Name:    "registry",
			Target:  &c.registry,
			Default: "",
			Usage:   `Registry name to filter packs by.`,
		})

		f.StringVar(&flag.StringVar{
			Name:    "ref",
			Target:  &c.ref,
			Default: "",
			Usage:   `Registry ref to filter packs by.`,
		})

	})
}

func (c *ListCommand) AutocompleteArgs() complete.Predictor {
	return complete.PredictNothing
}

func (c *ListCommand) AutocompleteFlags() complete.Flags {
	return c.Flags().Completions()
}

func (c *ListCommand) Synopsis() string {
	return "List packs available in the local environment."
}

func (c *ListCommand) Help() string {
	c.Example = `
	# List all available packs
	nomad-pack list
	`
	return formatHelp(`
	Usage: nomad-pack list

	List nomad packs.

` + c.GetExample() + c.Flags().Help())
}
