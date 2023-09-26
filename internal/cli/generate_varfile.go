// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package cli

import (
	"fmt"
	"io/fs"
	"os"
	"strings"

	"github.com/posener/complete"

	"github.com/hashicorp/nomad-pack/internal/pkg/cache"
	"github.com/hashicorp/nomad-pack/internal/pkg/errors"
	"github.com/hashicorp/nomad-pack/internal/pkg/flag"
	"github.com/hashicorp/nomad-pack/terminal"
)

// generateVarFileCommand is a command that allows users to generate a skeleton
// variables value file based on the provided pack. This can be a useful start
// to deploying a customized pack or for creating documentation for your own
// pack.
type generateVarFileCommand struct {
	*baseCommand
	packConfig *cache.PackConfig

	// renderTo is the path to write rendered var override files to in
	// addition to standard output.
	renderTo string

	// overwriteAll is set to true when someone specifies "a" to the y/n/a
	overwrite bool
}

func (c *generateVarFileCommand) confirmOverwrite(path string) (bool, error) {
	// For non-interactive UIs, the value must be passed by flag.
	if !c.ui.Interactive() {
		return c.autoApproved, nil
	}

	if c.autoApproved || c.overwrite {
		return true, nil
	}

	// For interactive UIs, we can do a y/n
	for {
		overwrite, err := c.ui.Input(&terminal.Input{
			Prompt: fmt.Sprintf("Output file %q exists, overwrite? [y/n] ", path),
			Style:  terminal.WarningBoldStyle,
		})
		if err != nil {
			return false, err
		}
		overwrite = strings.ToLower(overwrite)
		switch overwrite {
		case "y":
			return true, nil
		case "n":
			return false, nil
		default:
			c.ui.Output("Please select a valid option.\n", terminal.WithStyle(terminal.ErrorBoldStyle))
		}
	}
}

func (c *generateVarFileCommand) validateOutFile(path string) error {
	if path == "" {
		return nil
	}
	info, err := os.Stat(path)

	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil
		}

		return fmt.Errorf("unexpected error validating --to-file path: %w", err)
	}

	if info.IsDir() {
		return errors.New("--to-file must be a file")
	}

	return nil
}

func (c *generateVarFileCommand) writeFile(path string, content string) error {
	// Check to see if the file already exists and validate against the value
	// of overwrite.
	_, err := os.Stat(path)
	if err == nil {
		var overwrite bool
		overwrite, err = c.confirmOverwrite(path)
		if err != nil {
			return err
		}
		if !overwrite {
			return errors.New("destination file exists and overwrite is unset")
		}
	}

	err = os.WriteFile(path, []byte(content), 0644)
	if err != nil {
		return fmt.Errorf("failed to write to file: %s", err)
	}

	return nil
}

// Run satisfies the Run function of the cli.Command interface.
func (c *generateVarFileCommand) Run(args []string) int {
	c.cmdKey = "var-file" // Add cmdKey here to print out helpUsageMessage on Init error

	if err := c.Init(
		WithExactArgs(1, args),
		WithFlags(c.Flags()),
		WithNoConfig()); err != nil {

		c.ui.ErrorWithContext(err, ErrParsingArgsOrFlags)
		c.ui.Info(c.helpUsageMessage())

		return 1
	}

	c.packConfig.Name = c.args[0]

	// Set the packConfig defaults if necessary and generate our UI error context.
	errorContext := initPackCommand(c.packConfig)

	if err := cache.VerifyPackExists(c.packConfig, errorContext, c.ui); err != nil {
		return 1
	}

	packManager := generatePackManager(c.baseCommand, nil, c.packConfig)
	renderOutput, err := renderVariableOverrideFile(packManager, c.baseCommand.ui, errorContext)
	if err != nil {
		return 1
	}

	c.ui.Output(renderOutput.AsOverrideFile())
	if c.renderTo != "" {
		if err := c.validateOutFile(c.renderTo); err != nil {
			c.ui.Error(err.Error())
			return 1
		}
		if err := c.writeFile(c.renderTo, renderOutput.AsOverrideFile()); err != nil {
			c.ui.Error(err.Error())
			return 1
		}
	}
	return 0
}

func (c *generateVarFileCommand) Flags() *flag.Sets {
	return c.flagSet(flagSetNeedsApproval, func(set *flag.Sets) {
		c.packConfig = &cache.PackConfig{}

		f := set.NewSet("Render Options")

		f.StringVar(&flag.StringVar{
			Name:    "registry",
			Target:  &c.packConfig.Registry,
			Default: "",
			Usage: `Specific registry name containing the pack to be rendered.
					If not specified, the default registry will be used.`,
		})

		f.StringVar(&flag.StringVar{
			Name:    "ref",
			Target:  &c.packConfig.Ref,
			Default: "",
			Usage: `Specific git ref of the pack to be rendered.
					Supports tags, SHA, and latest. If no ref is specified,
					defaults to latest.

					Using ref with a file path is not supported.`,
		})

		f.StringVarP(&flag.StringVarP{
			StringVar: &flag.StringVar{
				Name:   "to-file",
				Target: &c.renderTo,
				Usage: `Path to write rendered variable override file to in addition to
						standard output.`,
			},
			Shorthand: "o",
		})
	})
}

func (c *generateVarFileCommand) AutocompleteArgs() complete.Predictor {
	return complete.PredictNothing
}

func (c *generateVarFileCommand) AutocompleteFlags() complete.Flags {
	return c.Flags().Completions()
}

// Help satisfies the Help function of the cli.Command interface.
func (c *generateVarFileCommand) Help() string {

	c.Example = `
	# Render a variables override file for the given pack to standard output.
	nomad-pack generate var-file example

	# Render a variable override for the example pack, outputting as a file in
	# addition to the terminal.
	nomad-pack generate var-file example --to-file ./overrides.hcl

	# Render a variable override for the example pack, outputting as a file in
	# addition to the terminal. Setting auto-approve allows the command to
	# overwrite existing files.
	nomad-pack generate var-file example --to-file ./overrides.hcl --auto-approve

	# Render a variable override pack under development from the filesystem -
	# supports current working directory or relative path
	nomad-pack generate var-file .
	`

	return formatHelp(`
	Usage: nomad-pack generate var-file <pack-name> [options]

	Render a variables file for the specified Nomad Pack.

` + c.GetExample() + c.Flags().Help())
}

// Synopsis satisfies the Synopsis function of the cli.Command interface.
func (c *generateVarFileCommand) Synopsis() string {
	return "Render the templates within a pack"
}
