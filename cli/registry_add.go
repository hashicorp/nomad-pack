package cli

import (
	"github.com/hashicorp/nomad-pack/flag"
	"github.com/posener/complete"
)

type RegistryAddCommand struct {
	*baseCommand
	command string
	from    string
	alias   string
	target  string
}

func (c *RegistryAddCommand) Run(args []string) int {
	c.cmdKey = "registry add"
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

func (c *RegistryAddCommand) run() int {
	// Get the global cache dir - may be configurable in the future, so using this
	// helper function rather than a direct reference to the CONST.
	cacheDir, err := globalCacheDir()
	if err != nil {
		c.ui.ErrorWithContext(err, "error retrieving global cache directory")
		return 1
	}

	if err = addRegistry(cacheDir, c.from, c.alias, c.target, c.ui); err != nil {
		c.ui.ErrorWithContext(err, "error adding registry")
		return 1
	}
	return 1
}

func (c *RegistryAddCommand) Flags() *flag.Sets {
	return c.flagSet(0, func(set *flag.Sets) {
		f := set.NewSet("Registry Options")

		f.StringVar(&flag.StringVar{
			Name:    "from",
			Target:  &c.from,
			Default: "",
			Usage: `The remote pack registry to be added. 
At this time, this must be a url that points to a git repository. Supports version with @ syntax.
For example, "nomad-pack registry add --from=github.com/hashicorp/nomad-pack-registry@v1.0.0".
Supports tags, releases, SHA, and latest. If no version is specified, defaults 
to latest. Running "nomad registry add" multiple times for the same version 
is idempotent, however running "nomad-pack registry add" without specifying 
a version, or when specifying @latest is destructive, and will overwrite 
current @latest in the global cache.`,
		})

		f.StringVar(&flag.StringVar{
			Name:    "alias",
			Target:  &c.alias,
			Default: "",
			Usage: `User defined name for the registry. 
Allows users to override the remote registry name in the global registry cache. 
By default, the source url will be turned into a slug and the registry will be 
named with the slug (e.g. github.com-hashicorp-nomad-pack-registry).`,
		})

		f.StringVar(&flag.StringVar{
			Name:    "target",
			Target:  &c.target,
			Default: "",
			Usage:   `A specific pack within the registry to be added or deleted.`,
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
	# Download latest version of the pack registry to the global cache.
	nomad-pack registry add --from=github.com/hashicorp/nomad-pack-registry

	# Download latest version of a specific pack from the registry to the global cache.
	nomad-pack registry add --from=github.com/hashicorp/nomad-pack-registry --target=nomad_example

	# Download a pack registry at a specific tag/release/SHA/latest to the global cache.
	nomad-pack registry add --from=github.com/hashicorp/nomad-pack-registry@v0.1.0
	`
	return formatHelp(`
	Usage: nomad-pack registry add [options]

	Add nomad pack registries.
	
` + c.GetExample() + c.Flags().Help())
}
