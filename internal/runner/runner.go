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
	PackRef        string
	RegistryName   string
}

// PlanCode* is the set of expected error codes that Runner.PlanDeployment
// should return. Please see the interface function docstring for more details.
const (
	PlanCodeNoUpdates = 0
	PlanCodeUpdates   = 1
	PlanCodeError     = 255
)

// HigherPlanCode is a helper function that returns the highest plan exit code
// so implementations can easily track the code to return.
func HigherPlanCode(old, new int) int {
	if new > old {
		return new
	}
	return old
}

// Runner is the interface that defines the deployment mechanism for creating
// objects in a Nomad cluster from pack templates. This currently only covers
// validation of templates against their native Nomad object, but will be
// expanded to cover planning and running.
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
	// implementation to print console information via the terminal.UI. The
	// returned int identifies the exit code for the CLI. In order to keep
	// consistency with the Nomad CLI and across pack objects, the following
	// rules should be used:
	//
	// code 0:   No objects will be created or destroyed.
	// code 1:   Objects will be created or destroyed.
	// code 255: An error occurred determining the plan.
	PlanDeployment(terminal.UI, *errors.UIErrorContext) (int, []*errors.WrappedUIContext)

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
