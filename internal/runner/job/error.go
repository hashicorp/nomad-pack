package job

import (
	stdErrors "errors"
	"regexp"
	"strings"

	v1 "github.com/hashicorp/nomad-openapi/v1"
	"github.com/hashicorp/nomad-pack/internal/pkg/errors"
)

const (
	validationSubjParseFailed = "failed to parse job specification"
	validationSubjConflict    = "failed job conflict validation"
)

var (
	// enforceIndexRegex is a regular expression which extracts the enforcement
	// error.
	enforceIndexRegex = regexp.MustCompile(`\((Enforcing job modify index.*)\)`)
)

// newValidationDeployerError is a small helper to create a
// deploy.DeployerError when Nomad job validation fails.
func newValidationDeployerError(err error, sub, tplName string) *errors.WrappedUIContext {
	depErr := errors.WrappedUIContext{
		Err:     err,
		Subject: sub,
		Context: errors.NewUIErrorContext(),
	}
	depErr.Context.Add(errors.UIContextPrefixTemplateName, tplName)
	return &depErr
}

func newNoParsedTemplatesError(sub string, errCtx *errors.UIErrorContext) *errors.WrappedUIContext {
	return &errors.WrappedUIContext{
		Err:     stdErrors.New("no parsed templates found"),
		Subject: sub,
		Context: errCtx,
	}
}

// generateRegisterError creates an appropriate DeployerError based on the
// submitted Nomad registration API error.
func generateRegisterError(err error, errCtx *errors.UIErrorContext, jobName string) *errors.WrappedUIContext {

	// Copy and add the job name to the error context.
	registerErr := errCtx.Copy()
	registerErr.Add(errors.UIContextPrefixJobName, jobName)

	// Create our base error.
	deployErr := errors.WrappedUIContext{
		Err:      err,
		Subject:  "failed to register job",
		Context: registerErr,
	}

	// If the error was due to a problems enforcing the index, alter the
	// subject, so it is clear.
	if strings.Contains(err.Error(), v1.RegisterEnforceIndexErrPrefix) {
		matches := enforceIndexRegex.FindStringSubmatch(err.Error())
		if len(matches) == 2 {
			deployErr.Subject = "failed to register job due to check index failure"
		}
	}

	return &deployErr
}
