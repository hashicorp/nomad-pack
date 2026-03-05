// Copyright IBM Corp. 2021, 2025
// SPDX-License-Identifier: MPL-2.0

package job

import (
	"fmt"

	"github.com/hashicorp/nomad/api"

	"github.com/hashicorp/nomad-pack/internal/pkg/errors"
	"github.com/hashicorp/nomad-pack/terminal"
)

// stopRemovedJobs queries Nomad for all jobs belonging to this pack deployment
// and stops any that are no longer present in the current set of rendered
// templates. This handles the case where a job is removed from a pack
// definition between versions: without this reconciliation, the old job would
// continue running after a pack update.
func (r *Runner) stopRemovedJobs(ui terminal.UI, errorContext *errors.UIErrorContext) *errors.WrappedUIContext {
	// Build a set of job IDs that were just deployed.
	deployedJobIDs := make(map[string]struct{}, len(r.deployedJobs))
	for _, dj := range r.deployedJobs {
		if id := dj.Job().ID; id != nil {
			deployedJobIDs[*id] = struct{}{}
		}
	}

	// Query Nomad for all jobs and filter to those belonging to this deployment.
	staleJobs, err := r.findStaleJobs(deployedJobIDs)
	if err != nil {
		errCtx := errorContext.Copy()
		return &errors.WrappedUIContext{
			Err:     err,
			Subject: "failed to query for previously deployed jobs",
			Context: errCtx,
		}
	}

	if len(staleJobs) == 0 {
		return nil
	}

	// Stop each stale job.
	for _, staleJob := range staleJobs {
		jobID := *staleJob.ID
		ui.Warning(fmt.Sprintf(
			"Job '%s' is no longer defined in pack deployment '%s' - stopping",
			jobID, r.runnerCfg.DeploymentName,
		))

		_, _, err := r.client.Jobs().DeregisterOpts(jobID, &api.DeregisterOptions{
			Purge: false,
		}, r.newWriteOptsFromClientJob(staleJob))
		if err != nil {
			errCtx := errorContext.Copy()
			errCtx.Add(errors.UIContextPrefixJobName, jobID)
			return &errors.WrappedUIContext{
				Err:     err,
				Subject: fmt.Sprintf("failed to stop removed job '%s'", jobID),
				Context: errCtx,
			}
		}

		ui.Success(fmt.Sprintf("Job '%s' stopped successfully", jobID))
	}

	return nil
}

// findStaleJobs queries Nomad for all running jobs that belong to this pack
// deployment (matched by pack.deployment_name metadata) but are NOT in the
// provided set of currently deployed job IDs.
func (r *Runner) findStaleJobs(currentJobIDs map[string]struct{}) ([]*api.Job, error) {
	jobsAPI := r.client.Jobs()

	// List all jobs across all namespaces to find ones belonging to this deployment.
	jobStubs, _, err := jobsAPI.List(&api.QueryOptions{
		Namespace: "*",
	})
	if err != nil {
		// Fallback: if wildcard namespace is not supported, try default namespace.
		jobStubs, _, err = jobsAPI.List(&api.QueryOptions{})
		if err != nil {
			return nil, fmt.Errorf("error listing jobs: %w", err)
		}
	}

	var staleJobs []*api.Job

	for _, stub := range jobStubs {
		// Skip dead jobs.
		if stub.Status == "dead" {
			continue
		}

		// Query the full job to get metadata.
		queryOpts := &api.QueryOptions{}
		if stub.JobSummary != nil && stub.JobSummary.Namespace != "" {
			queryOpts.Namespace = stub.JobSummary.Namespace
		}

		fullJob, _, err := jobsAPI.Info(stub.ID, queryOpts)
		if err != nil {
			continue // Skip jobs we can't read.
		}

		if fullJob.Meta == nil {
			continue
		}

		// Check if this job belongs to the same pack deployment.
		jobDeploymentName, ok := fullJob.Meta[PackDeploymentNameKey]
		if !ok || jobDeploymentName != r.runnerCfg.DeploymentName {
			continue
		}

		// Check if this job is NOT in the set of currently deployed jobs.
		if fullJob.ID == nil {
			continue
		}

		if _, exists := currentJobIDs[*fullJob.ID]; !exists {
			staleJobs = append(staleJobs, fullJob)
		}
	}

	return staleJobs, nil
}
