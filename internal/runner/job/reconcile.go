// Copyright IBM Corp. 2023, 2026
// SPDX-License-Identifier: MPL-2.0

package job

import (
	"fmt"
	"strings"

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
	for _, stub := range staleJobs {
		ui.Warning(fmt.Sprintf(
			"Job '%s' is no longer defined in pack deployment '%s' - stopping",
			stub.ID, r.runnerCfg.DeploymentName,
		))

		writeOpts := &api.WriteOptions{}
		if stub.Namespace != "" {
			writeOpts.Namespace = stub.Namespace
		}

		_, _, err := r.client.Jobs().DeregisterOpts(stub.ID, &api.DeregisterOptions{
			Purge: false,
		}, writeOpts)
		if err != nil {
			errCtx := errorContext.Copy()
			errCtx.Add(errors.UIContextPrefixJobName, stub.ID)
			return &errors.WrappedUIContext{
				Err:     err,
				Subject: fmt.Sprintf("failed to stop removed job '%s'", stub.ID),
				Context: errCtx,
			}
		}

		ui.Success(fmt.Sprintf("Job '%s' stopped successfully", stub.ID))
	}

	return nil
}

// findStaleJobs returns job list stubs that belong to this pack deployment but
// are no longer in the provided set of currently deployed job IDs.
//
// A server-side filter expression is used so Nomad evaluates the
// pack.deployment_name metadata match before sending any data over the wire.
// This means only a single List API call is made regardless of how many jobs
// or namespaces exist in the cluster.
func (r *Runner) findStaleJobs(currentJobIDs map[string]struct{}) ([]*api.JobListStub, error) {
	// Filter expression evaluated server-side: only jobs whose
	// pack.deployment_name meta tag matches this deployment are returned.
	filter := fmt.Sprintf(`Meta[%q] == %q`, PackDeploymentNameKey, r.runnerCfg.DeploymentName)

	jobStubs, _, err := r.client.Jobs().List(&api.QueryOptions{
		Namespace: "*",
		Filter:    filter,
	})
	if err != nil {
		// Fallback: wildcard namespace may not be supported on older agents.
		jobStubs, _, err = r.client.Jobs().List(&api.QueryOptions{
			Filter: filter,
		})
		if err != nil {
			return nil, fmt.Errorf("error listing jobs for deployment %q: %w", r.runnerCfg.DeploymentName, err)
		}
	}

	var staleJobs []*api.JobListStub
	for _, stub := range jobStubs {
		if stub == nil {
			continue
		}

		// Skip already-stopped jobs — nothing to do.
		if stub.Status == "dead" {
			continue
		}

		if stub.ID == "" {
			continue
		}

		// Child jobs should not be treated as pack-managed root jobs that can be stopped.
		if isChildJobStub(stub) {
			continue
		}

		// If this job is not among the ones just deployed, it is stale.
		if _, exists := currentJobIDs[stub.ID]; !exists {
			staleJobs = append(staleJobs, stub)
		}
	}

	return staleJobs, nil
}

func isChildJobStub(stub *api.JobListStub) bool {
	if stub == nil {
		return false
	}

	// Preferred signal from Nomad.
	if stub.ParentID != "" {
		return true
	}

	// Metadata-based signal from nomad-pack tags. For child jobs, pack.job points
	// to the parent/root job name, which differs from the child ID.
	if packJob, ok := stub.Meta[PackJobKey]; ok && packJob != "" {
		return packJob != stub.ID
	}

	// Fallback only when pack.job metadata is missing.
	return isChildStyleJobID(stub.ID)
}

// isChildStyleJobID is a conservative fallback: child jobs are represented as
// "<parent>/<child-run-id>", so they always contain a non-leading slash.
func isChildStyleJobID(jobID string) bool {
	// Find the last occurrence of "/" to handle job names that might contain slashes
	idx := strings.LastIndex(jobID, "/")
	if idx <= 0 {
		// No slash found, or slash is at the beginning (invalid)
		return false
	}

	return idx < len(jobID)-1
}
