// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path"
	"strings"
	"testing"
	"time"

	"github.com/hashicorp/nomad/api"
	"github.com/hashicorp/nomad/command/agent"
	"github.com/mitchellh/cli"
	"github.com/shoenig/test/must"
	"github.com/shoenig/test/wait"

	ct "github.com/hashicorp/nomad-pack/internal/cli/testhelper"
	"github.com/hashicorp/nomad-pack/internal/pkg/cache"
	flag "github.com/hashicorp/nomad-pack/internal/pkg/flag"
	"github.com/hashicorp/nomad-pack/internal/pkg/helper"
	"github.com/hashicorp/nomad-pack/internal/pkg/helper/filesystem"
	"github.com/hashicorp/nomad-pack/internal/pkg/logging"
	"github.com/hashicorp/nomad-pack/internal/pkg/testfixture"
	"github.com/hashicorp/nomad-pack/internal/pkg/version"
	"github.com/hashicorp/nomad-pack/internal/runner/job"
	"github.com/hashicorp/nomad-pack/internal/testui"
)

func TestCLI_V1_JobRun(t *testing.T) {
	ct.HTTPTestParallel(t, ct.WithDefaultConfig(), func(s *agent.TestAgent) {
		expectGoodPackDeploy(t, runTestPackV1Cmd(t, s, []string{"run", getTestPackV1Path(t, testPack)}))
	})
}

// Confirm that another pack with the same job names but a different deployment name fails
func TestCLI_V1_JobRunConflictingDeployment(t *testing.T) {
	ct.HTTPTestParallel(t, ct.WithDefaultConfig(), func(s *agent.TestAgent) {
		expectGoodPackDeploy(t, runTestPackV1Cmd(t, s, []string{"run", getTestPackV1Path(t, testPack)}))

		result := runTestPackV1Cmd(t, s, []string{"run", getTestPackV1Path(t, testPack), "--name=with-name"})
		must.Eq(t, 1, result.exitCode)
		expectNoStdErrOutput(t, result)
		must.StrContains(t, result.cmdOut.String(), job.ErrExistsInDeployment{JobID: testPack, Deployment: testPack}.Error())

		// Confirm that it's still possible to update the existing pack
		expectGoodPackDeploy(t, runTestPackV1Cmd(t, s, []string{"run", getTestPackV1Path(t, testPack)}))
	})
}

// Check for conflict with non-pack job i.e. no meta
func TestCLI_V1_JobRunConflictingNonPackJob(t *testing.T) {
	ct.HTTPTestParallel(t, ct.WithDefaultConfig(), func(s *agent.TestAgent) {
		// Register non pack job
		err := ct.NomadRun(s, getTestNomadJobPath(t, testPack))
		must.NoError(t, err)

		// Now try to register the pack
		result := runTestPackV1Cmd(t, s, []string{"run", getTestPackV1Path(t, testPack)})

		must.Eq(t, 1, result.exitCode)
		expectNoStdErrOutput(t, result)
		must.StrContains(t, result.cmdOut.String(), job.ErrExistsNonPack{JobID: testPack}.Error())
	})
}

// Check for conflict with job that has meta
func TestCLI_V1_JobRunConflictingJobWithMeta(t *testing.T) {
	ct.HTTPTestParallel(t, ct.WithDefaultConfig(), func(s *agent.TestAgent) {
		// Register non pack job
		err := ct.NomadRun(s, getTestNomadJobPath(t, "simple_raw_exec_with_meta"))
		must.NoError(t, err)

		// Now try to register the pack
		result := runTestPackV1Cmd(t, s, []string{"run", getTestPackV1Path(t, testPack)})
		must.Eq(t, 1, result.exitCode)
		expectNoStdErrOutput(t, result)
		must.StrContains(t, result.cmdOut.String(), job.ErrExistsNonPack{JobID: testPack}.Error())
	})
}

func TestCLI_V1_JobRunFails(t *testing.T) {
	t.Parallel()
	// This test doesn't require a Nomad cluster.
	result := runPackV1Cmd(t, []string{"run", "fake-job"})

	must.Eq(t, 1, result.exitCode)
	expectNoStdErrOutput(t, result)
	must.StrContains(t, result.cmdOut.String(), "Failed To Find Pack")
}

func TestCLI_V1_JobPlan(t *testing.T) {
	ct.HTTPTestParallel(t, ct.WithDefaultConfig(), func(s *agent.TestAgent) {
		expectGoodPackPlan(t, runTestPackV1Cmd(t, s, []string{"plan", getTestPackV1Path(t, testPack)}))
	})
}

func TestCLI_V1_JobPlan_BadJob(t *testing.T) {
	ct.HTTPTestParallel(t, ct.WithDefaultConfig(), func(s *agent.TestAgent) {
		result := runTestPackV1Cmd(t, s, []string{"plan", "fake-job"})

		must.Eq(t, 255, result.exitCode) // Should return 255 indicating an error
		must.Eq(t, "", result.cmdErr.String(), must.Sprintf("cmdErr should be empty, but was %q", result.cmdErr.String()))
		must.StrContains(t, result.cmdOut.String(), "Failed To Find Pack")
	})
}

// Confirm that another pack with the same job names but a different deployment name fails
func TestCLI_V1_JobPlan_ConflictingDeployment(t *testing.T) {
	ct.HTTPTestParallel(t, ct.WithDefaultConfig(), func(s *agent.TestAgent) {
		reg, _, regPath := createV1TestRegistries(t)
		defer cleanTestRegistry(t, regPath)

		testRegFlag := "--registry=" + reg.Name
		expectGoodPackDeploy(t, runTestPackV1Cmd(t, s, []string{"run", testPack, testRegFlag}))

		result := runTestPackV1Cmd(t, s, []string{"run", testPack, testRegFlag, testRefFlag})
		must.Eq(t, 1, result.exitCode)
		expectNoStdErrOutput(t, result)
		must.StrContains(t, result.cmdOut.String(), job.ErrExistsInDeployment{JobID: testPack, Deployment: testPack + "@latest"}.Error())
	})
}

// Check for conflict with non-pack job i.e. no meta
func TestCLI_V1_JobPlan_ConflictingNonPackJob(t *testing.T) {
	ct.HTTPTestParallel(t, ct.WithDefaultConfig(), func(s *agent.TestAgent) {
		// Register non pack job
		err := ct.NomadRun(s, getTestNomadJobPath(t, testPack))
		must.NoError(t, err)

		// Now try to register the pack
		result := runTestPackV1Cmd(t, s, []string{"plan", getTestPackV1Path(t, testPack)})
		must.Eq(t, 255, result.exitCode) // Should return 255 indicating an error
		expectNoStdErrOutput(t, result)
		must.StrContains(t, result.cmdOut.String(), job.ErrExistsNonPack{JobID: testPack}.Error())
	})
}

func TestCLI_V1_PackPlan_OverrideExitCodes(t *testing.T) {
	ct.HTTPTest(t, ct.WithDefaultConfig(), func(s *agent.TestAgent) {
		testPlanCommand := func(t *testing.T) []string {
			out := strings.Split(testPlanCmdString, " ")
			out = append(out, getTestPackV1Path(t, testPack))
			return out
		}
		t.Run("plan_against_empty", func(t *testing.T) {
			// Plan against empty - should be makes-changes
			result := runTestPackV1Cmd(t, s, testPlanCommand(t))
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
			// Now try to register the pack, should make error
			result := runTestPackV1Cmd(t, s, testPlanCommand(t))
			must.Eq(t, "", result.cmdErr.String(), must.Sprintf("cmdErr should be empty, but was %q", result.cmdErr.String()))
			must.StrContains(t, result.cmdOut.String(), job.ErrExistsNonPack{JobID: testPack}.Error())
			must.Eq(t, exitcodeError, result.exitCode) // Should exit-code-error
		})

		t.Run("cleanup non-pack-job", func(t *testing.T) {
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
			// scoping this part because I reuse result in the last part
			result := runTestPackV1Cmd(t, s, []string{"run", getTestPackV1Path(t, testPack)})
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
			result := runTestPackV1Cmd(t, s, testPlanCommand(t))

			must.Eq(t, "", result.cmdErr.String(), must.Sprintf("cmdErr should be empty, but was %q", result.cmdErr.String()))
			must.StrContains(t, result.cmdOut.String(), "Plan succeeded\n")
			must.Eq(t, exitcodeNoChanges, result.exitCode, must.Sprintf("stdout:\n%s\n\nstderr:\n%s\n", result.cmdOut.String(), result.cmdErr.String())) // Should return exit-code-no-changes
		})
	})
}

func TestCLI_V1_PackStop(t *testing.T) {
	ct.HTTPTestParallel(t, ct.WithDefaultConfig(), func(s *agent.TestAgent) {
		expectGoodPackDeploy(t, runTestPackV1Cmd(t, s, []string{"run", getTestPackV1Path(t, testPack)}))

		result := runTestPackV1Cmd(t, s, []string{"stop", getTestPackV1Path(t, testPack), "--purge=true"})
		must.Zero(t, result.exitCode)
		expectNoStdErrOutput(t, result)
		must.StrContains(t, result.cmdOut.String(), `Pack "`+testPack+`" destroyed`)
	})
}

func TestCLI_V1_PackStop_Conflicts(t *testing.T) {
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
						expectGoodPackDeploy(t, runTestPackV1Cmd(t, s, []string{"run", getTestPackV1Path(t, testPack), deploymentName, varJobName, namespaceFlag}))
					} else {
						expectGoodPackDeploy(t, runTestPackV1Cmd(t, s, []string{"run", getTestPackV1Path(t, testPack), deploymentName, varJobName}))
					}
				}

				// Try to stop job
				result := runTestPackV1Cmd(t, s, []string{"stop", tC.packName})
				must.Eq(t, 1, result.exitCode)
			})
		}
	})
}

// Destroy is just an alias for stop --purge so we only need to
// test that specific functionality
func TestCLI_V1_PackDestroy(t *testing.T) {
	ct.HTTPTestParallel(t, ct.WithDefaultConfig(), func(s *agent.TestAgent) {
		expectGoodPackDeploy(t, runTestPackV1Cmd(t, s, []string{"run", getTestPackV1Path(t, testPack)}))

		result := runTestPackV1Cmd(t, s, []string{"destroy", getTestPackV1Path(t, testPack)})
		must.Eq(t, 0, result.exitCode)
		expectNoStdErrOutput(t, result)
		must.StrContains(t, result.cmdOut.String(), `Pack "`+testPack+`" destroyed`)

		// Assert job no longer queryable
		c, err := ct.NewTestClient(s)
		must.NoError(t, err)

		j, _, err := c.Jobs().Info("bar", &api.QueryOptions{})
		must.Nil(t, j)
		must.ErrorContains(t, err, "job not found")
	})
}

// Test that destroy properly uses var overrides to target the job
func TestCLI_V1_PackDestroy_WithOverrides(t *testing.T) {
	ct.HTTPTestParallel(t, ct.WithDefaultConfig(), func(s *agent.TestAgent) {
		c, err := ct.NewTestClient(s)
		must.NoError(t, err)
		// Because this test uses ref, it requires a populated pack cache.
		reg, _, regPath := createV1TestRegistries(t)
		defer cleanTestRegistry(t, regPath)

		// 	TODO: Table Testing
		// Create multiple jobs in the same pack deployment

		jobNames := []string{"foo", "bar"}
		for _, j := range jobNames {
			expectGoodPackDeploy(t, runTestPackV1Cmd(
				t,
				s,
				[]string{
					"run",
					testPack,
					"--var=job_name=" + j,
					"--registry=" + reg.Name,
				}))
		}

		// Stop nonexistent job
		result := runTestPackV1Cmd(t, s, []string{"destroy", testPack, "--var=job_name=baz", "--registry=" + reg.Name})
		must.Eq(t, 1, result.exitCode, must.Sprintf("expected exitcode 1; got %v\ncmdOut:%v", result.exitCode, result.cmdOut.String()))

		// Stop job with var override
		result = runTestPackV1Cmd(t, s, []string{"destroy", testPack, "--var=job_name=foo", "--registry=" + reg.Name})
		must.Zero(t, result.exitCode, must.Sprintf("expected exitcode 0; got %v\ncmdOut:%v", result.exitCode, result.cmdOut.String()))

		q := api.QueryOptions{}

		// Assert job "bar" still exists
		tCtx, done := context.WithTimeout(context.TODO(), 5*time.Second)
		j, _, err := c.Jobs().Info("bar", q.WithContext(tCtx))
		done()
		must.NoError(t, err)
		must.NotNil(t, j)

		// Stop job with no overrides passed
		result = runTestPackV1Cmd(t, s, []string{"destroy", testPack, "--registry=" + reg.Name})
		must.Zero(t, result.exitCode, must.Sprintf("expected exitcode 0; got %v\ncmdOut:%v", result.exitCode, result.cmdOut.String()))

		// Assert job bar is gone
		tCtx, done = context.WithTimeout(context.TODO(), 5*time.Second)
		j, _, err = c.Jobs().Info("bar", q.WithContext(tCtx))
		done()
		must.Error(t, err)
		must.Eq(t, "Unexpected response code: 404 (job not found)", err.Error())
		must.Nil(t, j)
	})
}

func TestCLI_V1_CLIFlag_NotDefined(t *testing.T) {
	t.Parallel() // nomad not required

	// There is no job flag. This tests that adding an unspecified flag does not
	// create an invalid memory address error
	// Posix case
	result := runPackV1Cmd(t, []string{"run", "nginx", "--job=provided-but-not-defined"})
	must.Eq(t, 1, result.exitCode)

	// std go case
	result = runPackV1Cmd(t, []string{"run", "-job=provided-but-not-defined", "nginx"})
	must.Eq(t, 1, result.exitCode)
}

func TestCLI_V1_PackStatus(t *testing.T) {
	ct.HTTPTestParallel(t, ct.WithDefaultConfig(), func(s *agent.TestAgent) {
		result := runTestPackV1Cmd(t, s, []string{"run", getTestPackV1Path(t, testPack)})
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
				result := runTestPackV1Cmd(t, s, args)
				must.Zero(t, result.exitCode)
				must.StrContains(t, result.cmdOut.String(), "simple_raw_exec | "+cache.DevRegistryName+" ")
			})
		}
	})
}

func TestCLI_V1_PackStatus_Fails(t *testing.T) {
	ct.HTTPTestParallel(t, ct.WithDefaultConfig(), func(s *agent.TestAgent) {
		// test for status on missing pack
		result := runTestPackV1Cmd(t, s, []string{"status", getTestPackV1Path(t, testPack)})
		expectNoStdErrOutput(t, result)
		must.StrContains(t, result.cmdOut.String(), "no jobs found for pack \""+getTestPackV1Path(t, testPack)+"\"")
		// FIXME: Should this have a non-success exit-code?
		must.Zero(t, result.exitCode)

		// test flag validation for name flag without pack
		result = runTestPackV1Cmd(t, s, []string{"status", "--name=foo"})
		must.Eq(t, 1, result.exitCode)
		must.StrContains(t, result.cmdOut.String(), "--name can only be used if pack name is provided")
	})
}

func TestCLI_V1_PackRender_MyAlias(t *testing.T) {
	t.Parallel()
	// This test has to do some extra shenanigans because dependent pack template
	// output is not guaranteed to be ordered. This requires that the test handle
	// either order.
	expected := []string{
		"child1/child1.nomad=child1",
		"child2/child2.nomad=child2",
		"deps_test/deps_test.nomad=deps_test",
	}

	result := runPackV1Cmd(t, []string{
		"render",
		"--no-format",
		getTestPackV1Path(t, "my_alias_test"),
	})
	expectNoStdErrOutput(t, result)
	output := result.cmdOut.String()
	must.StrNotContains(t, output, "failed to render")

	// Performing a little clever string manipulation on the render output to
	// prepare it for splitting into a slice of string enables us to use
	// must.CliceContainsAll to validate goodness.
	outStr := strings.TrimSpace(output)
	outStr = strings.ReplaceAll(outStr, ":\n\n", "=")
	elems := strings.Split(outStr, "\n")

	must.SliceContainsAll(t, expected, elems, must.Sprintf("expected: %v\n got: %v", expected, elems))
}

func TestCLI_V1_CLIFlag_Namespace(t *testing.T) {
	testCases := []struct {
		desc   string
		args   []string
		env    map[string]string
		expect map[string]int
	}{
		{
			desc: "flags vs unspecified",
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

				result := runTestPackV1Cmd(t, srv, append([]string{
					"run",
					getTestPackV1Path(t, testPack),
				},
					tC.args...),
				)
				expectNoStdErrOutput(t, result)
				must.StrContains(t, result.cmdOut.String(), "Pack successfully deployed", must.Sprintf("Expected success message, received %q", result.cmdOut.String()))

				for ns, count := range tC.expect {
					j, _, err := c.Jobs().List(&api.QueryOptions{Namespace: ns})
					must.NoError(t, err)
					must.Eq(t, count, len(j), must.Sprintf(
						"Expected %v job(s) in %q namespace; found %v", count, ns, len(j)))
				}
			})
		})
	}
}

func TestCLI_V1_CLIFlag_Token(t *testing.T) {
	ct.HTTPTestWithACLParallel(t, ct.WithDefaultConfig(), func(srv *agent.TestAgent) {
		c, err := ct.NewTestClient(srv)
		must.NoError(t, err)

		ct.MakeTestNamespaces(t, c)

		result := runTestPackV1Cmd(t, srv, []string{
			"run",
			getTestPackV1Path(t, testPack),
			"--token=bad00000-bad0-bad0-bad0-badbadbadbad",
		})

		expectNoStdErrOutput(t, result)
		must.StrContains(t, result.cmdOut.String(), "Unexpected response code: 403 (ACL token not found)", must.Sprintf("Expected token not found error, received %q", result.cmdOut.String()))

		result = runTestPackV1Cmd(t, srv, []string{
			"run",
			getTestPackV1Path(t, testPack),
			"--token=" + srv.Config.Client.Meta["token"],
		})

		expectNoStdErrOutput(t, result)
		must.StrContains(t, result.cmdOut.String(), "Pack successfully deployed", must.Sprintf("Expected success message, received %q", result.cmdOut.String()))
	})
}

func TestCLI_V1_EnvConfig_Token(t *testing.T) {
	ct.HTTPTestWithACL(t, ct.WithDefaultConfig(), func(srv *agent.TestAgent) {
		// Garbage token - Should fail
		t.Setenv("NOMAD_TOKEN", badACLToken)

		result := runTestPackV1Cmd(t, srv, []string{
			"run",
			getTestPackV1Path(t, testPack),
		})
		expectNoStdErrOutput(t, result)
		must.StrContains(t, result.cmdOut.String(), "Unexpected response code: 403 (ACL token not found)", must.Sprintf("Expected token not found error, received %q", result.cmdOut.String()))

		// Good token - Should run
		t.Setenv("NOMAD_TOKEN", srv.Config.Client.Meta["token"])
		result = runTestPackV1Cmd(t, srv, []string{
			"run",
			getTestPackV1Path(t, testPack),
		})

		expectNoStdErrOutput(t, result)
		must.StrContains(t, result.cmdOut.String(), "Pack successfully deployed", must.Sprintf("Expected success message, received %q", result.cmdOut.String()))
	})
}

// This test can't benefit from parallelism since it mutates the environment
func TestCLI_V1_EnvConfig_Namespace(t *testing.T) {
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
				result := runTestPackV1Cmd(t, srv, append([]string{
					"run",
					getTestPackV1Path(t, testPack),
				},
					tC.args...),
				)
				expectNoStdErrOutput(t, result)
				must.StrContains(t, result.cmdOut.String(), "Pack successfully deployed", must.Sprintf("Expected success message, received %q", result.cmdOut.String()))

				for ns, count := range tC.expect {
					j, _, err := c.Jobs().List(&api.QueryOptions{Namespace: ns})
					must.NoError(t, err)
					must.Eq(t, count, len(j), must.Sprintf(
						"Expected %v job(s) in %q namespace; found %v", count, ns, len(j)))
				}
			})
		})
	}
}

func TLSConfigFromTestServer(srv *agent.TestAgent) []string {
	srv.T.Helper()
	if srv.Config.TLSConfig == nil {
		return []string{}
	}
	return []string{
		"--client-cert", srv.Config.TLSConfig.CertFile,
		"--client-key", srv.Config.TLSConfig.KeyFile,
		"--ca-cert", srv.Config.TLSConfig.CAFile,
	}
}

func runTestPackV1Cmd(t *testing.T, srv *agent.TestAgent, args []string) PackCommandResult {
	t.Helper()
	args = append(args, AddressFromTestServer(srv)...)
	return runPackV1Cmd(t, args)
}

func runPackV1Cmd(t *testing.T, args []string) PackCommandResult {
	t.Helper()
	args = append(args, "--parser-v1")

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

	must.Eq(t, "", cmdErr.String(), must.Sprintf("cmdErr should be empty, but was %q", cmdErr.String()))

	return PackCommandResult{
		exitCode: exitCode,
		cmdOut:   cmdOut,
		cmdErr:   cmdErr,
	}
}

// getTestPackPath returns the full path to a pack in the test fixtures folder.
func getTestPackV1Path(t *testing.T, packname string) string {
	t.Helper()
	return path.Join(getTestPackRegistryV1Path(t), "packs", packname)
}

// getTestPackRegistryPath returns the full path to a registry in the test
// fixtures folder.
func getTestPackRegistryV1Path(t *testing.T) string {
	t.Helper()
	return path.Join(testfixture.AbsPath(t, "v1/test_registry"))
}

// createTestRegistries creates two registries: first one has "latest" ref,
// second one has testRef ref. It returns registry objects, and a string that
// points to the root where the two refs are on the filesystem.
func createV1TestRegistries(t *testing.T) (*cache.Registry, *cache.Registry, string) {
	t.Helper()

	// Fake a clone
	registryName := fmt.Sprintf("test-%v", time.Now().UnixMilli())

	regDir := path.Join(cache.DefaultCachePath(), registryName)
	err := filesystem.MaybeCreateDestinationDir(regDir)
	must.NoError(t, err)

	for _, r := range []string{"latest", testRef} {
		must.NoError(t, filesystem.CopyDir(
			getTestPackV1Path(t, testPack),
			path.Join(regDir, r, testPack+"@"+r),
			false,
			logging.Default(),
		))
	}

	// create output registries and metadata.json files
	latestReg := &cache.Registry{
		Name:     registryName,
		Source:   "github.com/hashicorp/nomad-pack-test-registry",
		LocalRef: testRef,
		Ref:      "latest",
	}
	latestMetaPath := path.Join(regDir, "latest", "metadata.json")
	b, _ := json.Marshal(latestReg)
	must.NoError(t, os.WriteFile(latestMetaPath, b, 0644))

	testRefReg := &cache.Registry{
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
