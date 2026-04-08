// Copyright IBM Corp. 2023, 2026
// SPDX-License-Identifier: MPL-2.0

package job

import (
	"testing"

	"github.com/hashicorp/nomad/api"
	"github.com/shoenig/test/must"
)

func TestIsChildJobStub_PackJobMetadataPrecedence(t *testing.T) {
	t.Run("pack.job equal to ID is root even with slash", func(t *testing.T) {
		stub := &api.JobListStub{
			ID:       "namespace/my-root-job",
			Status:   "running",
			ParentID: "",
			Meta: map[string]string{
				PackJobKey: "namespace/my-root-job",
			},
		}

		must.Eq(t, false, isChildJobStub(stub))
	})

	t.Run("pack.job different from ID is child", func(t *testing.T) {
		stub := &api.JobListStub{
			ID:       "namespace/my-root-job/dispatch-123",
			Status:   "running",
			ParentID: "",
			Meta: map[string]string{
				PackJobKey: "namespace/my-root-job",
			},
		}

		must.Eq(t, true, isChildJobStub(stub))
	})

	t.Run("missing parent and metadata is treated as root", func(t *testing.T) {
		stub := &api.JobListStub{
			ID:       "namespace/my-root-job",
			Status:   "running",
			ParentID: "",
			Meta:     map[string]string{},
		}

		must.Eq(t, false, isChildJobStub(stub))
	})
}

// TestFindStaleJobs_ChildJobsWithParentIDFiltered tests that child jobs with
// ParentID set are correctly filtered out during reconciliation.
func TestFindStaleJobs_ChildJobsWithParentIDFiltered(t *testing.T) {
	currentJobIDs := map[string]struct{}{
		"parent-job": {},
	}
	nomadJobStubs := []*api.JobListStub{
		{
			ID:       "parent-job",
			Status:   "running",
			ParentID: "",
			Meta: map[string]string{
				PackJobKey: "parent-job",
			},
		},
		{
			ID:       "parent-job/periodic-1774607280",
			Status:   "running",
			ParentID: "parent-job",
			Meta: map[string]string{
				PackJobKey: "parent-job",
			},
		},
		{
			ID:       "parent-job/dispatch-1774607281-093f6605",
			Status:   "running",
			ParentID: "parent-job",
			Meta: map[string]string{
				PackJobKey: "parent-job",
			},
		},
	}

	staleJobs := filterStaleJobs(currentJobIDs, nomadJobStubs)
	must.SliceEmpty(t, staleJobs, must.Sprint("child jobs should be filtered out"))
}

// TestFindStaleJobs_ChildJobsWithoutParentIDFiltered tests that child jobs
// without ParentID are filtered by metadata/ID fallback mechanism.
func TestFindStaleJobs_ChildJobsWithoutParentIDFiltered(t *testing.T) {
	currentJobIDs := map[string]struct{}{
		"nuclei": {},
	}
	nomadJobStubs := []*api.JobListStub{
		{
			ID:       "nuclei",
			Status:   "running",
			ParentID: "",
			Meta: map[string]string{
				PackJobKey: "nuclei",
			},
		},
		{
			ID:       "nuclei/dispatch-1774607281-093f6605",
			Status:   "running",
			ParentID: "", // ParentID NOT set (simulating API inconsistency)
			Meta: map[string]string{
				PackJobKey: "nuclei",
			},
		},
		{
			ID:       "job-requester/periodic-1774607280",
			Status:   "running",
			ParentID: "", // ParentID NOT set (simulating API inconsistency)
			Meta: map[string]string{
				PackJobKey: "job-requester",
			},
		},
	}

	staleJobs := filterStaleJobs(currentJobIDs, nomadJobStubs)
	must.SliceEmpty(t, staleJobs, must.Sprint("child jobs should be filtered"))
}

// TestFindStaleJobs_ActualStaleJobsIdentified tests that actual stale jobs
// (not in current deployment) are correctly identified.
func TestFindStaleJobs_ActualStaleJobsIdentified(t *testing.T) {
	currentJobIDs := map[string]struct{}{
		"new-job": {},
	}
	nomadJobStubs := []*api.JobListStub{
		{
			ID:       "new-job",
			Status:   "running",
			ParentID: "",
			Meta: map[string]string{
				PackJobKey: "new-job",
			},
		},
		{
			ID:       "old-job",
			Status:   "running",
			ParentID: "",
			Meta: map[string]string{
				PackJobKey: "old-job",
			},
		},
	}

	staleJobs := filterStaleJobs(currentJobIDs, nomadJobStubs)
	must.SliceLen(t, 1, staleJobs)
	must.Eq(t, "old-job", staleJobs[0])
}

// TestFindStaleJobs_StaleParentNotChildren tests that when a parent job is stale,
// only the parent is identified, not its children.
func TestFindStaleJobs_StaleParentNotChildren(t *testing.T) {
	currentJobIDs := map[string]struct{}{
		"new-job": {},
	}
	nomadJobStubs := []*api.JobListStub{
		{
			ID:       "new-job",
			Status:   "running",
			ParentID: "",
			Meta: map[string]string{
				PackJobKey: "new-job",
			},
		},
		{
			ID:       "old-parent",
			Status:   "running",
			ParentID: "",
			Meta: map[string]string{
				PackJobKey: "old-parent",
			},
		},
		{
			ID:       "old-parent/periodic-123",
			Status:   "running",
			ParentID: "old-parent",
			Meta: map[string]string{
				PackJobKey: "old-parent",
			},
		},
		{
			ID:       "old-parent/dispatch-456-abc",
			Status:   "running",
			ParentID: "old-parent",
			Meta: map[string]string{
				PackJobKey: "old-parent",
			},
		},
	}

	staleJobs := filterStaleJobs(currentJobIDs, nomadJobStubs)
	must.SliceLen(t, 1, staleJobs)
	must.Eq(t, "old-parent", staleJobs[0])
}

// TestFindStaleJobs_DeadJobsFiltered tests that dead jobs are filtered out.
func TestFindStaleJobs_DeadJobsFiltered(t *testing.T) {
	currentJobIDs := map[string]struct{}{
		"active-job": {},
	}
	nomadJobStubs := []*api.JobListStub{
		{
			ID:       "active-job",
			Status:   "running",
			ParentID: "",
			Meta: map[string]string{
				PackJobKey: "active-job",
			},
		},
		{
			ID:       "dead-job",
			Status:   "dead",
			ParentID: "",
			Meta: map[string]string{
				PackJobKey: "dead-job",
			},
		},
	}

	staleJobs := filterStaleJobs(currentJobIDs, nomadJobStubs)
	must.SliceEmpty(t, staleJobs, must.Sprint("dead jobs should be filtered out"))
}

// TestFindStaleJobs_EmptyIDsFiltered tests that jobs with empty IDs are filtered out.
func TestFindStaleJobs_EmptyIDsFiltered(t *testing.T) {
	currentJobIDs := map[string]struct{}{
		"valid-job": {},
	}
	nomadJobStubs := []*api.JobListStub{
		{
			ID:       "valid-job",
			Status:   "running",
			ParentID: "",
			Meta: map[string]string{
				PackJobKey: "valid-job",
			},
		},
		{
			ID:       "",
			Status:   "running",
			ParentID: "",
			Meta:     map[string]string{},
		},
	}

	staleJobs := filterStaleJobs(currentJobIDs, nomadJobStubs)
	must.SliceEmpty(t, staleJobs, must.Sprint("empty IDs should be filtered out"))
}

// TestFindStaleJobs_NilStubFiltered tests that nil stubs are safely ignored.
func TestFindStaleJobs_NilStubFiltered(t *testing.T) {
	currentJobIDs := map[string]struct{}{
		"valid-job": {},
	}
	nomadJobStubs := []*api.JobListStub{
		nil,
		{
			ID:       "valid-job",
			Status:   "running",
			ParentID: "",
			Meta: map[string]string{
				PackJobKey: "valid-job",
			},
		},
	}

	staleJobs := filterStaleJobs(currentJobIDs, nomadJobStubs)
	must.SliceEmpty(t, staleJobs, must.Sprint("nil stubs should be filtered out"))
}

// TestFindStaleJobs_ComplexScenario tests a complex real-world scenario with
// multiple job types, child jobs, stale jobs, and dead jobs.
func TestFindStaleJobs_ComplexScenario(t *testing.T) {
	currentJobIDs := map[string]struct{}{
		"web-service":    {},
		"batch-job":      {},
		"periodic-cron":  {},
		"dispatch-queue": {},
	}
	nomadJobStubs := []*api.JobListStub{
		// Current active jobs
		{ID: "web-service", Status: "running", Meta: map[string]string{PackJobKey: "web-service"}},
		{ID: "batch-job", Status: "running", Meta: map[string]string{PackJobKey: "batch-job"}},
		{ID: "periodic-cron", Status: "running", Meta: map[string]string{PackJobKey: "periodic-cron"}},
		{ID: "dispatch-queue", Status: "running", Meta: map[string]string{PackJobKey: "dispatch-queue"}},

		// Child jobs that should be filtered
		{ID: "periodic-cron/periodic-1774607280", Status: "running", ParentID: "periodic-cron", Meta: map[string]string{PackJobKey: "periodic-cron"}},
		{ID: "dispatch-queue/dispatch-1774607281-abc123", Status: "running", ParentID: "dispatch-queue", Meta: map[string]string{PackJobKey: "dispatch-queue"}},
		{ID: "orphan-child/dispatch-1774607282-def456", Status: "running", ParentID: "", Meta: map[string]string{PackJobKey: "orphan-child"}},

		// Actual stale jobs that should be identified
		{ID: "old-service", Status: "running", Meta: map[string]string{PackJobKey: "old-service"}},
		{ID: "legacy-batch", Status: "running", Meta: map[string]string{PackJobKey: "legacy-batch"}},

		// Dead job that should be filtered out
		{ID: "dead-service", Status: "dead", Meta: map[string]string{PackJobKey: "dead-service"}},

		// Invalid entries that should be filtered out
		nil,
		{ID: "", Status: "running", Meta: map[string]string{}},
	}

	staleJobs := filterStaleJobs(currentJobIDs, nomadJobStubs)
	must.SliceLen(t, 2, staleJobs)
	must.Eq(t, "old-service", staleJobs[0])
	must.Eq(t, "legacy-batch", staleJobs[1])
}

// filterStaleJobs extracts the core filtering logic from findStaleJobs for unit testing.
// This allows us to test the logic without requiring a real Nomad client.
func filterStaleJobs(currentJobIDs map[string]struct{}, jobStubs []*api.JobListStub) []string {
	var staleJobs []string
	for _, stub := range jobStubs {
		if stub == nil {
			continue
		}

		if stub.Status == "dead" {
			continue
		}

		if stub.ID == "" {
			continue
		}

		if isChildJobStub(stub) {
			continue
		}

		if _, exists := currentJobIDs[stub.ID]; !exists {
			staleJobs = append(staleJobs, stub.ID)
		}
	}
	return staleJobs
}
