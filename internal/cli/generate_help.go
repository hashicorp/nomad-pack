// Copyright IBM Corp. 2021, 2025
// SPDX-License-Identifier: MPL-2.0

package cli

import (
	"github.com/hashicorp/nomad-pack/internal/pkg/flag"
	"github.com/posener/complete"
)

// GenerateHelpCommand exists solely to provide top level help for the registry
// set of subcommands.
type GenerateHelpCommand struct {
	*baseCommand
}

func (c *GenerateHelpCommand) Run(args []string) int {
	c.cmdKey = "generate"

	// Initialize. If we fail, we just exit since Init handles the UI.
	if err := c.Init(
		WithNoArgs(args),
		WithNoConfig(),
		WithClient(false),
	); err != nil {
		c.ui.Info("The generate command requires one of the following subcommands: pack, registry, var-file.")
		return 1
	}

	c.ui.Info("The generate command requires one of the following subcommands: pack, registry, var-file.")
	return 0
}

func (c *GenerateHelpCommand) Flags() *flag.Sets {
	return c.flagSet(0, nil)
}

func (c *GenerateHelpCommand) AutocompleteArgs() complete.Predictor {
	return complete.PredictNothing
}

func (c *GenerateHelpCommand) AutocompleteFlags() complete.Flags {
	return c.Flags().Completions()
}

func (c *GenerateHelpCommand) Synopsis() string {
	return "Generate a sample nomad-pack registry, pack, or variable overrides file for a pack."
}

func (c *GenerateHelpCommand) Help() string {
	return formatHelp(`
	Usage: nomad-pack generate <subcommand> [options]

	Generate a sample nomad-pack registry, pack, or variable overrides file for a
	pack.

` + c.GetExample() + c.Flags().Help())
}
