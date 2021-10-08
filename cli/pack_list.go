package cli

import (
	"github.com/hashicorp/nomad-pack/flag"
	"github.com/posener/complete"
)

// ListCommand lists all packs in the specified registry
// or the packs in the default registry if no registry is specified
type ListCommand struct {
	*baseCommand
	command string
}

func (c *ListCommand) Run(args []string) int {
	c.cmdKey = "list"

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

func (c ListCommand) run() int {
	c.cmdKey = "list"
	if err := listPacks(c.ui); err != nil {
		c.ui.ErrorWithContext(err, "error listing packs")
		return 1
	}

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
	return "Used to list packs"
}

func (c *ListCommand) Help() string {
	c.Example = `
	# List all packs in specified registry, or default if none specified
	nomad-pack list
	`
	return formatHelp(`
	Usage: nomad-pack list

	List nomad packs.
	
` + c.GetExample() + c.Flags().Help())
}
