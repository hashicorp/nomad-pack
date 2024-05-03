// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package job

import (
	"fmt"

	"github.com/hashicorp/nomad/api"

	"github.com/hashicorp/nomad-pack/internal/pkg/errors"
	"github.com/hashicorp/nomad-pack/internal/runner"
	"github.com/hashicorp/nomad-pack/terminal"
)

const (
	jobModifyIndexHelp = `To submit the job with version verification run:

nomad-pack run %s --check-index=%d [options]

When running the job with the check-index flag, the job will only be run if the
job modify index given matches the server-side version. If the index has
changed, another user has modified the job and the plan's results are
potentially invalid.`
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
		planOpts := &api.PlanOptions{
			Diff:           r.cfg.PlanConfig.Diff,
			PolicyOverride: r.cfg.PlanConfig.PolicyOverride,
		}

		if parsedJob.Job().IsMultiregion() {
			return r.multiRegionPlan(planOpts, parsedJob.Job(), ui, tplErrorContext)
		}

		// Submit the job
		planResponse, _, err := r.client.Jobs().PlanOpts(parsedJob.Job(), planOpts, r.newWriteOptsFromJob(parsedJob))
		if err != nil {
			outputErrors = append(outputErrors, &errors.WrappedUIContext{
				Err:     err,
				Subject: "failed to perform plan",
				Context: tplErrorContext,
			})
			exitCode = runner.HigherPlanCode(exitCode, runner.PlanCodeError)
			continue
		}

		exitCode = runner.HigherPlanCode(exitCode, r.outputPlannedJob(ui, parsedJob.Job(), planResponse))
		r.formatJobModifyIndex(planResponse.JobModifyIndex, ui)
	}

	if outputErrors != nil || len(outputErrors) > 0 {
		return exitCode, outputErrors
	}
	return exitCode, nil
}

func (r *Runner) multiRegionPlan(
	opts *api.PlanOptions,
	job *api.Job,
	ui terminal.UI,
	errCtx *errors.UIErrorContext) (int, []*errors.WrappedUIContext) {

	// Setup our return objects along with a map to store all the plans.
	var (
		exitCode     int
		outputErrors []*errors.WrappedUIContext
	)

	plans := map[string]*api.JobPlanResponse{}

	// collect all the plans first so that we can report all errors
	for _, region := range job.Multiregion.Regions {

		job.Region = &region.Name

		regionCtx := errCtx.Copy()
		regionCtx.Add(errors.UIContextPrefixRegion, region.Name)

		// Submit the job for this region
		result, _, err := r.client.Jobs().PlanOpts(job, opts, r.newWriteOptsFromClientJob(job))
		if err != nil {
			outputErrors = append(outputErrors, &errors.WrappedUIContext{
				Err:     err,
				Subject: "failed to perform regional plan",
				Context: regionCtx,
			})
			exitCode = runner.HigherPlanCode(exitCode, runner.PlanCodeError)
			continue
		}
		plans[region.Name] = result
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

func (r *Runner) outputPlannedJob(ui terminal.UI, job *api.Job, resp *api.JobPlanResponse) int {

	// Print the diff if not disabled
	if r.cfg.PlanConfig.Diff {
		formatJobDiff(*resp.Diff, r.cfg.PlanConfig.Verbose, ui)
	}

	// Print the scheduler dry-run output
	ui.Header("Scheduler dry-run:")
	formatDryRun(resp, job, ui)

	// Print any warnings if there are any
	if resp.Warnings != "" {
		ui.Warning(fmt.Sprintf("\nJob Warnings:\n%s", resp.Warnings))
	}

	// Print preemptions if there are any
	if resp.Annotations != nil && resp.Annotations.PreemptedAllocs != nil {
		formatPreemptions(ui, resp)
	}

	return getExitCode(resp)
}

// formatJobModifyIndex produces a help string that displays the job modify
// index and how to submit a job with it.
func (r *Runner) formatJobModifyIndex(jobModifyIndex uint64, ui terminal.UI) {
	ui.AppendToRow(jobModifyIndexHelp, r.runnerCfg.PackName, jobModifyIndex, terminal.WithStyle(terminal.BoldStyle))
}

func getExitCode(resp *api.JobPlanResponse) int {
	for _, d := range resp.Annotations.DesiredTGUpdates {
		if d.Stop+d.Place+d.Migrate+d.DestructiveUpdate+d.Canary > 0 {
			return runner.PlanCodeUpdates
		}
	}

	return runner.PlanCodeNoUpdates
}
