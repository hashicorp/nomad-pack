package job

import (
	"fmt"

	"github.com/hashicorp/nom/internal/deploy"
	"github.com/hashicorp/nom/internal/pkg/errors"
	v1client "github.com/hashicorp/nomad-openapi/clients/go/v1"
)

func (d *Deployer) CheckForConflicts(errCtx *errors.UIErrorContext) []*deploy.DeployerError {

	var outputErrors []*deploy.DeployerError

	if len(d.parsedTemplates) < 1 {
		outputErrors = append(outputErrors, newNoParsedTemplatesError("failed to check for conflicts", errCtx))
		return outputErrors
	}

	for tplName, jobSpec := range d.parsedTemplates {
		if err := d.checkForConflict(jobSpec.GetName()); err != nil {
			outputErrors = append(outputErrors, newValidationDeployerError(err, validationSubjConflict, tplName))
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
func (d *Deployer) checkForConflict(jobName string) error {

	existing, _, err := d.client.Jobs().GetJob(d.clientQueryOpts.Ctx(), jobName)
	if err != nil {
		openAPIErr, ok := err.(v1client.GenericOpenAPIError)
		if !ok || string(openAPIErr.Body()) != "job not found" {
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

	// if there is a job with this ID, that has no pack-deployment-name meta,
	// it was created by something other than the package manager and this
	// process should abort.
	existingDeploymentName, ok := meta["pack-deployment-name"]
	if !ok {
		return fmt.Errorf("job with id %q already exists and is not manage by nomad pack", *existing.ID)
	}

	// If there is a job with this ID, and a different deployment name, this
	// process should abort.
	if existingDeploymentName != d.deployerCfg.DeploymentName {
		return fmt.Errorf("job with id %q' already exists and is part of deployment %q",
			*existing.ID, existingDeploymentName)
	}

	return nil
}
