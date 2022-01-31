package cli

import (
	"fmt"
	"strings"

	"github.com/hashicorp/nomad-pack/internal/pkg/cache"
	"github.com/hashicorp/nomad-pack/internal/pkg/errors"
	"github.com/hashicorp/nomad-pack/internal/pkg/flag"
	"github.com/posener/complete"
)

// RegistryAddCommand adds a regsitry to the global cache.
type RegistryAddCommand struct {
	*baseCommand
	command string
	source  string
	name    string
	target  string
	ref     string
}

func (c *RegistryAddCommand) Run(args []string) int {
	c.cmdKey = "registry add"
	flagSet := c.Flags()

	// Initialize. If we fail, we just exit since Init handles the UI.
	if err := c.Init(
		WithExactArgs(2, args),
		WithFlags(flagSet),
		WithNoConfig(),
		WithClient(false),
	); err != nil {
		c.ui.ErrorWithContext(err, "error parsing args or flags")
		return 1
	}

	// Generate our UI error context.
	errorContext := errors.NewUIErrorContext()

	c.name = args[0]
	c.source = args[1]

	errorContext.Add(errors.UIContextPrefixRegistryName, c.name)
	errorContext.Add(errors.UIContextPrefixGitRegistryURL, c.source)

	if c.target != "" {
		errorContext.Add(errors.UIContextPrefixRegistryTarget, c.target)
	}

	// Add the registry or registry target to the global cache
	globalCache, err := cache.NewCache(&cache.CacheConfig{
		Path:   cache.DefaultCachePath(),
		Logger: c.ui,
	})
	if err != nil {
		return 1
	}

	newRegistry, err := globalCache.Add(&cache.AddOpts{
		RegistryName: c.name,
		Source:       c.source,
		PackName:     c.target,
		Ref:          c.ref,
	})
	if err != nil {
		return 1
	}

	// If subprocess fails to add any packs, report this to the user.
	if len(newRegistry.Packs) == 0 {
		c.ui.ErrorWithContext(errors.New("failed to add packs for registry"), "see output for reason", errorContext.GetAll()...)
		return 1
	}

	// Initialize output table
	table := registryTable()
	var validPack *cache.Pack
	// If only targeting a single pack, only output a single row
	if c.target != "" {
		// It is safe to target pack 0 here because registry.AddFromGitURL will
		// ensure only the target pack is returned.
		tableRow := registryPackRow(newRegistry, newRegistry.Packs[0])
		table.Rows = append(table.Rows, tableRow)
		for _, registryPack := range newRegistry.Packs {
			if !strings.Contains(strings.ToLower(registryPack.Ref), "invalid") {
				validPack = registryPack
			}
		}
	} else {
		for _, registryPack := range newRegistry.Packs {
			tableRow := registryPackRow(newRegistry, registryPack)
			table.Rows = append(table.Rows, tableRow)
			// Grab a successful pack to show extra help text.
			if validPack == nil &&
				!strings.Contains(strings.ToLower(registryPack.Ref), "invalid") {
				validPack = registryPack
			}
		}
	}

	c.ui.Info("Registry successfully added to cache.")
	c.ui.Table(table)

	if validPack != nil {
		c.ui.Info(fmt.Sprintf("Try running one the packs you just added liked this\n\n  nomad-pack run %s --registry=%s --ref=%s", validPack.Name(), newRegistry.Name, validPack.Ref))
	}

	return 0
}

func (c *RegistryAddCommand) Flags() *flag.Sets {
	return c.flagSet(0, func(set *flag.Sets) {
		f := set.NewSet("Registry Options")

		f.StringVar(&flag.StringVar{
			Name:    "target",
			Target:  &c.target,
			Default: "",
			Usage:   `A specific pack within the registry to be added.`,
		})

		f.StringVar(&flag.StringVar{
			Name:    "ref",
			Target:  &c.ref,
			Default: "",
			Usage: `Specific git ref of the registry or pack to be added. 
Supports tags, SHA, and latest. If no ref is specified, defaults to latest. 
Running "nomad registry add" multiple times for the same ref is idempotent, 
however running "nomad-pack registry add" without specifying a ref, or when 
specifying @latest, is destructive, and will overwrite current @latest in the global cache.

Using ref with a file path is not supported.`,
		})
	})
}

func (c *RegistryAddCommand) AutocompleteArgs() complete.Predictor {
	return complete.PredictNothing
}

func (c *RegistryAddCommand) AutocompleteFlags() complete.Flags {
	return c.Flags().Completions()
}

func (c *RegistryAddCommand) Synopsis() string {
	return "Add registries or packs to the local environment."
}

func (c *RegistryAddCommand) Help() string {
	c.Example = `
	# Download latest ref of the pack registry to the global cache.
	nomad-pack registry add community github.com/hashicorp/nomad-pack-community-registry

	# Download latest ref of a specific pack from the registry to the global cache.
	nomad-pack registry add community github.com/hashicorp/nomad-pack-community-registry --target=nomad_example

	# Download packs from a registry at a specific tag/release/SHA.
	nomad-pack registry add community github.com/hashicorp/nomad-pack-community-registry  --ref=v0.1.0
	`
	return formatHelp(`
	Usage: nomad-pack registry add <name> <source> [options]

	Add nomad pack registries.
	
` + c.GetExample() + c.Flags().Help())
}
