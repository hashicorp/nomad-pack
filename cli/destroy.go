package cli

import (
	"github.com/hashicorp/nomad-pack/internal/pkg/cache"
	"github.com/hashicorp/nomad-pack/internal/pkg/flag"
	"github.com/posener/complete"
)

type DestroyCommand struct {
	*StopCommand
}

func (c *DestroyCommand) Run(args []string) int {
	c.cmdKey = "destroy" // Add cmdKey here to print out helpUsageMessage on Init error
	// Initialize. If we fail, we just exit since Init handles the UI.
	if err := c.Init(
		WithExactArgs(1, args),
		WithFlags(c.Flags()),
		WithNoConfig(),
	); err != nil {

		c.ui.ErrorWithContext(err, ErrParsingArgsOrFlags)
		c.ui.Info(c.helpUsageMessage())

		return 1
	} else {
		// This needs to be in an else block so that it doesn't try to run while
		// the error above is still being handled. Without it, the error message
		// appears twice.
		s := c.StopCommand
		args = append(args, "--purge=true")
		// This will re-init and re-parse in the stop command but since we've already
		// successfully parsed flags and validated args here, we should exit the stop
		// init without error
		return s.Run(args)
	}

}

func (c *DestroyCommand) Flags() *flag.Sets {
	return c.flagSet(flagSetOperation, func(set *flag.Sets) {
		c.packConfig = &cache.PackConfig{}

		set.HideUnusedFlags("Operation Options", []string{"var", "var-file"})

		f := set.NewSet("Destroy Options")

		f.StringVar(&flag.StringVar{
			Name:    "registry",
			Target:  &c.packConfig.Registry,
			Default: "",
			Usage:   `Specific registry name containing the pack to be destroyed.`,
		})

		f.StringVar(&flag.StringVar{
			Name:    "ref",
			Target:  &c.packConfig.Ref,
			Default: "",
			Usage: `Specific git ref of the pack to be destroyed. 
Supports tags, SHA, and latest. If no ref is specified, defaults to 
latest.

Using ref with a file path is not supported.`,
		})

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
	# Stop an example pack in deployment "dev" and delete it from the cluster
	nomad-pack destroy example --name=dev

	# Stop and delete an example pack in deployment "dev" that has a job named "test"
	# If the same pack has been installed in deployment "dev" but overriding the job 
	# name to "hello", only "test" will be deleted
	nomad-pack destroy example --name=dev --var=job_name=test	
	`
	return formatHelp(`
	Usage: nomad-pack destroy <pack name> [options]

	Stop and delete the specified Nomad Pack from the configured Nomad cluster.
	This is the same as using the command "nomad-pack stop <pack name> --purge".
	By default, the destroy command will delete ALL jobs in the pack deployment. 
	If a pack was run using var overrides to specify the job name(s), the var 
	overrides MUST be provided when destroying the pack to guarantee nomad-pack 
	targets the correct job(s) in the pack deployment.
	
` + c.GetExample() + c.Flags().Help())
}

// Synopsis satisfies the Synopsis function of the cli.Command interface.
func (c *DestroyCommand) Synopsis() string {
	return "Delete an existing pack"
}
