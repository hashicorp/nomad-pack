package job

import (
	stdErrors "errors"
	"regexp"
	"strings"

	"github.com/hashicorp/nom/internal/deploy"
	"github.com/hashicorp/nom/internal/pkg/errors"
	v1 "github.com/hashicorp/nomad-openapi/v1"
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
func newValidationDeployerError(err error, sub, tplName string) *deploy.DeployerError {
	depErr := deploy.DeployerError{
		Err:      err,
		Subject:  sub,
		Contexts: errors.NewUIErrorContext(),
	}
	depErr.Contexts.Add(errors.UIContextPrefixTemplateName, tplName)
	return &depErr
}

func newNoParsedTemplatesError(sub string, errCtx *errors.UIErrorContext) *deploy.DeployerError {
	return &deploy.DeployerError{
		Err:      stdErrors.New("no parsed templates found"),
		Subject:  sub,
		Contexts: errCtx,
	}
}

// generateRegisterError creates an appropriate DeployerError based on the
// submitted Nomad registration API error.
func generateRegisterError(err error, errCtx *errors.UIErrorContext, jobName string) *deploy.DeployerError {

	// Copy and add the job name to the error context.
	registerErr := errCtx.Copy()
	registerErr.Add(errors.UIContextPrefixJobName, jobName)

	// Create our base error.
	deployErr := deploy.DeployerError{
		Err:      err,
		Subject:  "failed to register job",
		Contexts: registerErr,
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
