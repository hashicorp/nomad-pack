package cli

import (
	v1 "github.com/hashicorp/nomad-openapi/v1"
	"github.com/hashicorp/nomad-pack/flag"
	"github.com/hashicorp/nomad-pack/internal/pkg/cache"
	"github.com/hashicorp/nomad-pack/internal/pkg/errors"
	"github.com/mitchellh/go-glint"
	"github.com/posener/complete"
)

// RenderCommand is a command that allows users to render the templates within
// a pack and display them on the console. This is useful when developing or
// debugging packs.
type RenderCommand struct {
	*baseCommand
	packConfig *cache.PackConfig
	// renderOutputTemplate is a boolean flag to control whether the outpu
	// template is rendered.
	renderOutputTemplate bool
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

	packManager := generatePackManager(c.baseCommand, client, c.packConfig)

	renderOutput, err := renderPack(packManager, c.baseCommand.ui, errorContext)
	if err != nil {
		return 1
	}

	// The render command should at least render one parent, or one dependant
	// pack template.
	if renderOutput.LenParentRenders() < 1 && renderOutput.LenDependentRenders() < 1 {
		c.ui.ErrorWithContext(errors.ErrNoTemplatesRendered, "no templates rendered", errorContext.GetAll()...)
		return 1
	}

	d := glint.New()

	// Iterate the rendered files and add these to the Glint document.
	// TODO(jrasell): trim at least the templates directory name from the name
	//  as it doesn't provide much benefit.
	for name, renderedFile := range renderOutput.DependentRenders() {
		addRenderToDoc(d, name, renderedFile)
	}
	for name, renderedFile := range renderOutput.ParentRenders() {
		addRenderToDoc(d, name, renderedFile)
	}

	// If the user wants to render and display the outputs template file then
	// render this. In the event the render returns an error, print this but do
	// not exit. The render can fail due to template function errors, but we
	// can still display the pack templates from above. The error will be
	// displayed before the template renders, so the UI looks OK.
	if c.renderOutputTemplate {
		outputRender, err := packManager.ProcessOutputTemplate()
		if err != nil {
			c.ui.ErrorWithContext(err, "failed to render output template", errorContext.GetAll()...)
		} else {
			addRenderToDoc(d, "outputs.tpl", outputRender)
		}
	}

	d.RenderFrame()
	return 0
}

// addRenderToDoc updates the Glint Document to include the provided template
// within the layout.
func addRenderToDoc(doc *glint.Document, name, tpl string) {
	doc.Append(glint.Layout(glint.Style(glint.Text(name+":"), glint.Bold())).Row())
	doc.Append(glint.Layout(glint.Style(glint.Text(""))).Row())
	doc.Append(glint.Layout(glint.Style(glint.Text(tpl))).Row())
}

func (c *RenderCommand) Flags() *flag.Sets {
	return c.flagSet(flagSetOperation, func(set *flag.Sets) {
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
Supports tags, SHA, and latest. If no ref is specified, defaults to latest.

Using ref with a file path is not supported.`,
		})

		f.BoolVar(&flag.BoolVar{
			Name:    "render-output-template",
			Target:  &c.renderOutputTemplate,
			Default: false,
			Usage: `Controls whether or not the output template file within the
                      pack is rendered and displayed.`,
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

    # Render a pack under development from the filesystem - supports current working 
    # directory or relative path
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
