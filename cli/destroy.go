package cli

import (
	"github.com/hashicorp/nom/flag"
	"github.com/posener/complete"
)

type DestroyCommand struct {
	*StopCommand
}

func (c *DestroyCommand) Run(args []string) int {
	c.cmdKey = "destroy" // Add cmd key here so help text is available in Init
	// Initialize. If we fail, we just exit since Init handles the UI.
	if err := c.Init(
		WithExactArgs(1, args),
		WithFlags(c.Flags()),
		WithNoConfig(),
	); err != nil {
		return 1
	}

	s := c.StopCommand
	args = append(args, "--purge=true")
	// This will re-init and re-parse in the stop command but since we've already
	// successfully parsed flags and validated args here, we should exit the stop
	// init without error
	return s.Run(args)
}

func (c *DestroyCommand) Flags() *flag.Sets {
	return c.flagSet(flagSetOperation, func(set *flag.Sets) {
		set.HideUnusedFlags("Operation Options", []string{"var", "var-file"})

		f := set.NewSet("Destroy Options")
		// TODO: is there a way to reuse the flag from StopCommand so we're not just copy/pasting
		f.BoolVar(&flag.BoolVar{
			Name:    "global",
			Target:  &c.global,
			Default: false,
			Usage: `Destroy multi-region pack in all its regions. By default, pack 
					destroy will destroy only a single region at a time. Ignored for single-region packs.`,
		})
	})
}

func (c *DestroyCommand) AutocompleteArgs() complete.Predictor {
	return complete.PredictNothing
}

func (c *DestroyCommand) AutocompleteFlags() complete.Flags {
	return c.Flags().Completions()
}

func (c *DestroyCommand) Help() string {
	c.Example = `
	# Stop an example pack named "dev" and delete it from the cluster
	nomad-pack destroy example --name=dev
	`
	return formatHelp(`
	Usage: nomad-pack destroy <pack name> [options]

	Stop and delete the specified Nomad Pack from the configured Nomad cluster.
	This is the same as using the command "nomad-pack stop <pack name> --purge"
	
` + c.GetExample() + c.Flags().Help())
}

// Synopsis satisfies the Synopsis function of the cli.Command interface.
func (c *DestroyCommand) Synopsis() string {
	return "Delete an existing pack"
}
