// Copyright IBM Corp. 2021, 2025
// SPDX-License-Identifier: MPL-2.0

package cli

import (
	"github.com/posener/complete"

	"github.com/hashicorp/nomad-pack/internal/config"
	"github.com/hashicorp/nomad-pack/internal/creator"
	"github.com/hashicorp/nomad-pack/internal/pkg/errors"
	"github.com/hashicorp/nomad-pack/internal/pkg/flag"
	"github.com/hashicorp/nomad-pack/sdk/pack"
)

// GeneratePackCommand adds a registry to the global caching.
type GeneratePackCommand struct {
	*baseCommand
	cfg config.PackConfig
}

func (c *GeneratePackCommand) Run(args []string) int {
	c.cmdKey = "generate pack"
	flagSet := c.Flags()

	// Initialize. If we fail, we just exit since Init handles the UI.
	if err := c.Init(
		WithExactArgs(1, args),
		WithFlags(flagSet),
		WithNoConfig(),
		WithClient(false),
	); err != nil {
		c.ui.ErrorWithContext(err, ErrParsingArgsOrFlags)
		c.ui.Info(c.helpUsageMessage())
		return 1
	}

	errorContext := errors.NewUIErrorContext()
	c.cfg.PackName = c.args[0]

	if !pack.IsValidName(c.cfg.PackName) {
		errorContext.Add(errors.UIContextErrorDetail,
			"Pack names may only contain letters, numbers, and underscores.")
		errorContext.Add(errors.UIContextErrorSuggestion,
			"To write the generated pack somewhere other than the current working directory, use the `--to-dir` flag.")
		c.ui.ErrorWithContext(errors.New("Invalid pack name"), ErrParsingArgsOrFlags, errorContext.GetAll()...)
		c.ui.Info(c.helpUsageMessage())
		return 1
	}

	// Generate the typical Pack UI error context.
	errorContext.Add(errors.UIContextPrefixPackName, c.cfg.PackName)
	errorContext.Add(errors.UIContextPrefixOutputPath, c.cfg.OutPath)

	err := creator.CreatePack(c.cfg)
	if err != nil {
		c.ui.ErrorWithContext(err, "failed to generate pack", errorContext.GetAll()...)
		return 1
	}
	return 0
}

func (c *GeneratePackCommand) Flags() *flag.Sets {
	return c.flagSet(flagSetNeedsApproval, func(set *flag.Sets) {
		c.cfg = config.PackConfig{}
		f := set.NewSet("Output Options")

		f.StringVarP(&flag.StringVarP{
			StringVar: &flag.StringVar{
				Name:    "to-dir",
				Target:  &c.cfg.OutPath,
				Usage:   `Path to write generated pack to.`,
				Default: "",
			},
			Shorthand: "o",
		})

		f.BoolVarP(&flag.BoolVarP{
			BoolVar: &flag.BoolVar{
				Name:    "overwrite",
				Target:  &c.cfg.Overwrite,
				Usage:   `If the output directory is not empty, should we overwrite?`,
				Default: false,
			},
			Shorthand: "f",
		})
	})
}

func (c *GeneratePackCommand) AutocompleteArgs() complete.Predictor {
	return complete.PredictNothing
}

func (c *GeneratePackCommand) AutocompleteFlags() complete.Flags {
	return c.Flags().Completions()
}

func (c *GeneratePackCommand) Synopsis() string {
	return "Generate a new pack."
}

func (c *GeneratePackCommand) Help() string {
	c.Example = `
	# Create a new pack named "my-new-pack" in the current directory.
	nomad-pack generate pack my-new-pack

	`
	return formatHelp(`
	Usage: nomad-pack generate pack <name>

	Generate a new pack.

` + c.GetExample() + c.Flags().Help())
}
