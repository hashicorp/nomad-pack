package runner

import (
	"github.com/hashicorp/nomad-pack/internal/pkg/errors"
	"github.com/hashicorp/nomad-pack/terminal"
)

// Config is the generic configuration used by each runner implementation
// to identify key pack elements. This should be set using the
// Runner.SetRunnerConfig function.
type Config struct {
	DeploymentName string
	PackName       string
	PathPath       string
	PackVersion    string
	RegistryName   string
}

// Runner is the interface that defines the deployment mechanism for creating
// objects in a Nomad cluster from pack templates.
type Runner interface {

	// CanonicalizeTemplates performs Nomad Pack specific canonicalization on
	// the pack templates. This allows planning and rendering outputs to ensure
	// the rendered object matches exactly what would be deployed.
	CanonicalizeTemplates() []*errors.WrappedUIContext

	// CheckForConflicts iterates over parsed templates, and checks for
	// conflicts with running packs.
	CheckForConflicts(*errors.UIErrorContext) []*errors.WrappedUIContext

	// Deploy the rendered templates to the Nomad cluster. A single error is
	// returned as any error encountered is terminal. Any warnings and errors
	// that need to be displayed to the console should be printed within the
	// function and is why the UI and UIErrorContext is passed.
	Deploy(terminal.UI, *errors.UIErrorContext) *errors.WrappedUIContext

	// DestroyDeployment destroys the deployment as provided by the
	// configuration set within SetDeployerConfig.
	DestroyDeployment(terminal.UI) []*errors.WrappedUIContext

	// ParsedTemplates returns the parsed and canonicalized templates to the
	// caller whose responsibility it is to assert the mapping type expected
	// based on the deployer implementation.
	ParsedTemplates() interface{}

	// Name returns the name of the deployer which indicates the Nomad object
	// it is designed to handle.
	Name() string

	// PlanDeployment plans the deployment of the templates. As the information
	// of the plan is specific to the object, it is the responsibility of the
	// implementation to print console information via the terminal.UI.
	PlanDeployment(terminal.UI) []*errors.WrappedUIContext

	// SetTemplates supplies the rendered templates to the deployer for use in
	// subsequent function calls.
	SetTemplates(map[string]string)

	// SetRunnerConfig is used to set the deployer configuration on the created
	// deployer implementation.
	SetRunnerConfig(config *Config)

	// ParseTemplates iterates the templates stored by SetTemplates and
	// performs validation against their desired object. If the validation
	// includes parsing the string template into a Nomad object, the
	// implementor should store these to avoid having to do this again when
	// deploying.
	ParseTemplates() []*errors.WrappedUIContext
}
