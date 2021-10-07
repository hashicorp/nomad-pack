package cli

import (
	"github.com/hashicorp/nomad-pack/flag"
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

	return c.run()
}

func (c *RegistryListCommand) run() int {
	c.cmdKey = "registry list"
	if err := listRegistries(c.ui); err != nil {
		c.ui.ErrorWithContext(err, "error listing registries")
		return 1
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
