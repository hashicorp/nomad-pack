package cli

import (
	"errors"
	"fmt"
	"strings"

	"github.com/hashicorp/nomad-pack/flag"
	"github.com/posener/complete"
)

type RegistryDeleteCommand struct {
	*baseCommand
	command string
	name    string
	target  string
}

func (c *RegistryDeleteCommand) Run(args []string) int {
	c.cmdKey = "registry delete"
	flagSet := c.Flags()

	// Initialize. If we fail, we just exit since Init handles the UI.
	if err := c.Init(
		WithNoArgs(args),
		WithFlags(flagSet),
		WithNoConfig(),
		WithClient(false),
	); err != nil {
		return 1
	}

	return c.run()
}

func (c *RegistryDeleteCommand) run() int {
	// Ensure a name is passed
	if c.name == "" {
		c.ui.ErrorWithContext(errors.New("registry name is required"), "error deleting registry")
		return 1
	}

	// Get the global cache dir - may be configurable in the future, so using this
	// helper function rather than a direct reference to the CONST.
	cacheDir, err := globalCacheDir()
	if err != nil {
		c.ui.ErrorWithContext(err, "error retrieving global cache directory")
		return 1
	}

	if err = deleteRegistry(cacheDir, c.name, c.target, c.ui); err != nil {
		c.ui.ErrorWithContext(err, "error deleting registry")
		return 1
	}

	// Format output based on passed flags.
	if c.target == "" {
		if strings.Contains(c.name, "@") {
			c.ui.Info(fmt.Sprintf("packs in registry %s that match version have been deleted", c.name))
		} else {
			c.ui.Info(fmt.Sprintf("registry %s deleted", c.name))
		}
	} else {
		c.ui.Info(fmt.Sprintf("registry %s target %s deleted", c.name, c.target))
	}

	return 0
}

func (c *RegistryDeleteCommand) Flags() *flag.Sets {
	return c.flagSet(0, func(set *flag.Sets) {
		f := set.NewSet("Registry Options")

		f.StringVar(&flag.StringVar{
			Name:    "name",
			Target:  &c.name,
			Default: "",
			Usage: `Global cache name for the registry to delete. To target a 
specific version of the packs within a registry, append the @version to the 
registry name. If no version is specified, the entire registry will be deleted.
`,
		})

		f.StringVar(&flag.StringVar{
			Name:    "target",
			Target:  &c.target,
			Default: "",
			Usage: `A specific pack within the registry to be deleted. This 
should be the subdirectory containing the pack. It is assumed that the subdirectory 
will be directly beneath the registry root. Deeply nested directories are not 
supported at this time. If a version specifier has been added to the name option, 
only that version of the target pack will be deleted.
`,
		})
	})
}

func (c *RegistryDeleteCommand) AutocompleteArgs() complete.Predictor {
	return complete.PredictNothing
}

func (c *RegistryDeleteCommand) AutocompleteFlags() complete.Flags {
	return c.Flags().Completions()
}

func (c *RegistryDeleteCommand) Synopsis() string {
	return "Delete registries or packs from the local environment."
}

func (c *RegistryDeleteCommand) Help() string {
	c.Example = `
	# Delete a pack registry at a specific tag/release/SHA/latest at tag/release/SHA/latest defined.
	If no tag/release/SHA defined, will delete all versions of registry.

	nomad-pack registry delete --from=https://github.com/hashicorp/nomad-pack-registry@v0.1.0 --target=example_nomad
	`
	return formatHelp(`
	Usage: nomad-pack registry delete [options]

	Delete nomad pack registries or packs.
	
` + c.GetExample() + c.Flags().Help())
}
