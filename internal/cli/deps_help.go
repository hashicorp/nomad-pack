// Copyright IBM Corp. 2021, 2025
// SPDX-License-Identifier: MPL-2.0

package cli

import (
	"github.com/posener/complete"

	"github.com/hashicorp/nomad-pack/internal/pkg/flag"
)

// depsHelpCommand exists solely to provide top level help for the deps
// set of subcommands.
type depsHelpCommand struct {
	*baseCommand
}

func (d *depsHelpCommand) Run(args []string) int {
	d.cmdKey = "deps"

	// Initialize. If we fail, we just exit since Init handles the UI.
	if err := d.Init(
		WithNoArgs(args),
		WithNoConfig(),
		WithClient(false),
	); err != nil {
		d.ui.Info("The deps command requires the following subcommand: vendor.")
		return 1
	}

	d.ui.Info("The deps command requires the following subcommand: vendor.")
	return 0
}

func (d *depsHelpCommand) Flags() *flag.Sets {
	return d.flagSet(0, nil)
}

func (d *depsHelpCommand) AutocompleteArgs() complete.Predictor {
	return complete.PredictNothing
}

func (d *depsHelpCommand) AutocompleteFlags() complete.Flags {
	return d.Flags().Completions()
}

func (d *depsHelpCommand) Synopsis() string {
	return "Manage dependencies for pack."
}

func (d *depsHelpCommand) Help() string {
	return formatHelp(`
	Usage: nomad-pack deps <subcommand> [options]

	Manage dependencies for pack.

` + d.GetExample() + d.Flags().Help())
}
