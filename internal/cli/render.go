// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package cli

import (
	"context"
	"fmt"
	"io/fs"
	"os"
	"path"
	"slices"
	"strings"

	"github.com/posener/complete"
	"golang.org/x/exp/maps"

	"github.com/hashicorp/nomad-pack/internal/pkg/cache"
	"github.com/hashicorp/nomad-pack/internal/pkg/errors"
	"github.com/hashicorp/nomad-pack/internal/pkg/flag"
	"github.com/hashicorp/nomad-pack/internal/pkg/helper/filesystem"
	"github.com/hashicorp/nomad-pack/terminal"
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

	// noRenderAuxFiles is a boolean flag to control whether we should also render
	// auxiliary files inside templates/
	noRenderAuxFiles bool

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
			return errors.New("destination file exists and overwrite is unset")
		}
	}

	err = os.WriteFile(path, []byte(content), 0644)
	if err != nil {
		return fmt.Errorf("failed to write rendered template to file: %s", err)
	}

	return nil
}

// rangeRenders populates a slice of `Render` (rendered templates) such that the
// target slice is sorted by Pack ID, Filename.
func rangeRenders(subj map[string]string, target *[]Render) {

	// The problem: the keys don't trivially sort in pack order anymore because
	// they have both "/template/" and a filename in them.

	// Declare some types to make the map key types a bit more obvious.
	type PackKey string  // pack key
	type Filename string // filename
	type Content string  // render

	// The rendered templates are in a map[string]string, with the key being the
	// pack-relative path to the template and the value being the rendered
	// template's file content. Dependency packs will have more path components
	// before the `/templates/` component.

	// Build a map that contains the Template slices produced by the renderer. The
	// key of the map is a pack-relative path to the template, with dependency
	// packs being child elements of the pack that depends on them.
	packKeySet := make(map[PackKey]map[Filename]Content)
	for k, v := range subj {

		// Using strings.Cut with `/templates/` provides the pack key in the
		// `before` and the template filename in the `after`. This also trims
		// `/templates/` out of the produced key as a side-effect since it's
		// low value.
		key, val, _ := strings.Cut(k, "/templates/")

		var packKey = PackKey(key)

		// Remove the .tpl from the rendered template filenames
		var filename = Filename(strings.TrimSuffix(val, ".tpl"))

		// If this is the first time we have encountered this pack's key,
		// we need to build the map to hold the Filename and content.
		if _, found := packKeySet[packKey]; !found {
			packKeySet[packKey] = make(map[Filename]Content)
		}

		// Add the template content to the map
		packKeySet[packKey][filename] = Content(v)
	}

	// At this point, we have a map[PackKey]map[Filename]Content. Sorting the
	// outer map's keys, accessing that element, and then sorting the inner
	// map's keys (Filename), enables us to rewrite the target []Render in
	// Pack, Filename order should be able to to some sorting and traversing into
	// an ordered slice.

	// Grab a list of the pack keys and sort them. Note, they are full pack-
	// relative paths, so they nicely sort in depth-sensitive way
	packKeys := maps.Keys(packKeySet)
	slices.Sort(packKeys)

	// Range the sorted list of pack keys...
	for _, packKey := range packKeys {

		// Grab the map[Filename]Content
		mFileContent := packKeySet[packKey]

		// Extract the keys as a slice
		filenames := maps.Keys(mFileContent)

		// Sort the filenames alphabetically
		slices.Sort(filenames)

		// Range the sorted list of filenames...
		for _, filename := range filenames {
			// Grab the Content
			content := mFileContent[filename]

			// Create a `Render` from the currently referenced content; write it into
			// the target slice.
			*target = append(*target,
				Render{
					Name:    fmt.Sprintf("%v/%v", packKey, filename),
					Content: string(content),
				},
			)
		}
	}
}

// Run satisfies the Run function of the cli.Command interface.
func (c *RenderCommand) Run(args []string) int {
	c.cmdKey = "render" // Add cmdKey here to print out helpUsageMessage on Init error

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

	if err := cache.VerifyPackExists(c.packConfig, errorContext, c.ui); err != nil {
		return 1
	}

	client, err := c.getAPIClient()
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

	renderOutput, err := renderPack(
		packManager,
		c.baseCommand.ui,
		!c.noRenderAuxFiles,
		!c.noFormat,
		c.baseCommand.ignoreMissingVars,
		errorContext,
	)
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
	rangeRenders(renderOutput.DependentRenders(), &renders)
	rangeRenders(renderOutput.ParentRenders(), &renders)

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
			Name:    "skip-aux-files",
			Target:  &c.noRenderAuxFiles,
			Default: false,
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
