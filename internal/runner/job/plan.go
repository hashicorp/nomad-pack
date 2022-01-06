package job

import (
	"fmt"

	v1client "github.com/hashicorp/nomad-openapi/clients/go/v1"
	v1 "github.com/hashicorp/nomad-openapi/v1"
	"github.com/hashicorp/nomad-pack/internal/pkg/errors"
	intHelper "github.com/hashicorp/nomad-pack/internal/pkg/helper"
	"github.com/hashicorp/nomad-pack/internal/runner"
	"github.com/hashicorp/nomad-pack/terminal"
)

// PlanDeployment satisfies the PlanDeployment function of the runner.Runner
// interface.
func (r *Runner) PlanDeployment(ui terminal.UI, errCtx *errors.UIErrorContext) (int, []*errors.WrappedUIContext) {

	var (
		exitCode     int
		outputErrors []*errors.WrappedUIContext
	)

	if len(r.parsedTemplates) < 1 {
		outputErrors = append(outputErrors, newNoParsedTemplatesError("failed to check for conflicts", errCtx))
		return runner.PlanCodeError, outputErrors
	}

	for tplName, parsedJob := range r.parsedTemplates {

		// tplErrorContext forms the basis for error output context as is
		// appended to when new information becomes available.
		tplErrorContext := errCtx.Copy()
		tplErrorContext.Add(errors.UIContextPrefixTemplateName, tplName)
		tplErrorContext.Add(errors.UIContextPrefixJobName, parsedJob.GetName())

		// Set up the options.
		planOpts := v1.PlanOpts{
			Diff:           r.cfg.PlanConfig.Diff,
			PolicyOverride: r.cfg.PlanConfig.PolicyOverride,
		}

		if r.client.Jobs().IsMultiRegion(parsedJob) {
			return r.multiRegionPlan(&planOpts, parsedJob, ui, tplErrorContext)
		}

		// Submit the job
		planResponse, _, err := r.client.Jobs().PlanOpts(newWriteOptsFromJob(parsedJob).Ctx(), parsedJob, &planOpts)
		if err != nil {
			outputErrors = append(outputErrors, &errors.WrappedUIContext{
				Err:     intHelper.UnwrapAPIError(err),
				Subject: "failed to perform plan",
				Context: tplErrorContext,
			})
			exitCode = runner.HigherPlanCode(exitCode, runner.PlanCodeError)
			continue
		}

		exitCode = runner.HigherPlanCode(exitCode, r.outputPlannedJob(ui, parsedJob, planResponse))
	}

	if outputErrors != nil || len(outputErrors) > 0 {
		return exitCode, outputErrors
	}
	return exitCode, nil
}

func (r *Runner) multiRegionPlan(
	opts *v1.PlanOpts,
	job *v1client.Job,
	ui terminal.UI,
	errCtx *errors.UIErrorContext) (int, []*errors.WrappedUIContext) {

	// Setup our return objects along with a map to store all the plans.
	var (
		exitCode     int
		outputErrors []*errors.WrappedUIContext
	)

	plans := map[string]*v1client.JobPlanResponse{}

	// collect all the plans first so that we can report all errors
	for _, region := range *job.Multiregion.Regions {

		job.SetRegion(*region.Name)

		regionCtx := errCtx.Copy()
		regionCtx.Add(errors.UIContextPrefixRegion, *region.Name)

		// Submit the job for this region
		result, _, err := r.client.Jobs().PlanOpts(newQueryOptsFromJob(job).Ctx(), job, opts)
		if err != nil {
			outputErrors = append(outputErrors, &errors.WrappedUIContext{
				Err:     intHelper.UnwrapAPIError(err),
				Subject: "failed to perform regional plan",
				Context: regionCtx,
			})
			exitCode = runner.HigherPlanCode(exitCode, runner.PlanCodeError)
			continue
		}
		plans[*region.Name] = result
	}

	if outputErrors != nil || len(outputErrors) > 0 {
		return exitCode, outputErrors
	}

	for regionName, resp := range plans {
		ui.Info(fmt.Sprintf("Region: %q", regionName))
		exitCode = runner.HigherPlanCode(exitCode, r.outputPlannedJob(ui, job, resp))
	}

	return exitCode, outputErrors
}

func (r *Runner) outputPlannedJob(ui terminal.UI, job *v1client.Job, resp *v1client.JobPlanResponse) int {

	// Print the diff if not disabled
	if r.cfg.PlanConfig.Diff {
		formatJobDiff(*resp.Diff, r.cfg.PlanConfig.Verbose, ui)
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

func getExitCode(resp *v1client.JobPlanResponse) int {
	for _, d := range *resp.Annotations.DesiredTGUpdates {
		if *d.Stop+*d.Place+*d.Migrate+*d.DestructiveUpdate+*d.Canary > 0 {
			return runner.PlanCodeUpdates
		}
	}

	return runner.PlanCodeNoUpdates
}
