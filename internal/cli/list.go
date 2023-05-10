// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package cli

import (
	"github.com/hashicorp/nomad-pack/internal/pkg/cache"
	"github.com/hashicorp/nomad-pack/internal/pkg/flag"
	"github.com/posener/complete"
)

// ListCommand lists all registries and pack that have been downloaded
// to the current machine.
type ListCommand struct {
	*baseCommand
}

func (c *ListCommand) Run(args []string) int {
	c.cmdKey = "registry list"
	// Initialize. If we fail, we just exit since Init handles the UI.
	if err := c.Init(
		WithNoArgs(args),
		WithNoConfig(),
		WithClient(false),
	); err != nil {
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

	// Initialize a table for a nice glint UI rendering
	table := packTable()

	// Iterate over the registries and build a table row for each cachedRegistry/pack
	// entry at each ref. Hierarchically, this should equate to the default
	// cachedRegistry and all its peers.
	for _, cachedRegistry := range globalCache.Registries() {
		for _, registryPack := range cachedRegistry.Packs {
			tableRow := packRow(cachedRegistry, registryPack)
			// append table row
			table.Rows = append(table.Rows, tableRow)
		}
	}

	// Display output table
	c.ui.Table(table)

	return 0
}

func (c *ListCommand) Flags() *flag.Sets {
	return c.flagSet(0, nil)
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
