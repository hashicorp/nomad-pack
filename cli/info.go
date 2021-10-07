package cli

import (
	"fmt"
	"path"

	flag "github.com/hashicorp/nomad-pack/flag"
	"github.com/hashicorp/nomad-pack/internal/pkg/errors"
	"github.com/hashicorp/nomad-pack/internal/pkg/loader"
	"github.com/hashicorp/nomad-pack/internal/pkg/variable"
	"github.com/mitchellh/go-glint"
)

type InfoCommand struct {
	*baseCommand
	packName string
	repoName string
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

	packRepoName := c.args[0]

	// Generate our UI error context.
	errorContext := errors.NewUIErrorContext()
	repoName, packName, err := parseRegistryAndPackName(packRepoName)

	if err != nil {
		c.ui.ErrorWithContext(err, "failed to parse pack name", errorContext.GetAll()...)
		return 1
	}
	c.packName = packName
	c.repoName = repoName
	errorContext.Add(errors.UIContextPrefixPackName, c.packName)
	errorContext.Add(errors.UIContextPrefixRegistryName, c.repoName)

	registryPath, err := getRegistryPath(repoName, c.ui, errorContext)
	if err != nil {
		return 1
	}

	// Add the path to the pack on the error context.
	errorContext.Add(errors.UIContextPrefixPackPath, registryPath)

	// verify packs exist before running jobs
	if err = verifyPackExist(c.ui, c.packName, registryPath, errorContext); err != nil {
		return 1
	}

	packPath, err := getPackPath(repoName, packName)
	if err != nil {
		c.ui.ErrorWithContext(err, "failed to get path directory", errorContext.GetAll()...)
		return 1
	}

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
	return c.flagSet(0, nil)
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
