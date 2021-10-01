package job

import (
	"fmt"

	"github.com/hashicorp/nom/internal/deploy"
	"github.com/hashicorp/nom/internal/pkg/errors"
	"github.com/hashicorp/nom/terminal"
	v1client "github.com/hashicorp/nomad-openapi/clients/go/v1"
	v1 "github.com/hashicorp/nomad-openapi/v1"
)

// PlanDeployment satisfies the PlanDeployment function of the deploy.Deployer
// interface.
func (d *Deployer) PlanDeployment(ui terminal.UI, errCtx *errors.UIErrorContext) (int, []*deploy.DeployerError) {

	var outputErrors []*deploy.DeployerError

	if len(d.parsedTemplates) < 1 {
		outputErrors = append(outputErrors, newNoParsedTemplatesError("failed to plan deployment", errCtx))
		return deploy.DeployerPlanCodeError, outputErrors
	}

	// outputCode tracks the plan the highest output code.
	var outputCode int

	for tplName, jobSpec := range d.parsedTemplates {

		tplCtx := errCtx.Copy()
		tplCtx.Add(errors.UIContextPrefixTemplateName, tplName)

		// Set up the options.
		planOpts := v1.PlanOpts{
			Diff:           d.cfg.PlanConfig.Diff,
			PolicyOverride: d.cfg.PlanConfig.PolicyOverride,
		}

		// If the job is multi-region, use the custom multi-region planner.
		if d.client.Jobs().IsMultiRegion(jobSpec) {
			return d.multiRegionPlan(ui, jobSpec, &planOpts, errCtx)
		}

		planResp, _, err := d.client.Jobs().PlanOpts(newWriteOptsFromJob(jobSpec).Ctx(), jobSpec, &planOpts)
		if err != nil {
			outputErrors = append(outputErrors, &deploy.DeployerError{
				Err:      err,
				Subject:  "failed to run plan",
				Contexts: tplCtx,
			})
			outputCode = deploy.HigherPlanCode(outputCode, deploy.DeployerPlanCodeError)
			continue
		}

		outputCode = deploy.HigherPlanCode(outputCode, d.outputPlannedJob(ui, jobSpec, planResp))
	}

	return outputCode, outputErrors
}

func (d *Deployer) multiRegionPlan(ui terminal.UI, job *v1client.Job, opts *v1.PlanOpts, errCtx *errors.UIErrorContext) (int, []*deploy.DeployerError) {

	var errs []*deploy.DeployerError
	plans := map[string]*v1client.JobPlanResponse{}

	// collect all the plans first so that we can report all errors
	for _, region := range *job.Multiregion.Regions {

		// Grab and set the region whilst also creating a new error context for
		// use with this specific region.
		regionName := region.GetName()
		job.SetRegion(regionName)

		regionalCtx := errCtx.Copy()
		regionalCtx.Add(errors.UIContextPrefixRegion, regionName)

		// Submit the job for this region
		planResp, _, err := d.client.Jobs().PlanOpts(newQueryOptsFromJob(job).Ctx(), job, opts)
		if err != nil {
			errs = append(errs, &deploy.DeployerError{
				Err:      err,
				Subject:  "failed to run regional plan",
				Contexts: regionalCtx,
			})
			continue
		}
		plans[regionName] = planResp
	}

	// Any errors at this point should be treated as terminal. Outputting a
	// subset of plans, alongside errors will be messy and a little confusing.
	if errs != nil {
		return deploy.DeployerPlanCodeError, errs
	}

	// outputCode tracks the highest return code for the multi-region plan so
	// the CLI can exit with the correct code.
	var outputCode int

	for regionName, planResp := range plans {
		ui.Info(fmt.Sprintf("Region: %q", regionName))
		outputCode = deploy.HigherPlanCode(outputCode, d.outputPlannedJob(ui, job, planResp))
	}

	return outputCode, errs
}

func (d *Deployer) outputPlannedJob(ui terminal.UI, job *v1client.Job, resp *v1client.JobPlanResponse) int {

	// Print the diff if not disabled
	if d.cfg.PlanConfig.Diff {
		formatJobDiff(*resp.Diff, d.cfg.PlanConfig.Verbose, ui)
	}

	// Print the scheduler dry-run output
	ui.Header("Scheduler dry-run:")
	formatDryRun(resp, job, ui)

	// Print any warnings if there are any
	if resp.Warnings != nil && *resp.Warnings != "" {
		ui.Warning(fmt.Sprintf("\nJob Warnings:\n%s", *resp.Warnings))
	}

	// Print preemptions if there are any
	if resp.Annotations != nil && resp.Annotations.PreemptedAllocs != nil {
		formatPreemptions(ui, resp)
	}

	return getExitCode(resp)
}

// getExitCode identifies the correct exit code to use based on the
// JobPlanResponse object.
func getExitCode(resp *v1client.JobPlanResponse) int {
	for _, d := range *resp.Annotations.DesiredTGUpdates {
		if *d.Stop+*d.Place+*d.Migrate+*d.DestructiveUpdate+*d.Canary > 0 {
			return deploy.DeployerPlanCodeUpdates
		}
	}
	return deploy.DeployerPlanCodeNoUpdates
}
