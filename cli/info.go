package cli

import (
	"fmt"
	"path"

	"github.com/hashicorp/nomad-pack/internal/pkg/cache"
	"github.com/hashicorp/nomad-pack/internal/pkg/flag"
	"github.com/hashicorp/nomad-pack/internal/pkg/loader"
	"github.com/hashicorp/nomad-pack/internal/pkg/variable"
	"github.com/mitchellh/go-glint"
)

type InfoCommand struct {
	*baseCommand
	packConfig *cache.PackConfig
}

func (c *InfoCommand) Run(args []string) int {
	c.cmdKey = "info" // Add cmdKey here to print out helpUsageMessage on Init error
	// Initialize. If we fail, we just exit since Init handles the UI.
	if err := c.Init(
		WithExactArgs(1, args),
		WithFlags(c.Flags()),
		WithNoConfig(),
	); err != nil {

		c.ui.ErrorWithContext(err, ErrParsingArgsOrFlags)
		c.ui.Info(c.helpUsageMessage())

		return 1
	}

	c.packConfig.Name = c.args[0]

	// Set the packConfig defaults if necessary and generate our UI error context.
	errorContext := initPackCommand(c.packConfig)

	// verify packs exist before running jobs
	if err := cache.VerifyPackExists(c.packConfig, errorContext, c.ui); err != nil {
		return 1
	}

	packPath := c.packConfig.Path

	pack, err := loader.Load(packPath)
	if err != nil {
		c.ui.ErrorWithContext(err, "failed to load pack from local directory", errorContext.GetAll()...)
		return 1
	}

	variableParser, err := variable.NewParser(&variable.ParserConfig{
		ParentName:        path.Base(packPath),
		RootVariableFiles: pack.RootVariableFiles(),
	})
	if err != nil {
		return 1
	}

	parsedVars, diags := variableParser.Parse()
	if diags != nil && diags.HasErrors() {
		c.ui.Info(diags.Error())
		return 1
	}

	// Create a new glint document to handle the outputting of information.
	doc := glint.New()

	doc.Append(glint.Layout(
		glint.Style(glint.Text("Pack Name          "), glint.Bold()),
		glint.Text(pack.Metadata.Pack.Name),
	).Row())

	doc.Append(glint.Layout(
		glint.Style(glint.Text("Description        "), glint.Bold()),
		glint.Text(pack.Metadata.Pack.Description),
	).Row())

	doc.Append(glint.Layout(
		glint.Style(glint.Text("Application URL    "), glint.Bold()),
		glint.Text(pack.Metadata.App.URL),
	).Row())

	doc.Append(glint.Layout(
		glint.Style(glint.Text("Application Author "), glint.Bold()),
		glint.Text(pack.Metadata.App.Author),
		glint.Text("\n"),
	).Row())

	for pName, variables := range parsedVars.Vars {

		doc.Append(glint.Layout(
			glint.Style(glint.Text(fmt.Sprintf("Pack %q Variables:", pName)), glint.Bold()),
		).Row())

		for _, v := range variables {
			doc.Append(glint.Layout(
				glint.Style(glint.Text(fmt.Sprintf("\t- %q (%s) - %s",
					v.Name, v.Type.FriendlyName(), v.Description))),
			).Row())
		}
		glint.Text("\n")
	}

	doc.RenderFrame()
	return 0
}

func (c *InfoCommand) Flags() *flag.Sets {
	return c.flagSet(flagSetOperation, func(set *flag.Sets) {
		c.packConfig = &cache.PackConfig{}

		f := set.NewSet("Render Options")

		f.StringVar(&flag.StringVar{
			Name:    "registry",
			Target:  &c.packConfig.Registry,
			Default: "",
			Usage: `Specific registry name containing the pack to retrieve info about.
If not specified, the default registry will be used.`,
		})

		f.StringVar(&flag.StringVar{
			Name:    "ref",
			Target:  &c.packConfig.Ref,
			Default: "",
			Usage: `Specific git ref of the pack to retrieve info about. 
Supports tags, SHA, and latest. If no ref is specified, defaults to 
latest.

Using ref with a file path is not supported.`,
		})
	})
}

func (c *InfoCommand) Help() string {
	c.Example = `
	# Get information on the "hello-world" pack
	nomad-pack info hello-world
	`

	return formatHelp(`
	Usage: nomad-pack info <pack-name>

	Returns information on the given pack including name, description, and variable details.

` + c.GetExample())
}

func (c *InfoCommand) Synopsis() string {
	return "Get information on a pack"
}
