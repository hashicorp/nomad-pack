package cli

import (
	"github.com/hashicorp/nomad-pack/internal/pkg/flag"
	"github.com/hashicorp/nomad-pack/internal/pkg/version"
	"github.com/mitchellh/go-glint"
	"github.com/posener/complete"
)

type VersionCommand struct {
	*baseCommand
}

func (c *VersionCommand) Run(args []string) int {
	flagSet := c.Flags()
	c.cmdKey = "version"

	// Initialize. If we fail, we just exit since Init handles the UI.
	if err := c.Init(WithNoArgs(args), WithFlags(flagSet), WithNoConfig(), WithClient(false)); err != nil {
		c.ui.ErrorWithContext(err, ErrParsingArgsOrFlags)
		c.ui.Info(c.helpUsageMessage())
		return 1
	}

	// Create our new glint document.
	d := glint.New()

	// Create our layout.
	d.Append(glint.Layout(
		glint.Style(
			glint.Text("Nomad Pack"),
			glint.Bold(),
		),
		glint.Text(" "),
		glint.Text(version.HumanVersion()),
	).Row())

	// Essentially force a newline and render the output.
	d.Append(glint.Text(""))
	d.RenderFrame()

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
