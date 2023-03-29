package cli

import (
	"context"
	"fmt"
	"io/fs"
	"os"
	"path"
	"strings"

	v1 "github.com/hashicorp/nomad-openapi/v1"
	"github.com/hashicorp/nomad-pack/internal/pkg/cache"
	"github.com/hashicorp/nomad-pack/internal/pkg/errors"
	"github.com/hashicorp/nomad-pack/internal/pkg/flag"
	"github.com/hashicorp/nomad-pack/internal/pkg/helper/filesystem"
	"github.com/hashicorp/nomad-pack/terminal"
	"github.com/posener/complete"
)

// RenderCommand is a command that allows users to render the templates within
// a pack and display them on the console. This is useful when developing or
// debugging packs.
type RenderCommand struct {
	*baseCommand
	packConfig *cache.PackConfig

	// renderOutputTemplate is a boolean flag to control whether the output
	// template is rendered.
	renderOutputTemplate bool

	// renderToDir is the path to write rendered job files to in addition to
	// standard output.
	renderToDir string

	// renderAuxFiles is a boolean flag to control whether we should also render
	// auxiliary files inside templates/
	renderAuxFiles bool

	// noFormat is a boolean flag to control whether we should hcl-format the
	// templates before rendering them.
	noFormat bool

	// overwriteAll is set to true when someone specifies "a" to the y/n/a
	overwriteAll bool
}

type Render struct {
	Name    string
	Content string
}

func (r Render) toTerminal(c *RenderCommand) {
	c.ui.Output(r.Name+":", terminal.WithStyle(terminal.BoldStyle))
	c.ui.Output("")
	c.ui.Output(r.Content)
}

func (r Render) toFile(c *RenderCommand, ec *errors.UIErrorContext) error {
	renderToDir := path.Clean(c.renderToDir)
	err := validateOutDir(renderToDir)
	if err != nil {
		ec.Add("Destination Dir: ", renderToDir)
		return err
	}

	filePath, fileName := path.Split(r.Name)
	outDir := path.Join(renderToDir, filePath)
	outFile := path.Join(outDir, fileName)

	filesystem.MaybeCreateDestinationDir(outDir)

	err = writeFile(c, outFile, r.Content)
	if err != nil {
		ec.Add("Destination File: ", outFile)
		return err
	}

	return nil
}

func confirmOverwrite(c *RenderCommand, path string) (bool, error) {
	// For non-interactive UIs, the value must be passed by flag.
	if !c.ui.Interactive() {
		return c.autoApproved, nil
	}

	if c.autoApproved || c.overwriteAll {
		return true, nil
	}

	// For interactive UIs, we can do a y/n/a
	for {
		overwrite, err := c.ui.Input(&terminal.Input{
			Prompt: fmt.Sprintf("Output file %q exists, overwrite? [y/n/a] ", path),
			Style:  terminal.WarningBoldStyle,
		})
		if err != nil {
			return false, err
		}
		overwrite = strings.ToLower(overwrite)
		switch overwrite {
		case "a":
			c.overwriteAll = true
			return true, nil
		case "y":
			return true, nil
		case "n":
			return false, nil
		default:
			c.ui.Output("Please select a valid option.\n", terminal.WithStyle(terminal.ErrorBoldStyle))
		}
	}
}

func validateOutDir(path string) error {
	if path == "" {
		return nil
	}
	info, err := os.Stat(path)

	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil
		}

		return fmt.Errorf("unexpected error validating --to-dir path: %w", err)
	}

	if !info.IsDir() {
		return errors.New("--to-dir must be a directory")
	}

	return nil
}

func writeFile(c *RenderCommand, path string, content string) error {
	// Check to see if the file already exists and validate against the value
	// of overwrite.
	_, err := os.Stat(path)
	if err == nil {
		var overwrite bool
		overwrite, err = confirmOverwrite(c, path)
		if err != nil {
			return err
		}
		if !overwrite {
			return fmt.Errorf("destination file exists and overwrite is unset")
		}
	}

	err = os.WriteFile(path, []byte(content), 0644)
	if err != nil {
		return fmt.Errorf("failed to write rendered template to file: %s", err)
	}

	return nil
}

// formatRenderName trims the low-value elements from the rendered template
// name.
func formatRenderName(name string) string {
	outName := strings.Replace(name, "/templates/", "/", 1)
	outName = strings.TrimSuffix(outName, ".tpl")

	return outName
}

// Run satisfies the Run function of the cli.Command interface.
func (c *RenderCommand) Run(args []string) int {
	c.cmdKey = "render" // Add cmdKey here to print out helpUsageMessage on Init error

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

	client, err := v1.NewClient()
	if err != nil {
		c.ui.ErrorWithContext(err, "failed to initialize client", errorContext.GetAll()...)
		return 1
	}
	err = validateOutDir(c.renderToDir)
	if err != nil {
		c.ui.Error(err.Error())
		return 1
	}
	packManager := generatePackManager(c.baseCommand, client, c.packConfig)

	renderOutput, err := renderPack(packManager, c.baseCommand.ui, c.renderAuxFiles, !c.noFormat, errorContext)
	if err != nil {
		return 1
	}

	// The render command should at least render one parent, or one dependant
	// pack template.
	if renderOutput.LenParentRenders() < 1 && renderOutput.LenDependentRenders() < 1 {
		c.ui.ErrorWithContext(errors.ErrNoTemplatesRendered, "no templates rendered", errorContext.GetAll()...)
		return 1
	}

	var renders []Render

	// Iterate the rendered files and add these to the list of renders to
	// output. This allows errors to surface and end things without emitting
	// partial output and then erroring out.

	for name, renderedFile := range renderOutput.DependentRenders() {
		renders = append(renders, Render{Name: formatRenderName(name), Content: renderedFile})
	}
	for name, renderedFile := range renderOutput.ParentRenders() {
		renders = append(renders, Render{Name: formatRenderName(name), Content: renderedFile})
	}

	// If the user wants to render and display the outputs template file then
	// render this. In the event the render returns an error, print this but do
	// not exit. The render can fail due to template function errors, but we
	// can still display the pack templates from above. The error will be
	// displayed before the template renders, so the UI looks OK.
	if c.renderOutputTemplate {
		var outputRender string
		outputRender, err = packManager.ProcessOutputTemplate()
		if err != nil {
			c.ui.ErrorWithContext(err, "failed to render output template", errorContext.GetAll()...)
		} else {
			renders = append(renders, Render{Name: "outputs.tpl", Content: outputRender})
		}
	}

	// Output the renders. Output the files first if enabled so that any renders
	// that display will also have been written to disk.
	for _, render := range renders {
		if c.renderToDir != "" {
			err = render.toFile(c, errorContext)
			if err != nil {
				if errors.Is(err, context.Canceled) {
					return 1
				}
				c.ui.ErrorWithContext(err, "failed to render to file", errorContext.GetAll()...)
				return 1
			}
		}
		render.toTerminal(c)
	}

	return 0
}

func (c *RenderCommand) Flags() *flag.Sets {
	return c.flagSet(flagSetOperation|flagSetNeedsApproval, func(set *flag.Sets) {
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

		f.BoolVar(&flag.BoolVar{
			Name:    "render-output-template",
			Target:  &c.renderOutputTemplate,
			Default: false,
			Usage: `Controls whether or not the output template file within the
					pack is rendered and displayed.`,
		})

		f.BoolVar(&flag.BoolVar{
			Name:    "render-aux-files",
			Target:  &c.renderAuxFiles,
			Default: true,
			Usage: `Controls whether or not the rendered output contains auxiliary
					files found in the 'templates' folder.`,
		})

		f.BoolVar(&flag.BoolVar{
			Name:    "no-format",
			Target:  &c.noFormat,
			Default: false,
			Usage:   `Controls whether or not to format templates before outputting.`,
		})

		f.StringVarP(&flag.StringVarP{
			StringVar: &flag.StringVar{
				Name:   "to-dir",
				Target: &c.renderToDir,
				Usage: `Path to write rendered job files to in addition to
						standard output.`,
			},
			Shorthand: "o",
		})
	})
}

func (c *RenderCommand) AutocompleteArgs() complete.Predictor {
	return complete.PredictNothing
}

func (c *RenderCommand) AutocompleteFlags() complete.Flags {
	return c.Flags().Completions()
}

// Help satisfies the Help function of the cli.Command interface.
func (c *RenderCommand) Help() string {

	c.Example = `
	# Render an example pack with override variables in a variable file.
	nomad-pack render example --var-file="./overrides.hcl"

	# Render an example pack with cli variable overrides.
	nomad-pack render example --var="redis_image_version=latest" \
		--var="redis_resources={"cpu": "1000", "memory": "512"}"

	# Render an example pack including the outputs template file.
	nomad-pack render example --render-output-template

	# Render an example pack, outputting the rendered templates to file in
	# addition to the terminal. Setting auto-approve allows the command to
	# overwrite existing files.
	nomad-pack render example --to-dir ~/out --auto-approve

	# Render a pack under development from the filesystem - supports current
	# working directory or relative path
	nomad-pack render .
	`

	return formatHelp(`
	Usage: nomad-pack render <pack-name> [options]

	Render the specified Nomad Pack and view the results.

` + c.GetExample() + c.Flags().Help())
}

// Synopsis satisfies the Synopsis function of the cli.Command interface.
func (c *RenderCommand) Synopsis() string {
	return "Render the templates within a pack"
}
