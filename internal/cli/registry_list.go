// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package cli

import (
	"github.com/posener/complete"

	"github.com/hashicorp/nomad-pack/internal/pkg/cache"
	"github.com/hashicorp/nomad-pack/internal/pkg/flag"
)

// RegistryListCommand lists all registries and pack that have been downloaded
// to the current machine.
type RegistryListCommand struct {
	*baseCommand
}

func (c *RegistryListCommand) Run(args []string) int {
	c.cmdKey = "registry list"

	if err := c.Init(
		WithNoArgs(args),
		WithNoConfig(),
		WithClient(false),
	); err != nil {
		c.ui.ErrorWithContext(err, ErrParsingArgsOrFlags)
		c.ui.Info(c.helpUsageMessage())
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
	if len(globalCache.Registries()) > 0 {
		table := registryTable()
		for _, registry := range globalCache.Registries() {
			tableRow := registryTableRow(registry)
			table.Rows = append(table.Rows, tableRow)
		}
		c.ui.Table(table)

		// TODO: This message is to make upgrading from tech preview versions
		// to 0.1 easier. Remove it at 0.2 release.
		if table.Rows[0][2] == "" {
			c.ui.Warning("It appears that you have a cache created before Nomad-Pack 0.1 (hence the\n" +
				"missing values in LOCAL REF and REGISTRY URL columns). We recommend deleting\n" +
				"your current cache with `nomad-pack registry delete <name>` and re-adding your\n" +
				"registries to take full advantage of new `nomad-pack registry list` and\n" +
				"`nomad-pack list` commands behavior.")
		}
	} else {
		c.ui.Output("No registries present in the cache.")
	}

	return 0
}

func (c *RegistryListCommand) Flags() *flag.Sets {
	return c.flagSet(0, nil)
}

func (c *RegistryListCommand) AutocompleteArgs() complete.Predictor {
	return complete.PredictNothing
}

func (c *RegistryListCommand) AutocompleteFlags() complete.Flags {
	return c.Flags().Completions()
}

func (c *RegistryListCommand) Synopsis() string {
	return "List registries configured in the local environment."
}

func (c *RegistryListCommand) Help() string {
	c.Example = `
	# List all configured registries
	nomad-pack registry list
	`
	return formatHelp(`
	Usage: nomad-pack registry list

	List nomad pack registries.

` + c.GetExample() + c.Flags().Help())
}
