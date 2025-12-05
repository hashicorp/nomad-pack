// Copyright IBM Corp. 2021, 2025
// SPDX-License-Identifier: MPL-2.0

package cli

import (
	"github.com/hashicorp/nomad-pack/internal/pkg/flag"
	"github.com/hashicorp/nomad-pack/internal/pkg/version"
	"github.com/posener/complete"
)

type VersionCommand struct {
	*baseCommand
}

func (c *VersionCommand) Run(args []string) int {
	flagSet := c.Flags()
	c.cmdKey = "version"

	// Initialize. If we fail, we just exit since Init handles the UI.
	if err := c.Init(
		WithNoArgs(args),
		WithFlags(flagSet),
		WithNoConfig(),
		WithClient(false),
	); err != nil {
		c.ui.ErrorWithContext(err, ErrParsingArgsOrFlags)
		c.ui.Info(c.helpUsageMessage())
		return 1
	}

	c.ui.Output("Nomad Pack %s\n", version.HumanVersion())

	// Exit zero since we have completed successfully.
	return 0
}

func (c *VersionCommand) Flags() *flag.Sets {
	return c.flagSet(0, nil)
}

func (c *VersionCommand) AutocompleteArgs() complete.Predictor {
	return complete.PredictNothing
}

func (c *VersionCommand) AutocompleteFlags() complete.Flags {
	return c.Flags().Completions()
}

func (c *VersionCommand) Synopsis() string {
	return "Prints the version of Nomad Pack"
}

func (c *VersionCommand) Help() string {
	return formatHelp(`
	Usage: nomad-pack version

	Prints the version information for Nomad Pack.
`)
}
