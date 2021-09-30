package cli

import (
	"fmt"
	"path"

	"github.com/hashicorp/nom/internal/pkg/errors"
	"github.com/hashicorp/nom/internal/pkg/loader"
	"github.com/hashicorp/nom/internal/pkg/variable"
)

type InfoCommand struct {
	*baseCommand
	packName string
	repoName string
}

func (c *InfoCommand) Run(args []string) int {
	c.cmdKey = "info" // Add cmd key here so help text is available in Init
	// Initialize. If we fail, we just exit since Init handles the UI.
	if err := c.Init(
		WithExactArgs(1, args),
		WithNoConfig(),
	); err != nil {
		return 1
	}

	packRepoName := c.args[0]

	// Generate our UI error context.
	errorContext := errors.NewUIErrorContext()
	repoName, packName, err := parseRepoFromPackName(packRepoName)

	if err != nil {
		c.ui.ErrorWithContext(err, "failed to parse pack name", errorContext.GetAll()...)
		return 1
	}
	c.packName = packName
	c.repoName = repoName
	errorContext.Add(errors.UIContextPrefixPackName, c.packName)
	errorContext.Add(errors.UIContextPrefixRegistry, c.repoName)

	repoPath, err := getRepoPath(repoName, c.ui, errorContext)
	if err != nil {
		return 1
	}

	// Add the path to the pack on the error context.
	errorContext.Add(errors.UIContextPrefixPackPath, repoPath)

	// verify packs exist before running jobs
	if err = verifyPackExist(c.ui, c.packName, repoPath, errorContext); err != nil {
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

	c.ui.Info("----------------------")
	c.ui.Info(fmt.Sprintf("Pack Name: %q", pack.Metadata.Pack.Name))
	c.ui.Info(fmt.Sprintf("Description: %q", pack.Metadata.Pack.Description))
	c.ui.Info(fmt.Sprintf("Application URL: %q", pack.Metadata.App.URL))
	c.ui.Info(fmt.Sprintf("Application Author: %q", pack.Metadata.App.Author))

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

	for _, variables := range parsedVars.Vars {
		for _, v := range variables {
			c.ui.Info("-----------")
			c.ui.Info(fmt.Sprintf("Variable Name: %q", v.Name))
			c.ui.Info(fmt.Sprintf("Type: %q", v.Type.FriendlyName()))
		}
	}
	c.ui.Info("----------------------")

	return 0
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
