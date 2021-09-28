package cli

import (
	"github.com/hashicorp/nom/flag"
	"github.com/hashicorp/nom/internal/pkg/errors"
)

type InitCommand struct {
	*baseCommand

	// TODO: better names for these options
	fromProject string
	into        string
	update      bool
	from        string
}

func (c *InitCommand) Run(args []string) int {
	c.cmdKey = "init" // Add cmd key here so help text is available in Init
	// Initialize. If we fail, we just exit since Init handles the UI.
	if err := c.Init(
		WithExactArgs(0, args),
		WithFlags(c.Flags()),
		WithNoConfig(),
	); err != nil {
		return 1
	}

	// Generate our UI error context.
	errorContext := errors.NewUIErrorContext()

	err := createGlobalCache(c.ui, errorContext)
	if err != nil {
		c.ui.ErrorWithContext(err, "error creating global cache", errorContext.GetAll()...)
		return 1
	}

	err = installDefaultRegistry(c.ui, errorContext)
	if err != nil {
		c.ui.ErrorWithContext(err, "failed to install registry", errorContext.GetAll()...)
		return 1
	}

	// Optionally create other registries, if registry flag passed
	if c.from != "" && c.into != "" {
		err = installUserRegistry(c.from, c.into, c.ui, errorContext)
		if err != nil {
			c.ui.ErrorWithContext(err, "failed to install registry", errorContext.GetAll()...)
			return 1
		}
	}

	return 0
}


func (c *InitCommand) Synopsis() string {
	return ""
}

// Flags defines the flag.Sets for the operation.
func (c *InitCommand) Flags() *flag.Sets {
	return c.flagSet(flagSetOperation, func(set *flag.Sets) {
		f := set.NewSet("Init Options")

		// TODO: validation that it has both from & into flags
		f.StringVar(&flag.StringVar{
			Name:    "from",
			Target:  &c.from,
			Default: "",
			Usage: `Allows you to install packs from other registries besides the default.
                      `,
		})
		f.StringVar(&flag.StringVar{
			Name:    "into",
			Target:  &c.into,
			Default: "",
			Usage: `Allows you to install packs from other registries besides the default.
                      `,
		})

	})
}

func (c *InitCommand) Help() string {
	return formatHelp(`
	Usage: nom run <pack-name> [options]

	Install the specified NOM pack to a configured Nomad cluster.
	
	` + c.Flags().Help())
}
