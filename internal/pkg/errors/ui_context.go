package errors

import (
	stdErrors "errors"
	"strings"
)

// ErrNoTemplatesRendered is an error to be used when the CLI runs a render
// process that doesn't result in parent templates. This helps provide a clear
// indication to the problem, as I have certainly been confused by this.
var ErrNoTemplatesRendered = stdErrors.New("no templates were rendered by the renderer process run")

// UIContextPrefix* are the prefixes commonly used to create a string used in
// UI errors outputs. If a prefix is used more than once, it should have a
// const created.
const (
	UIContextPrefixPackName       = "Pack Name: "
	UIContextPrefixRepoName       = "Repo Name: "
	UIContextPrefixPackPath       = "Pack Path: "
	UIContextPrefixPackVersion    = "Pack Version: "
	UIContextPrefixTemplateName   = "Template Name: "
	UIContextPrefixJobName        = "Job Name: "
	UIContextPrefixDeploymentName = "Deployment Name: "
	UIContextPrefixRegion         = "Region: "
	UIContextPrefixHCLRange       = "HCL Range: "
	UIContextPrefixRegistryName   = "Registry Name: "
)

// UIErrorContext is used to store and manipulate error context strings used
// by the CLI to output user-friendly, rich information.
type UIErrorContext struct {
	contexts []string
}

// NewUIErrorContext creates an empty UIErrorContext.
func NewUIErrorContext() *UIErrorContext { return &UIErrorContext{} }

// Add formats and appends the passed prefix and value onto the error contexts.
func (u *UIErrorContext) Add(prefix, val string) {
	u.contexts = append(u.contexts, prefix+val)
}

// Append takes an existing UIErrorContext and appends any context into the
// current.
func (u *UIErrorContext) Append(context *UIErrorContext) {
	u.contexts = append(u.contexts, context.GetAll()...)
}

// Copy to currently stored contexts into a new UIErrorContext.
func (u *UIErrorContext) Copy() *UIErrorContext { return &UIErrorContext{contexts: u.contexts} }

// GetAll returns all the stored context strings.
func (u *UIErrorContext) GetAll() []string { return u.contexts }

// String returns the stored contexts as a minimally formatted string.
func (u *UIErrorContext) String() string { return strings.Join(u.contexts, "\n") }
