package cli

import (
	"github.com/hashicorp/nomad-pack/internal/pkg/cache"
	"github.com/hashicorp/nomad-pack/internal/pkg/flag"
	"github.com/posener/complete"
)

// RegistryListCommand lists all registries and pack that have been downloaded
// to the current machine.
type RegistryListCommand struct {
	*baseCommand
	command string
}

func (c *RegistryListCommand) Run(args []string) int {
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
	table := registryTable()

	// Iterate over the registries and build a table row for each cachedRegistry/pack
	// entry at each ref. Hierarchically, this should equate to the default
	// cachedRegistry and all its peers.
	for _, cachedRegistry := range globalCache.Registries() {
		// If no packs, just show registry.
		if cachedRegistry.Packs == nil || len(cachedRegistry.Packs) == 0 {
			tableRow := emptyRegistryTableRow(cachedRegistry)
			// append table row
			table.Rows = append(table.Rows, tableRow)
		} else {
			// Show registry/pack combo for each pack.
			for _, registryPack := range cachedRegistry.Packs {
				tableRow := registryPackRow(cachedRegistry, registryPack)
				// append table row
				table.Rows = append(table.Rows, tableRow)
			}
		}
	}

	// Display output table
	c.ui.Table(table)

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
	return "List registries and packs available in the local environment."
}

func (c *RegistryListCommand) Help() string {
	c.Example = `
	# List all available registries and their packs
	nomad-pack registry list
	`
	return formatHelp(`
	Usage: nomad-pack registry list

	List nomad pack registries.
	
` + c.GetExample() + c.Flags().Help())
}
