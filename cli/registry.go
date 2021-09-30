package cli

import (
	"fmt"

	"github.com/hashicorp/nomad-pack/flag"
	"github.com/posener/complete"
)

type RegistryCommand struct {
	*baseCommand
	command string
	from    string
	alias   string
	target  string
}

func (c *RegistryCommand) Run(args []string) int {
	c.cmdKey = "registry"
	flagSet := c.Flags()

	// Initialize. If we fail, we just exit since Init handles the UI.
	if err := c.Init(
		WithMinimumNArgs(1, args),
		WithFlags(flagSet),
		WithNoConfig(),
		WithClient(false),
	); err != nil {
		c.ui.ErrorWithContext(err, "invalid registry args")
		return 1
	}

	c.command = args[0]

	switch c.command {
	case "add":
		return c.add()
	case "delete":
		return c.delete()
	case "list":
		return c.list()
	default:
		c.ui.Error(fmt.Sprintf("invalid registry command: %s", c.command))
		c.ui.Info(c.Help())
		return 1
	}
}

func (c *RegistryCommand) add() int {
	c.ui.Error("add not implemented")
	return 1
}

func (c *RegistryCommand) delete() int {
	c.ui.Error("delete not implemented")
	return 1
}

func (c *RegistryCommand) list() int {
	if err := listRegistries(c.ui); err != nil {
		c.ui.ErrorWithContext(err, "error listing registries")
		return 1
	}

	return 0
}

func (c *RegistryCommand) Flags() *flag.Sets {
	return c.flagSet(flagSetOperation, func(set *flag.Sets) {
		f := set.NewSet("Registry Options")

		f.StringVar(&flag.StringVar{
			Name:    "from",
			Target:  &c.from,
			Default: "",
			Usage: `The remote pack registry to be added. 
At this time, this must be a url that points to a git repository. Supports version with @ syntax.
For example, "nomad-pack registry add --from=https://github.com/hashicorp/nomad-pack-registry@v1.0.0".
Supports tags, releases, SHA, and latest. If no version specified, defaults 
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
Allows users to override remote registry name in the global registry cache. 
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

func (c *RegistryCommand) AutocompleteArgs() complete.Predictor {
	return complete.PredictNothing
}

func (c *RegistryCommand) AutocompleteFlags() complete.Flags {
	return c.Flags().Completions()
}

func (c *RegistryCommand) Synopsis() string {
	return "Used to manage registries for Nomad Pack"
}

func (c *RegistryCommand) Help() string {
	c.Example = `
	# Download latest version of the pack registry to the global cache.
	nomad-pack registry add --from=https://github.com/hashicorp/nomad-pack-registry

	# Download latest version of a specific pack from the registry to the global cache.
	nomad-pack registry add --from=https://github.com/hashicorp/nomad-pack-registry --target=nomad_example

	# Download a pack registry at a specific tag/release/SHA/latest to the global cache.
	nomad-pack registry add --from=https://github.com/hashicorp/nomad-pack-registry@v0.1.0

	# Delete a pack registry at a specific tag/release/SHA/latest at tag/release/SHA/latest defined. If no tag/release/SHA defined, will delete all versions of registry.
	nomad-pack registry add --from=https://github.com/hashicorp/nomad-pack-registry@v0.1.0

	# List all available registries and their packs
	nomad-pack registry list
	`
	return formatHelp(`
	Usage: nomad-pack registry <sub-command> [options]

	Manage nomad pack registries.
	
` + c.GetExample() + c.Flags().Help())
}
