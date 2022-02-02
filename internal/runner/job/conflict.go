package job

import (
	"fmt"

	"github.com/hashicorp/nomad-pack/internal/pkg/errors"
	intHelper "github.com/hashicorp/nomad-pack/internal/pkg/helper"
)

func (r *Runner) CheckForConflicts(errCtx *errors.UIErrorContext) []*errors.WrappedUIContext {

	var outputErrors []*errors.WrappedUIContext

	if len(r.parsedTemplates) < 1 {
		outputErrors = append(outputErrors, newNoParsedTemplatesError("failed to check for conflicts", errCtx))
		return outputErrors
	}

	for tplName, jobSpec := range r.parsedTemplates {
		if err := r.checkForConflict(jobSpec.GetName()); err != nil {
			outputErrors = append(outputErrors, newValidationDeployerError(intHelper.UnwrapAPIError(err), validationSubjConflict, tplName))
			continue
		}
	}

	if len(outputErrors) > 0 {
		return outputErrors
	}
	return nil
}

// checkForConflict performs a lookup against Nomad, to check whether the
// supplied job is found. If the job is found, we confirm if it belongs to this
// Nomad Pack deployment. In the event it doesn't this will result in an error.
func (r *Runner) checkForConflict(jobName string) error {

	existing, _, err := r.client.Jobs().GetJob(r.clientQueryOpts.Ctx(), jobName)
	if err != nil {
		err = intHelper.UnwrapAPIError(err)
		if err.Error() != "job not found" {
			return err
		}
	}

	// If no existing job, no possible error condition.
	if existing == nil {
		return nil
	}

	// if there is a job with this name, that has no meta, it was
	// created by something other than the package manager and this
	// process should fail.
	if existing.Meta == nil {
		return fmt.Errorf("job with id %q already exists and is not manage by nomad pack", *existing.ID)
	}

	meta := *existing.Meta

	// if there is a job with this ID, that has no pack_deployment_name meta,
	// it was created by something other than the package manager and this
	// process should abort.
	existingDeploymentName, ok := meta[PackDeploymentNameKey]
	if !ok {
		return fmt.Errorf("job with id %q already exists and is not manage by nomad pack", *existing.ID)
	}

	// If there is a job with this ID, and a different deployment name, this
	// process should abort.
	if existingDeploymentName != r.runnerCfg.DeploymentName {
		return fmt.Errorf("job with id %q' already exists and is part of deployment %q",
			*existing.ID, existingDeploymentName)
	}

	return nil
}
