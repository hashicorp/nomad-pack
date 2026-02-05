// Copyright IBM Corp. 2021, 2025
// SPDX-License-Identifier: MPL-2.0

package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/hashicorp/nomad/api"
	"github.com/hashicorp/nomad/command/agent"
	"github.com/mitchellh/cli"
	"github.com/shoenig/test/must"
	"github.com/shoenig/test/wait"

	ct "github.com/hashicorp/nomad-pack/internal/cli/testhelper"
	"github.com/hashicorp/nomad-pack/internal/pkg/caching"
	flag "github.com/hashicorp/nomad-pack/internal/pkg/flag"
	"github.com/hashicorp/nomad-pack/internal/pkg/helper"
	"github.com/hashicorp/nomad-pack/internal/pkg/helper/filesystem"
	"github.com/hashicorp/nomad-pack/internal/pkg/logging"
	"github.com/hashicorp/nomad-pack/internal/pkg/testfixture"
	"github.com/hashicorp/nomad-pack/internal/pkg/version"
	"github.com/hashicorp/nomad-pack/internal/runner/job"
	"github.com/hashicorp/nomad-pack/internal/testui"
)

// TODO: Test job run with diffs
// TODO: Test job run plan with diffs
// TODO: Test multi-region plan without conflicts
// TODO: Test multi-region plan with conflicts
// TODO: Test outputPlannedJob that returns non-zero exit code

const (
	testPack             = "simple_raw_exec"
	testRef              = "48eb7d5"
	testRefFlag          = "--ref=" + testRef
	badACLToken          = "bad00000-bad0-bad0-bad0-badbadbadbad"
	exitcodeMakesChanges = 91
	exitcodeNoChanges    = 90
	exitcodeError        = 92

	testPlanCmdString = "plan --exit-code-no-changes=90 --exit-code-makes-changes=91 --exit-code-error=92"
)

func TestCLI_CreateTestRegistry(t *testing.T) {
	// This test is here to help setup the pack registry cache. It needs to be
	// the first one in the file and can not be `Parallel()`
	reg, _, regPath := createTestRegistries(t)
	defer cleanTestRegistry(t, regPath)
	t.Logf("regName: %v\n", reg.Name)
	t.Logf("regPath: %v\n", regPath)
	err := filepath.Walk(regPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		t.Logf("dir: %v: name: %s\n", info.IsDir(), path)
		return nil
	})
	if err != nil {
		t.Log(err)
	}

	result := runPackCmd(t, []string{"registry", "list"})
	out := result.cmdOut.String()
	regex := regexp.MustCompile(`(?m)^ +` + reg.Name + ` +\| (\w+) +\| (\w+) +\| ` + reg.Source + `+[^\n]+?$`)
	matches := regex.FindAllString(out, -1)
	for i, match := range matches {
		t.Logf("match %v:  %v\n", i, match)
	}
	must.RegexMatch(t, regex, out)
	must.Zero(t, result.exitCode)
}

func TestCLI_Version(t *testing.T) {
	t.Parallel()
	// This test doesn't require a Nomad cluster.
	exitCode := Main([]string{"nomad-pack", "-v"})
	must.Zero(t, exitCode)
}

func TestCLI_JobRun(t *testing.T) {
	ct.HTTPTestParallel(t, ct.WithDefaultConfig(), func(s *agent.TestAgent) {
		expectGoodPackDeploy(t, runTestPackCmd(t, s, []string{"run", getTestPackPath(t, testPack)}))
	})
}

// Confirm that another pack with the same job names but a different deployment name fails
func TestCLI_JobRunConflictingDeployment(t *testing.T) {
	ct.HTTPTestParallel(t, ct.WithDefaultConfig(), func(s *agent.TestAgent) {
		expectGoodPackDeploy(t, runTestPackCmd(t, s, []string{"run", getTestPackPath(t, testPack)}))

		result := runTestPackCmd(t, s, []string{"run", getTestPackPath(t, testPack), "--name=with-name"})
		must.Eq(t, 1, result.exitCode)
		must.Eq(t, result.cmdErr.String(), "", must.Sprintf("cmdErr should be empty, but was %q", result.cmdErr.String()))
		must.StrContains(t, result.cmdOut.String(), job.ErrExistsInDeployment{JobID: testPack, Deployment: testPack}.Error())

		// Confirm that it's still possible to update the existing pack
		expectGoodPackDeploy(t, runTestPackCmd(t, s, []string{"run", getTestPackPath(t, testPack)}))
	})
}

// Check for conflict with non-pack job i.e. no meta
func TestCLI_JobRunConflictingNonPackJob(t *testing.T) {
	ct.HTTPTestParallel(t, ct.WithDefaultConfig(), func(s *agent.TestAgent) {
		// Register non pack job
		err := ct.NomadRun(s, getTestNomadJobPath(t, testPack))
		must.NoError(t, err)

		// Now try to register the pack
		result := runTestPackCmd(t, s, []string{"run", getTestPackPath(t, testPack)})

		must.Eq(t, 1, result.exitCode)
		must.Eq(t, result.cmdErr.String(), "", must.Sprintf("cmdErr should be empty, but was %q", result.cmdErr.String()))
		must.StrContains(t, result.cmdOut.String(), job.ErrExistsNonPack{JobID: testPack}.Error())
	})
}

// Check for conflict with job that has meta
func TestCLI_JobRunConflictingJobWithMeta(t *testing.T) {
	ct.HTTPTestParallel(t, ct.WithDefaultConfig(), func(s *agent.TestAgent) {
		// Register non pack job
		err := ct.NomadRun(s, getTestNomadJobPath(t, "simple_raw_exec_with_meta"))
		must.NoError(t, err)

		// Now try to register the pack
		result := runTestPackCmd(t, s, []string{"run", getTestPackPath(t, testPack)})
		must.Eq(t, 1, result.exitCode)
		must.Eq(t, result.cmdErr.String(), "", must.Sprintf("cmdErr should be empty, but was %q", result.cmdErr.String()))
		must.StrContains(t, result.cmdOut.String(), job.ErrExistsNonPack{JobID: testPack}.Error())
	})
}

func TestCLI_JobRunFails(t *testing.T) {
	t.Parallel()
	// This test doesn't require a Nomad cluster.
	result := runPackCmd(t, []string{"run", "fake-job"})

	must.Eq(t, 1, result.exitCode)
	must.Eq(t, "", result.cmdErr.String(), must.Sprintf("cmdErr should be empty, but was %q", result.cmdErr.String()))
	must.StrContains(t, result.cmdOut.String(), "Failed To Find Pack")
}

func TestCLI_JobPlan(t *testing.T) {
	ct.HTTPTestParallel(t, ct.WithDefaultConfig(), func(s *agent.TestAgent) {
		expectGoodPackPlan(t, runTestPackCmd(t, s, []string{"plan", getTestPackPath(t, testPack)}))
	})
}

func TestCLI_JobPlan_BadJob(t *testing.T) {
	ct.HTTPTestParallel(t, ct.WithDefaultConfig(), func(s *agent.TestAgent) {
		result := runTestPackCmd(t, s, []string{"plan", "fake-job"})

		must.Eq(t, "", result.cmdErr.String(), must.Sprintf("cmdErr should be empty, but was %q", result.cmdErr.String()))
		must.StrContains(t, result.cmdOut.String(), "Failed To Find Pack")
		must.Eq(t, 255, result.exitCode) // Should return 255 indicating an error
	})
}

// Confirm that another pack with the same job names but a different deployment name fails
func TestCLI_JobPlan_ConflictingDeployment(t *testing.T) {
	ct.HTTPTestParallel(t, ct.WithDefaultConfig(), func(s *agent.TestAgent) {
		reg, _, regPath := createTestRegistries(t)
		defer cleanTestRegistry(t, regPath)

		testRegFlag := "--registry=" + reg.Name
		expectGoodPackDeploy(t, runTestPackCmd(t, s, []string{"run", testPack, testRegFlag}))

		result := runTestPackCmd(t, s, []string{"run", testPack, testRegFlag, testRefFlag})
		must.Eq(t, "", result.cmdErr.String(), must.Sprintf("cmdErr should be empty, but was %q", result.cmdErr.String()))
		must.StrContains(t, result.cmdOut.String(), job.ErrExistsInDeployment{JobID: testPack, Deployment: testPack + "@latest"}.Error())
		must.Eq(t, 1, result.exitCode)
	})
}

// Check for conflict with non-pack job i.e. no meta
func TestCLI_JobPlan_ConflictingNonPackJob(t *testing.T) {
	ct.HTTPTestParallel(t, ct.WithDefaultConfig(), func(s *agent.TestAgent) {
		// Register non pack job
		err := ct.NomadRun(s, getTestNomadJobPath(t, testPack))
		must.NoError(t, err)

		// Now try to register the pack
		result := runTestPackCmd(t, s, []string{"plan", getTestPackPath(t, testPack)})
		must.Eq(t, "", result.cmdErr.String(), must.Sprintf("cmdErr should be empty, but was %q", result.cmdErr.String()))
		must.StrContains(t, result.cmdOut.String(), job.ErrExistsNonPack{JobID: testPack}.Error())
		must.Eq(t, 255, result.exitCode) // Should return 255 indicating an error
	})
}

func TestCLI_PackPlan_OverrideExitCodes(t *testing.T) {
	ct.HTTPTest(t, ct.WithDefaultConfig(), func(s *agent.TestAgent) {
		testPlanCommand := func(t *testing.T) []string {
			out := strings.Split(testPlanCmdString, " ")
			out = append(out, getTestPackPath(t, testPack))
			return out
		}

		t.Run("plan_against_empty", func(t *testing.T) {
			// Plan against empty - should be makes-changes
			result := runTestPackCmd(t, s, testPlanCommand(t))
			must.Eq(t, "", result.cmdErr.String(), must.Sprintf("cmdErr should be empty, but was %q", result.cmdErr.String()))
			must.StrContains(t, result.cmdOut.String(), "Plan succeeded\n")
			must.Eq(t, exitcodeMakesChanges, result.exitCode) // Should return exit-code-makes-changes
		})

		t.Run("register non-pack-job", func(t *testing.T) {
			// Register non pack job
			err := ct.NomadRun(s, getTestNomadJobPath(t, testPack))
			must.NoError(t, err)
		})

		t.Run("register_pack_expect_error", func(t *testing.T) {
			// Now try to register the pack, should be error
			result := runTestPackCmd(t, s, testPlanCommand(t))
			must.Eq(t, "", result.cmdErr.String(), must.Sprintf("cmdErr should be empty, but was %q", result.cmdErr.String()))
			must.StrContains(t, result.cmdOut.String(), job.ErrExistsNonPack{JobID: testPack}.Error())
			must.Eq(t, exitcodeError, result.exitCode) // Should exit-code-error
		})

		t.Run("cleanup non-pack-job", func(t *testing.T) {
			// Purge the non-pack job created earlier.
			err := ct.NomadPurge(s, testPack)
			must.NoError(t, err)

			isGone := func() bool {
				_, err = ct.NomadJobStatus(s, testPack)
				if err != nil {
					return err.Error() == "Unexpected response code: 404 (job not found)"
				}
				return false
			}
			must.Wait(t, wait.InitialSuccess(
				wait.BoolFunc(isGone),
				wait.Timeout(10*time.Second),
				wait.Gap(500*time.Millisecond),
			), must.Sprint("test job failed to purge"))
		})

		// Make a pack deployment so we can validate the "no-change" condition
		t.Run("setup for pack_against_deployed", func(t *testing.T) {
			result := runTestPackCmd(t, s, []string{"run", getTestPackPath(t, testPack)})
			must.Eq(t, "", result.cmdErr.String(), must.Sprintf("cmdErr should be empty, but was %q", result.cmdErr.String()))
			must.StrContains(t, result.cmdOut.String(), "")
			must.Zero(t, result.exitCode) // Should return 0
			isStarted := func() bool {
				j, err := ct.NomadJobStatus(s, testPack)
				if err != nil {
					return false
				}
				return *j.Status == "running"
			}
			must.Wait(t, wait.InitialSuccess(
				wait.BoolFunc(isStarted),
				wait.Timeout(30*time.Second),
				wait.Gap(500*time.Millisecond),
			), must.Sprint("test job failed to start"))
		})

		t.Run("pack_against_deployed", func(t *testing.T) {
			// Plan against deployed - should be no-changes
			result := runTestPackCmd(t, s, testPlanCommand(t))
			must.Eq(t, "", result.cmdErr.String(), must.Sprintf("cmdErr should be empty, but was %q", result.cmdErr.String()))
			must.StrContains(t, result.cmdOut.String(), "Plan succeeded\n")
			must.Eq(t, exitcodeNoChanges, result.exitCode, must.Sprintf("stdout:\n%s\n\nstderr:\n%s\n", result.cmdOut.String(), result.cmdErr.String())) // Should return exit-code-no-changes
		})
	})
}

func TestCLI_PackStop(t *testing.T) {
	ct.HTTPTestParallel(t, ct.WithDefaultConfig(), func(s *agent.TestAgent) {
		expectGoodPackDeploy(t, runTestPackCmd(t, s, []string{"run", getTestPackPath(t, testPack)}))

		result := runTestPackCmd(t, s, []string{"stop", getTestPackPath(t, testPack), "--purge=true"})
		must.Eq(t, result.cmdErr.String(), "", must.Sprintf("cmdErr should be empty, but was %q", result.cmdErr.String()))
		must.StrContains(t, result.cmdOut.String(), `Pack "`+testPack+`" destroyed`)
		must.Zero(t, result.exitCode)
	})
}

func TestCLI_PackStop_Conflicts(t *testing.T) {
	ct.HTTPTestParallel(t, ct.WithDefaultConfig(), func(s *agent.TestAgent) {

		testCases := []struct {
			desc           string
			nonPackJob     bool
			packName       string
			deploymentName string
			jobName        string
			namespace      string
		}{
			// Give these each different job names so there's no conflicts
			// between the different tests cases when running
			{
				desc:           "non-pack-job",
				nonPackJob:     true,
				packName:       testPack,
				deploymentName: "",
				jobName:        testPack,
			},
			{
				desc:           "same-pack-diff-deploy",
				nonPackJob:     false,
				packName:       testPack,
				deploymentName: "foo",
				jobName:        "job2",
			},
			{
				desc:           "same-pack-diff-namespace",
				nonPackJob:     false,
				packName:       testPack,
				deploymentName: "",
				jobName:        testPack,
				namespace:      "job",
			},
		}
		client, err := ct.NewTestClient(s)
		must.NoError(t, err)
		for _, tC := range testCases {
			t.Run(tC.desc, func(t *testing.T) {
				defer ct.NomadCleanup(s)

				if tC.namespace != "" {
					ct.MakeTestNamespaces(t, client)
				}

				// Create job
				if tC.nonPackJob {
					err = ct.NomadRun(s, getTestNomadJobPath(t, testPack))
					must.NoError(t, err)
				} else {
					deploymentName := fmt.Sprintf("--name=%s", tC.deploymentName)
					varJobName := fmt.Sprintf("--var=job_name=%s", tC.jobName)
					if tC.namespace != "" {
						namespaceFlag := fmt.Sprintf("--namespace=%s", tC.namespace)
						expectGoodPackDeploy(t, runTestPackCmd(t, s, []string{"run", getTestPackPath(t, testPack), deploymentName, varJobName, namespaceFlag}))
					} else {
						expectGoodPackDeploy(t, runTestPackCmd(t, s, []string{"run", getTestPackPath(t, testPack), deploymentName, varJobName}))
					}
				}

				// Try to stop job
				result := runTestPackCmd(t, s, []string{"stop", tC.packName})
				must.Eq(t, 1, result.exitCode)
			})
		}
	})
}

// Destroy is just an alias for stop --purge so we only need to
// test that specific functionality
func TestCLI_PackDestroy(t *testing.T) {
	ct.HTTPTestParallel(t, ct.WithDefaultConfig(), func(s *agent.TestAgent) {
		expectGoodPackDeploy(t, runTestPackCmd(t, s, []string{"run", getTestPackPath(t, testPack)}))

		result := runTestPackCmd(t, s, []string{"destroy", getTestPackPath(t, testPack)})
		must.StrContains(t, result.cmdOut.String(), `Pack "`+testPack+`" destroyed`)
		must.Zero(t, result.exitCode)

		// Assert job no longer queryable
		c, err := ct.NewTestClient(s)
		must.NoError(t, err)

		r, _, err := c.Jobs().Info(testPack, &api.QueryOptions{})
		must.Nil(t, r)
		must.EqError(t, err, "Unexpected response code: 404 (job not found)")
	})
}

// Test that destroy properly uses var overrides to target the job
func TestCLI_PackDestroy_WithOverrides(t *testing.T) {
	ct.HTTPTestParallel(t, ct.WithDefaultConfig(), func(s *agent.TestAgent) {
		c, err := ct.NewTestClient(s)
		must.NoError(t, err)
		// Because this test uses ref, it requires a populated pack cache.
		reg, _, regPath := createTestRegistries(t)
		defer cleanTestRegistry(t, regPath)

		testCases := []struct {
			desc             string
			initialJobs      []string // Jobs to create before the test
			destroyJobName   string   // Job name to destroy (via --var)
			expectedExitCode int
			expectError      bool
			remainingJobs    []string // Jobs that should still exist after destroy
			deletedJobs      []string // Jobs that should be gone after destroy
		}{
			{
				desc:             "destroy-nonexistent-job",
				initialJobs:      []string{"foo", "bar"},
				destroyJobName:   "baz",
				expectedExitCode: 1,
				expectError:      true,
				remainingJobs:    []string{"foo", "bar"},
				deletedJobs:      []string{},
			},
			{
				desc:             "destroy-specific-job-with-override",
				initialJobs:      []string{"foo", "bar"},
				destroyJobName:   "foo",
				expectedExitCode: 0,
				expectError:      false,
				remainingJobs:    []string{"bar"},
				deletedJobs:      []string{"foo"},
			},
			{
				desc:             "destroy-with-prefix-conflict",
				initialJobs:      []string{"service", "service-test", "service-prod"},
				destroyJobName:   "service",
				expectedExitCode: 0,
				expectError:      false,
				remainingJobs:    []string{"service-test", "service-prod"},
				deletedJobs:      []string{"service"},
			},
			{
				desc:             "destroy-all-with-no-override",
				initialJobs:      []string{"job1", "job2"},
				destroyJobName:   "", // No var override, should destroy all
				expectedExitCode: 0,
				expectError:      false,
				remainingJobs:    []string{},
				deletedJobs:      []string{"job1", "job2"},
			},
		}

		for _, tC := range testCases {
			t.Run(tC.desc, func(t *testing.T) {
				// Setup: Clean up any existing jobs first
				defer ct.NomadCleanup(s)

				// Create initial jobs
				for _, jobName := range tC.initialJobs {
					result := runTestPackCmd(
						t, s, []string{"run", testPack, "--var=job_name=" + jobName, "--registry=" + reg.Name})
					expectGoodPackDeploy(t, result)
				}

				// Execute destroy command
				var args []string
				if tC.destroyJobName != "" {
					args = []string{"destroy", testPack, "--var=job_name=" + tC.destroyJobName, "--registry=" + reg.Name}
				} else {
					args = []string{"destroy", testPack, "--registry=" + reg.Name}
				}
				result := runTestPackCmd(t, s, args)

				// Verify exit code
				must.Eq(t, tC.expectedExitCode, result.exitCode, must.Sprintf(
					"expected exitcode %d; got %v\ncmdOut:%v", tC.expectedExitCode, result.exitCode, result.cmdOut.String()))

				// Verify remaining jobs still exist
				for _, jobName := range tC.remainingJobs {
					j, _, err := c.Jobs().Info(jobName, &api.QueryOptions{WaitTime: 5 * time.Second})
					must.NoError(t, err, must.Sprintf("job %s should still exist", jobName))
					must.NotNil(t, j, must.Sprintf("job %s should not be nil", jobName))
				}

				// Verify deleted jobs are gone
				for _, jobName := range tC.deletedJobs {
					j, _, err := c.Jobs().Info(jobName, &api.QueryOptions{WaitTime: 5 * time.Second})
					must.Error(t, err, must.Sprintf("job %s should be deleted", jobName))
					must.Eq(t, "Unexpected response code: 404 (job not found)", err.Error())
					must.Nil(t, j, must.Sprintf("deleted job %s should be nil", jobName))
				}
			})
		}
	})
}

func TestCLI_PackDestroy_PrefixListBehavior(t *testing.T) {
	ct.HTTPTestParallel(t, ct.WithDefaultConfig(), func(s *agent.TestAgent) {
		c, err := ct.NewTestClient(s)
		must.NoError(t, err)

		reg, _, regPath := createTestRegistries(t)
		defer cleanTestRegistry(t, regPath)
		defer ct.NomadCleanup(s)

		// Create jobs that will test prefix matching behavior
		// These job names are chosen to test edge cases:
		// - "service" is the base name
		// - "service-aaa" and "service-zzz" test alphabetical sorting
		// - All three start with "service" prefix
		jobNames := []string{"service", "service-aaa", "service-zzz"}
		for _, jobName := range jobNames {
			result := runTestPackCmd(t, s, []string{
				"run", testPack,
				"--var=job_name=" + jobName,
				"--registry=" + reg.Name,
			})
			expectGoodPackDeploy(t, result)
		}

		// Verify PrefixList returns all matching jobs
		jobs, _, err := c.Jobs().PrefixList("service")
		must.NoError(t, err)
		must.Eq(t, 3, len(jobs), must.Sprintf("expected 3 jobs with prefix 'service', got %d", len(jobs)))

		// Verify jobs are sorted (PrefixList guarantees lexicographic order)
		must.Eq(t, "service", jobs[0].ID, must.Sprint("first job should be 'service'"))
		must.Eq(t, "service-aaa", jobs[1].ID, must.Sprint("second job should be 'service-aaa'"))
		must.Eq(t, "service-zzz", jobs[2].ID, must.Sprint("third job should be 'service-zzz'"))

		// Now destroy the exact match "service"
		// This tests that the destroy logic correctly identifies "service"
		// among the prefix matches and only destroys that specific job
		result := runTestPackCmd(t, s, []string{
			"destroy", testPack,
			"--var=job_name=service",
			"--registry=" + reg.Name,
		})
		must.Zero(t, result.exitCode, must.Sprintf("destroy should succeed, got exitcode %d\ncmdOut: %v",
			result.exitCode, result.cmdOut.String()))

		// Verify only the exact match "service" was destroyed
		j, _, err := c.Jobs().Info("service", &api.QueryOptions{})
		must.Error(t, err, must.Sprint("job 'service' should be deleted"))
		must.Nil(t, j, must.Sprint("deleted job 'service' should be nil"))

		// Verify other jobs with similar prefixes still exist
		for _, jobName := range []string{"service-aaa", "service-zzz"} {
			j, _, err := c.Jobs().Info(jobName, &api.QueryOptions{})
			must.NoError(t, err, must.Sprintf("job %s should still exist", jobName))
			must.NotNil(t, j, must.Sprintf("job %s should not be nil", jobName))
		}

		// Additional test: Destroy one of the remaining jobs to verify
		// the logic works for non-first matches in the sorted list
		result = runTestPackCmd(t, s, []string{
			"destroy", testPack,
			"--var=job_name=service-zzz",
			"--registry=" + reg.Name,
		})
		must.Zero(t, result.exitCode, must.Sprintf("destroy should succeed, got exitcode %d", result.exitCode))

		// Verify "service-zzz" was destroyed
		j, _, err = c.Jobs().Info("service-zzz", &api.QueryOptions{})
		must.Error(t, err, must.Sprint("job 'service-zzz' should be deleted"))
		must.Nil(t, j, must.Sprint("deleted job 'service-zzz' should be nil"))

		// Verify "service-aaa" still exists
		j, _, err = c.Jobs().Info("service-aaa", &api.QueryOptions{})
		must.NoError(t, err, must.Sprint("job 'service-aaa' should still exist"))
		must.NotNil(t, j, must.Sprint("job 'service-aaa' should not be nil"))
	})
}

func TestCLI_CLIFlag_NotDefined(t *testing.T) {
	t.Parallel() // nomad not required

	// There is no job flag. This tests that adding an unspecified flag does not
	// create an invalid memory address error
	// Posix case
	result := runPackCmd(t, []string{"run", "nginx", "--job=provided-but-not-defined"})
	must.Eq(t, 1, result.exitCode)

	// std go case
	result = runPackCmd(t, []string{"run", "-job=provided-but-not-defined", "nginx"})
	must.Eq(t, 1, result.exitCode)
}

func TestCLI_PackStatus(t *testing.T) {
	ct.HTTPTestParallel(t, ct.WithDefaultConfig(), func(s *agent.TestAgent) {
		result := runTestPackCmd(t, s, []string{"run", getTestPackPath(t, testPack)})
		must.Zero(t, result.exitCode)

		testcases := []struct {
			name string
			args []string
		}{
			{
				name: "no-pack-name",
				args: []string{},
			},
			{
				name: "with-pack-name",
				args: []string{testPack},
			},
			{
				name: "with-pack-and-registry-name",
				args: []string{testPack, "--registry=default"},
			},
			{
				name: "with-pack-and-ref",
				args: []string{testPack, "--ref=latest"},
			},
		}

		for _, tc := range testcases {
			t.Run(tc.name, func(t *testing.T) {
				args := append([]string{"status"}, tc.args...)
				result := runTestPackCmd(t, s, args)
				must.Zero(t, result.exitCode)
				must.StrContains(t, result.cmdOut.String(), "simple_raw_exec | "+caching.DevRegistryName+" ")
			})
		}
	})
}

func TestCLI_PackStatus_Fails(t *testing.T) {
	ct.HTTPTestParallel(t, ct.WithDefaultConfig(), func(s *agent.TestAgent) {
		// test for status on missing pack
		result := runTestPackCmd(t, s, []string{"status", getTestPackPath(t, testPack)})
		must.Eq(t, result.cmdErr.String(), "", must.Sprintf("cmdErr should be empty, but was %q", result.cmdErr.String()))
		must.StrContains(t, result.cmdOut.String(), "no jobs found for pack \""+getTestPackPath(t, testPack)+"\"")
		// FIXME: Should this have a non-success exit-code?
		must.Zero(t, result.exitCode)

		// test flag validation for name flag without pack
		result = runTestPackCmd(t, s, []string{"status", "--name=foo"})
		must.Eq(t, 1, result.exitCode)
		must.StrContains(t, result.cmdOut.String(), "--name can only be used if pack name is provided")
	})
}

func TestCLI_PackRender_RootVar(t *testing.T) {
	t.Parallel()
	// This test has to do some extra shenanigans because dependent pack template
	// output is not guaranteed to be ordered. This requires that the test handle
	// either order.
	expected := []string{
		"deps_test/child1/child1.nomad=child1",
		"deps_test/child2/child2.nomad=child2",
		"deps_test/deps_test.nomad=deps_test",
	}

	result := runPackCmd(t, []string{
		"render",
		"--no-format=true",
		getTestPackPath(t, "my_alias_test"),
	})
	must.Eq(t, result.cmdErr.String(), "", must.Sprintf("cmdErr should be empty, but was %q", result.cmdErr.String()))

	// Performing a little clever string manipulation on the render output to
	// prepare it for splitting into a slice of string enables us to use
	// require.ElementsMatch to validate goodness.
	outStr := strings.TrimSpace(result.cmdOut.String())
	outStr = strings.ReplaceAll(outStr, ":\n\n", "=")
	elems := strings.Split(outStr, "\n")

	must.SliceContainsAll(t, expected, elems)
}

func TestCLI_PackRender_SetDepVarWithFlag(t *testing.T) {
	t.Parallel()
	// This test has to do some extra shenanigans because dependent pack template
	// output is not guaranteed to be ordered. This requires that the test handle
	// either order.
	expected := []string{
		"deps_test/child1/child1.nomad=override",
		"deps_test/child2/child2.nomad=child2",
		"deps_test/deps_test.nomad=deps_test",
	}

	result := runPackCmd(t, []string{
		"render",
		"--no-format=true",
		"--var", "child1.job_name=override",
		getTestPackPath(t, "my_alias_test"),
	})

	must.Eq(t, result.cmdErr.String(), "", must.Sprintf("cmdErr should be empty, but was %q", result.cmdErr.String()))
	must.Eq(t, 0, result.exitCode, must.Sprintf("incorrect exit code.\nstdout:\n%v\nstderr:%v\n", result.cmdOut.String(), result.cmdErr.String()))

	// Performing a little clever string manipulation on the render output to
	// prepare it for splitting into a slice of string enables us to use
	// require.ElementsMatch to validate goodness.
	outStr := strings.TrimSpace(result.cmdOut.String())
	outStr = strings.ReplaceAll(outStr, ":\n\n", "=")
	elems := strings.Split(outStr, "\n")

	must.SliceContainsAll(t, expected, elems)
}

func TestCLI_PackRender_VarsInOutputTemplate(t *testing.T) {
	t.Parallel()
	// This test has to do some extra shenanigans because dependent pack template
	// output is not guaranteed to be ordered. This requires that the test handle
	// either order.
	expected := []string{
		"deps_test/child1/child1.nomad=override",
		"deps_test/child2/child2.nomad=child2",
		"deps_test/deps_test.nomad=deps_test",
		"outputs.tpl=deps_test,override,child2",
	}

	result := runPackCmd(t, []string{
		"render",
		"--no-format=true",
		"--var", "child1.job_name=override",
		"--render-output-template=true",
		getTestPackPath(t, "my_alias_test"),
	})

	must.Eq(t, result.cmdErr.String(), "", must.Sprintf("cmdErr should be empty, but was %q", result.cmdErr.String()))
	must.Eq(t, 0, result.exitCode, must.Sprintf("incorrect exit code.\ncmdOut:\n%v\ns", result.cmdOut.String()))
	must.StrNotContains(t, result.cmdOut.String(), "Error")

	// Performing a little clever string manipulation on the render output to
	// prepare it for splitting into a slice of string enables us to use
	// require.ElementsMatch to validate goodness.
	outStr := strings.TrimSpace(result.cmdOut.String())
	outStr = strings.ReplaceAll(outStr, ":\n\n", "=")
	elems := strings.Split(outStr, "\n")

	must.SliceContainsAll(t, expected, elems, must.Sprintf("unexpected returned value.\nexpected: %v\nelems: %v\nstdout:\n%v\n", expected, elems, result.cmdOut.String()))
}

func TestCLI_CLIFlag_Namespace(t *testing.T) {
	testCases := []struct {
		desc   string
		args   []string
		env    map[string]string
		expect map[string]int
	}{
		{
			desc: "client flag vs unspecified",
			args: []string{
				`--namespace=flag`,
			},
			env: make(map[string]string),
			expect: map[string]int{
				"job":  0,
				"flag": 1,
				"env":  0,
			},
		},
		{
			desc: "flags vs job",
			args: []string{
				`--var=namespace=job`,
				`--namespace=flag`,
			},
			env: make(map[string]string),
			expect: map[string]int{
				"job":  1,
				"flag": 0,
				"env":  0,
			},
		},
		{
			desc: "flags vs second flag",
			args: []string{
				`--namespace=job`,
				`--namespace=flag`,
			},
			env: make(map[string]string),
			expect: map[string]int{
				"job":  0,
				"flag": 1,
				"env":  0,
			},
		},
	}
	for _, tC := range testCases {
		t.Run(tC.desc, func(t *testing.T) {
			ct.HTTPTestParallel(t, ct.WithDefaultConfig(), func(srv *agent.TestAgent) {
				c, err := ct.NewTestClient(srv)
				must.NoError(t, err)

				ct.MakeTestNamespaces(t, c)

				result := runTestPackCmd(t, srv, append([]string{
					"run",
					getTestPackPath(t, testPack),
				},
					tC.args...),
				)
				must.Eq(t, result.cmdErr.String(), "", must.Sprintf("cmdErr should be empty, but was %q", result.cmdErr.String()))
				must.StrContains(t, result.cmdOut.String(), "Pack successfully deployed", must.Sprintf(
					"Expected success message, received %q", result.cmdOut.String()))

				for ns, count := range tC.expect {
					tJobs, _, err := c.Jobs().List(&api.QueryOptions{Namespace: ns})
					must.NoError(t, err)
					must.Eq(t, count, len(tJobs), must.Sprintf("Expected %v job(s) in %q namespace; found %v", count, ns, len(tJobs)))
				}
			})
		})
	}
}

func TestCLI_CLIFlag_Token(t *testing.T) {
	ct.HTTPTestWithACLParallel(t, ct.WithDefaultConfig(), func(srv *agent.TestAgent) {
		c, err := ct.NewTestClient(srv)
		must.NoError(t, err)

		ct.MakeTestNamespaces(t, c)

		result := runTestPackCmd(t, srv, []string{
			"run",
			getTestPackPath(t, testPack),
			"--token=bad00000-bad0-bad0-bad0-badbadbadbad",
		})
		must.Eq(t, result.cmdErr.String(), "", must.Sprintf("cmdErr should be empty, but was %q", result.cmdErr.String()))
		must.StrContains(t, result.cmdOut.String(), "403 (Permission denied)", must.Sprintf(
			"Expected token not found error, received %q", result.cmdOut.String()))

		result = runTestPackCmd(t, srv, []string{
			"run",
			getTestPackPath(t, testPack),
			"--token=" + srv.Config.Client.Meta["token"],
		})

		must.Eq(t, result.cmdErr.String(), "", must.Sprintf("cmdErr should be empty, but was %q", result.cmdErr.String()))
		must.StrContains(t, result.cmdOut.String(), "Pack successfully deployed", must.Sprintf(
			"Expected success message, received %q", result.cmdOut.String()))
	})
}

func TestCLI_EnvConfig_Token(t *testing.T) {
	ct.HTTPTestWithACL(t, ct.WithDefaultConfig(), func(srv *agent.TestAgent) {
		_, err := ct.NewTestClient(srv)
		must.NoError(t, err)

		// Garbage token - Should fail
		t.Setenv("NOMAD_TOKEN", badACLToken)

		result := runTestPackCmd(t, srv, []string{
			"run",
			getTestPackPath(t, testPack),
		})
		must.Eq(t, result.cmdErr.String(), "", must.Sprintf("cmdErr should be empty, but was %q", result.cmdErr.String()))
		must.StrContains(t, result.cmdOut.String(), "403 (Permission denied)", must.Sprintf(
			"Expected token not found error, received %q", result.cmdOut.String()))

		// Good token - Should run
		t.Setenv("NOMAD_TOKEN", srv.Config.Client.Meta["token"])
		result = runTestPackCmd(t, srv, []string{
			"run",
			getTestPackPath(t, testPack),
		})

		must.Eq(t, result.cmdErr.String(), "", must.Sprintf("cmdErr should be empty, but was %q", result.cmdErr.String()))
		must.StrContains(t, result.cmdOut.String(), "Pack successfully deployed", must.Sprintf(
			"Expected success message, received %q", result.cmdOut.String()))
	})
}

// This test can't benefit from parallelism since it mutates the environment
func TestCLI_EnvConfig_Namespace(t *testing.T) {
	testCases := []struct {
		desc   string
		args   []string
		env    map[string]string
		expect map[string]int
	}{
		{
			desc: "flags vs unspecified",
			args: []string{},
			expect: map[string]int{
				"job":  0,
				"flag": 0,
				"env":  1,
			},
		},
		{
			desc: "env vs job",
			args: []string{
				`--var=namespace=job`,
			},
			expect: map[string]int{
				"job":  1,
				"flag": 0,
				"env":  0,
			},
		},
		{
			desc: "env vs flag",
			args: []string{
				`--namespace=flag`,
			},
			expect: map[string]int{
				"job":  0,
				"flag": 1,
				"env":  0,
			},
		},
		{
			desc: "env vs flag vs job",
			args: []string{
				`--namespace=flag`,
				`--var=namespace=job`,
			},
			expect: map[string]int{
				"job":  1,
				"flag": 0,
				"env":  0,
			},
		},
	}
	for _, tC := range testCases {
		t.Run(tC.desc, func(t *testing.T) {
			ct.HTTPTest(t, ct.WithDefaultConfig(), func(srv *agent.TestAgent) {
				c, err := ct.NewTestClient(srv)
				must.NoError(t, err)

				ct.MakeTestNamespaces(t, c)

				// Always set the namespace environment variable
				t.Setenv("NOMAD_NAMESPACE", "env")
				result := runTestPackCmd(t, srv, append([]string{
					"run",
					getTestPackPath(t, testPack),
				},
					tC.args...),
				)
				must.Eq(t, result.cmdErr.String(), "", must.Sprintf(
					"cmdErr should be empty, but was %q", result.cmdErr.String()))
				must.StrContains(t, result.cmdOut.String(), "Pack successfully deployed", must.Sprintf(
					"Expected success message, received %q", result.cmdOut.String()))

				for ns, count := range tC.expect {
					tJobs, _, err := c.Jobs().List(&api.QueryOptions{Namespace: ns})
					must.NoError(t, err)
					must.Eq(t, count, len(tJobs), must.Sprintf(
						"Expected %v job(s) in %q namespace; found %v", count, ns, len(tJobs)))
				}
			})
		})
	}
}

type PackCommandResult struct {
	exitCode int
	cmdOut   *bytes.Buffer
	cmdErr   *bytes.Buffer
}

func AddressFromTestServer(srv *agent.TestAgent) []string {
	srv.T.Helper()
	return []string{"--address", srv.HTTPAddr()}
}

func runTestPackCmd(t *testing.T, srv *agent.TestAgent, args []string) PackCommandResult {
	t.Helper()
	args = append(args, AddressFromTestServer(srv)...)
	return runPackCmd(t, args)
}

func runPackCmd(t *testing.T, args []string) PackCommandResult {
	t.Helper()
	cmdOut := bytes.NewBuffer(make([]byte, 0))
	cmdErr := bytes.NewBuffer(make([]byte, 0))

	// Build our cancellation context
	ctx, closer := helper.WithInterrupt(context.Background())
	defer closer()

	// Make a test UI
	ui := testui.NonInteractiveTestUI(ctx, cmdOut, cmdErr)

	// Get our base command
	fset := flag.NewSets()
	base, commands := Commands(ctx, WithFlags(fset), WithUI(ui))
	defer base.Close()

	command := &cli.CLI{
		Name:                       "nomad-pack",
		Args:                       args,
		Version:                    fmt.Sprintf("Nomad Pack %s", version.HumanVersion()),
		Commands:                   commands,
		Autocomplete:               true,
		AutocompleteNoDefaultFlags: true,
		HelpFunc:                   GroupedHelpFunc(cli.BasicHelpFunc(cliName)),
		HelpWriter:                 cmdOut,
		ErrorWriter:                cmdErr,
	}

	t.Logf("Running nomad-pack\n  args:%v", command.Args)

	// Run the CLI
	exitCode, err := command.Run()
	if err != nil {
		panic(err)
	}

	must.Eq(t, cmdErr.String(), "", must.Sprintf("cmdErr should be empty, but was %q", cmdErr.String()))

	return PackCommandResult{
		exitCode: exitCode,
		cmdOut:   cmdOut,
		cmdErr:   cmdErr,
	}
}

// Test Helper functions

// getTestPackPath returns the full path to a pack in the test fixtures folder.
func getTestPackPath(t *testing.T, packname string) string {
	t.Helper()
	return path.Join(getTestPackRegistryPath(t), "packs", packname)
}

// getTestPackRegistryPath returns the full path to a registry in the test
// fixtures folder.
func getTestPackRegistryPath(t *testing.T) string {
	t.Helper()
	return path.Join(testfixture.AbsPath(t, "v2/test_registry"))
}

// getTestNomadJobPath returns the full path to a pack in the test
// fixtures/jobspecs folder. The `.nomad` extension will be added
// for you.
func getTestNomadJobPath(t *testing.T, job string) string {
	t.Helper()
	return path.Join(testfixture.AbsPath(t, path.Join("jobspecs", job+".nomad")))
}

// expectGoodPackDeploy bundles the test expectations that should be met when
// determining if the pack CLI successfully deployed a pack.
func expectGoodPackDeploy(t *testing.T, r PackCommandResult) {
	t.Helper()
	expectNoStdErrOutput(t, r)
	must.StrContains(t, r.cmdOut.String(), "Pack successfully deployed", must.Sprintf(
		"Expected success message, received %q", r.cmdOut.String()))
	must.Zero(t, r.exitCode)
}

// expectGoodPackPlan bundles the test expectations that should be met when
// determining if the pack CLI successfully planned a pack.
func expectGoodPackPlan(t *testing.T, r PackCommandResult) {
	t.Helper()
	expectNoStdErrOutput(t, r)
	must.StrContains(t, r.cmdOut.String(), "Plan succeeded", must.Sprintf(
		"Expected success message, received %q", r.cmdOut.String()))
	must.Eq(t, 1, r.exitCode) // exitcode 1 means that an allocation will be created
}

// createTestRegistries creates two registries: first one has "latest" ref,
// second one has testRef ref. It returns registry objects, and a string that
// points to the root where the two refs are on the filesystem.
func createTestRegistries(t *testing.T) (*caching.Registry, *caching.Registry, string) {
	t.Helper()

	// Fake a clone
	registryName := fmt.Sprintf("test-%v", time.Now().UnixMilli())

	regDir := path.Join(caching.DefaultCachePath(), registryName)
	err := filesystem.MaybeCreateDestinationDir(regDir)
	must.NoError(t, err)

	for _, r := range []string{"latest", testRef} {
		must.NoError(t, filesystem.CopyDir(
			getTestPackPath(t, testPack),
			path.Join(regDir, r, testPack+"@"+r),
			false,
			logging.Default(),
		))
	}

	// create output registries and metadata.json files
	latestReg := &caching.Registry{
		Name:     registryName,
		Source:   "github.com/hashicorp/nomad-pack-test-registry",
		LocalRef: testRef,
		Ref:      "latest",
	}
	latestMetaPath := path.Join(regDir, "latest", "metadata.json")
	b, _ := json.Marshal(latestReg)
	must.NoError(t, os.WriteFile(latestMetaPath, b, 0644))

	testRefReg := &caching.Registry{
		Name:     registryName,
		Source:   "github.com/hashicorp/nomad-pack-test-registry",
		LocalRef: testRef,
		Ref:      testRef,
	}
	testRefMetaPath := path.Join(regDir, testRef, "metadata.json")
	b, _ = json.Marshal(testRefReg)
	must.NoError(t, os.WriteFile(testRefMetaPath, b, 0644))

	return latestReg, testRefReg, regDir
}

func cleanTestRegistry(t *testing.T, regPath string) {
	t.Helper()
	os.RemoveAll(regPath)
}

func expectNoStdErrOutput(t *testing.T, r PackCommandResult) {
	t.Helper()
	must.Eq(t, "", r.cmdErr.String(), must.Sprintf("cmdErr should be empty, but was %q", r.cmdErr.String()))
}
