package cli

import (
	"github.com/hashicorp/nomad-pack/internal/config"
	"github.com/hashicorp/nomad-pack/internal/creator"
	"github.com/hashicorp/nomad-pack/internal/pkg/errors"
	"github.com/hashicorp/nomad-pack/internal/pkg/flag"
	"github.com/posener/complete"
)

// GenerateRegistryCommand adds a registry to the global cache.
type GenerateRegistryCommand struct {
	*baseCommand
	cfg config.PackConfig
}

func (c *GenerateRegistryCommand) Run(args []string) int {
	c.cmdKey = "generate registry"
	flagSet := c.Flags()

	// Initialize. If we fail, we just exit since Init handles the UI.
	if err := c.Init(
		WithExactArgs(1, args),
		WithFlags(flagSet),
		WithNoConfig(),
		WithClient(false),
	); err != nil {
		c.ui.ErrorWithContext(err, "error parsing args or flags")
		return 1
	}

	c.cfg.PackName = c.args[0]

	// Generate our UI error context.
	errorContext := errors.NewUIErrorContext()

	errorContext.Add(errors.UIContextPrefixRegistryName, c.cfg.PackName)
	errorContext.Add(errors.UIContextPrefixOutputPath, c.cfg.OutPath)

	err := creator.CreateRegistry(c.cfg)
	if err != nil {
		c.ui.ErrorWithContext(err, "failed to generate registry", errorContext.GetAll()...)
		return 1
	}
	return 0
}

func (c *GenerateRegistryCommand) Flags() *flag.Sets {
	return c.flagSet(flagSetNeedsApproval, func(set *flag.Sets) {
		c.cfg = config.PackConfig{}

		f := set.NewSet("Output Options")
		f.StringVarP(&flag.StringVarP{
			StringVar: &flag.StringVar{
				Name:    "to-dir",
				Target:  &c.cfg.OutPath,
				Usage:   `Path to write generated registry to.`,
				Default: ".",
			},
			Shorthand: "o",
		})
		f.BoolVarP(&flag.BoolVarP{
			BoolVar: &flag.BoolVar{
				Name:    "with-sample-pack",
				Target:  &c.cfg.CreateSamplePack,
				Usage:   `Generate a sample "hello-world" pack in the registry.`,
				Default: false,
			},
		})
	})
}

func (c *GenerateRegistryCommand) AutocompleteArgs() complete.Predictor {
	return complete.PredictNothing
}

func (c *GenerateRegistryCommand) AutocompleteFlags() complete.Flags {
	return c.Flags().Completions()
}

func (c *GenerateRegistryCommand) Synopsis() string {
	return "Generate a sample pack."
}

func (c *GenerateRegistryCommand) Help() string {
	c.Example = `
	# Create a new pack named "my-new-pack" in the current directory.
	nomad-pack generate pack my-new-pack

	`
	return formatHelp(`
	Usage: nomad-pack generate pack <name>

	Generate a sample pack.

` + c.GetExample() + c.Flags().Help())
}
