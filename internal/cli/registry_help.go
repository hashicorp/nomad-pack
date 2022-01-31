package cli

import (
	"github.com/hashicorp/nomad-pack/internal/pkg/flag"
	"github.com/posener/complete"
)

// RegistryHelpCommand exists solely to provide top level help for the registry
// set of subcommands.
type RegistryHelpCommand struct {
	*baseCommand
}

func (c *RegistryHelpCommand) Run(args []string) int {
	c.cmdKey = "registry"

	// Initialize. If we fail, we just exit since Init handles the UI.
	if err := c.Init(
		WithNoArgs(args),
		WithNoConfig(),
		WithClient(false),
	); err != nil {
		return 1
	}

	c.ui.Info("The registry command requires one of the following subcommands: add, delete, list.")

	return 0
}

func (c *RegistryHelpCommand) Flags() *flag.Sets {
	return c.flagSet(0, nil)
}

func (c *RegistryHelpCommand) AutocompleteArgs() complete.Predictor {
	return complete.PredictNothing
}

func (c *RegistryHelpCommand) AutocompleteFlags() complete.Flags {
	return c.Flags().Completions()
}

func (c *RegistryHelpCommand) Synopsis() string {
	return "Add, delete, or list registries and packs in the local environment."
}

func (c *RegistryHelpCommand) Help() string {
	return formatHelp(`
	Usage: nomad-pack registry <subcommand> [options]

	Manage nomad-pack registries or packs.
	
` + c.GetExample() + c.Flags().Help())
}
