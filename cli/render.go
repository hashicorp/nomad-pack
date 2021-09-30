package cli

import (
	"github.com/hashicorp/nom/flag"
	"github.com/hashicorp/nom/internal/pkg/errors"
	v1 "github.com/hashicorp/nomad-openapi/v1"
	"github.com/mitchellh/go-glint"
	"github.com/posener/complete"
)

// RenderCommand is a command that allows users to render the templates within
// a pack and display them on the console. This is useful when developing or
// debugging packs.
type RenderCommand struct {
	*baseCommand

	// renderOutputTemplate is a boolean flag to control whether the outpu
	// template is rendered.
	renderOutputTemplate bool
}

// Run satisfies the Run function of the cli.Command interface.
func (r *RenderCommand) Run(args []string) int {

	r.cmdKey = "render"

	if err := r.Init(WithExactArgs(1, args), WithFlags(r.Flags()), WithNoConfig()); err != nil {
		return 1
	}

	// Generate our UI error context.
	errorContext := errors.NewUIErrorContext()

	packRepoName := args[0]

	repo, pack, err := parseRepoFromPackName(packRepoName)
	if err != nil {
		r.ui.ErrorWithContext(err, "failed to parse pack name", errorContext.GetAll()...)
		return 1
	}
	errorContext.Add(errors.UIContextPrefixPackName, pack)
	errorContext.Add(errors.UIContextPrefixRepoName, repo)

	// TODO: Refactor to context.nomad file in next phase.
	tempRepoPath, err := getRepoPath(repo, r.ui, errorContext)
	if err != nil {
		r.ui.ErrorWithContext(err, "failed to identify repository path", errorContext.GetAll()...)
		return 1
	}

	// Add the path to the error context, so this can be viewed when displaying
	// an error.
	errorContext.Add(errors.UIContextPrefixPackPath, tempRepoPath)

	if err = verifyPackExist(r.ui, pack, tempRepoPath, errorContext); err != nil {
		return 1
	}

	client, err := v1.NewClient()
	if err != nil {
		r.ui.ErrorWithContext(err, "failed to initialize client", errorContext.GetAll()...)
		return 1
	}

	packManager := generatePackManager(r.baseCommand, client, tempRepoPath, pack)

	renderOutput, err := renderPack(packManager, r.baseCommand.ui, errorContext)
	if err != nil {
		return 1
	}

	// The render command should at least render one parent, or one dependant
	// pack template.
	if renderOutput.LenParentRenders() < 1 && renderOutput.LenDependentRenders() < 1 {
		r.ui.ErrorWithContext(errors.ErrNoTemplatesRendered, "no templates rendered", errorContext.GetAll()...)
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
	if r.renderOutputTemplate {
		outputRender, err := packManager.ProcessOutputTemplate()
		if err != nil {
			r.ui.ErrorWithContext(err, "failed to render output template", errorContext.GetAll()...)
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

func (r *RenderCommand) Flags() *flag.Sets {
	return r.flagSet(flagSetOperation, func(set *flag.Sets) {

		f := set.NewSet("Render Options")

		f.BoolVar(&flag.BoolVar{
			Name:    "render-output-template",
			Target:  &r.renderOutputTemplate,
			Default: false,
			Usage: `Controls whether or not the output template file within the
                      pack is rendered and displayed.`,
		})
	})
}

func (r *RenderCommand) AutocompleteArgs() complete.Predictor {
	return complete.PredictNothing
}

func (r *RenderCommand) AutocompleteFlags() complete.Flags {
	return r.Flags().Completions()
}

// Help satisfies the Help function of the cli.Command interface.
func (r *RenderCommand) Help() string {

	r.Example = `
	# Render an example pack with override variables in a variable file.
	nomad-pack run example --var-file="./overrides.hcl"

	# Render an example pack with cli variable overrides.
	nomad-pack run example --var="redis_image_version=latest" \
		--var="redis_resources={"cpu": "1000", "memory": "512"}"

	# Render an example pack including the outputs template file.
	nomad-pack run example --render-output-template	
	`

	return formatHelp(`
	Usage: nomad-pack render <pack-name> [options]

	Render the specified Nomad Pack and view the results.

` + r.GetExample() + r.Flags().Help())
}

// Synopsis satisfies the Synopsis function of the cli.Command interface.
func (r *RenderCommand) Synopsis() string {
	return "Render the templates within a pack"
}
