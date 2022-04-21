package cli

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"

	ct "github.com/hashicorp/nomad-pack/internal/cli/testhelper"
	"github.com/hashicorp/nomad-pack/internal/pkg/cache"
	flag "github.com/hashicorp/nomad-pack/internal/pkg/flag"
	"github.com/hashicorp/nomad-pack/internal/pkg/helper/filesystem"
	"github.com/hashicorp/nomad-pack/internal/pkg/logging"
	"github.com/hashicorp/nomad-pack/internal/pkg/version"
	"github.com/hashicorp/nomad-pack/internal/runner/job"
	"github.com/hashicorp/nomad-pack/internal/testui"
	"github.com/hashicorp/nomad/command/agent"
	"github.com/mitchellh/cli"
	"github.com/stretchr/testify/require"
)

// TODO: Test job run with diffs
// TODO: Test job run plan with diffs
// TODO: Test multi-region plan without conflicts
// TODO: Test multi-region plan with conflicts
// TODO: Test outputPlannedJob that returns non-zero exit code

const (
	testPack    = "simple_raw_exec"
	testRef     = "48eb7d5"
	testRefFlag = "--ref=" + testRef
	badACLToken = "bad00000-bad0-bad0-bad0-badbadbadbad"
)

func TestCLI_CreateTestRegistry(t *testing.T) {
	// This test is here to help setup the pack registry cache. It needs to be
	// the first one in the file and can not be `Parallel()`
	regName, regPath := createTestRegistry(t)
	defer cleanTestRegistry(t, regPath)
	t.Logf("regName: %v\n", regName)
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
	packRegex := regexp.MustCompile(`(?m)^ +` + testPack + ` +\| ` + testRef + ` +\| 0\.0\.1 +\| ` + regName + ` +\|[^\n]+?$`)
	matches := packRegex.FindAllString(out, -1)
	for i, match := range matches {
		t.Logf("match %v:  %v\n", i, match)
	}
	require.Regexp(t, packRegex, out)
	require.Equal(t, 0, result.exitCode)
}

func TestCLI_Version(t *testing.T) {
	t.Parallel()
	// This test doesn't require a Nomad cluster.
	exitCode := Main([]string{"nomad-pack", "-v"})
	require.Equal(t, 0, exitCode)
}

func TestCLI_JobRun(t *testing.T) {
	ct.HTTPTestParallel(t, ct.WithDefaultConfig(), func(s *agent.TestAgent) {
		expectGoodPackDeploy(t, runTestPackCmd(t, s, []string{"run", getTestPackPath(testPack)}))
	})
}

// Confirm that another pack with the same job names but a different deployment name fails
func TestCLI_JobRunConflictingDeployment(t *testing.T) {
	ct.HTTPTestParallel(t, ct.WithDefaultConfig(), func(s *agent.TestAgent) {
		expectGoodPackDeploy(t, runTestPackCmd(t, s, []string{"run", getTestPackPath(testPack)}))

		result := runTestPackCmd(t, s, []string{"run", getTestPackPath(testPack), "--name=with-name"})
		require.Equal(t, 1, result.exitCode)
		require.Empty(t, result.cmdErr.String(), "cmdErr should be empty, but was %q", result.cmdErr.String())
		require.Contains(t, result.cmdOut.String(), job.ErrExistsInDeployment{JobID: testPack, Deployment: testPack}.Error())

		// Confirm that it's still possible to update the existing pack
		expectGoodPackDeploy(t, runTestPackCmd(t, s, []string{"run", getTestPackPath(testPack)}))
	})
}

// Check for conflict with non-pack job i.e. no meta
func TestCLI_JobRunConflictingNonPackJob(t *testing.T) {
	ct.HTTPTestParallel(t, ct.WithDefaultConfig(), func(s *agent.TestAgent) {
		// Register non pack job
		err := ct.NomadRun(s, getTestNomadJobPath(testPack))
		require.NoError(t, err)

		// Now try to register the pack
		result := runTestPackCmd(t, s, []string{"run", getTestPackPath(testPack)})

		require.Equal(t, 1, result.exitCode)
		require.Empty(t, result.cmdErr.String(), "cmdErr should be empty, but was %q", result.cmdErr.String())
		require.Contains(t, result.cmdOut.String(), job.ErrExistsNonPack{JobID: testPack}.Error())
	})
}

// Check for conflict with job that has meta
func TestCLI_JobRunConflictingJobWithMeta(t *testing.T) {
	ct.HTTPTestParallel(t, ct.WithDefaultConfig(), func(s *agent.TestAgent) {
		// Register non pack job
		err := ct.NomadRun(s, getTestNomadJobPath("simple_raw_exec_with_meta"))
		require.NoError(t, err)

		// Now try to register the pack
		result := runTestPackCmd(t, s, []string{"run", getTestPackPath(testPack)})
		require.Equal(t, 1, result.exitCode)
		require.Empty(t, result.cmdErr.String(), "cmdErr should be empty, but was %q", result.cmdErr.String())
		require.Contains(t, result.cmdOut.String(), job.ErrExistsNonPack{JobID: testPack}.Error())
	})
}

func TestCLI_JobRunFails(t *testing.T) {
	t.Parallel()
	// This test doesn't require a Nomad cluster.
	result := runPackCmd(t, []string{"run", "fake-job"})

	require.Equal(t, 1, result.exitCode)
	require.Empty(t, result.cmdErr.String(), "cmdErr should be empty, but was %q", result.cmdErr.String())
	require.Contains(t, result.cmdOut.String(), "Failed To Find Pack")
}

func TestCLI_JobPlan(t *testing.T) {
	ct.HTTPTestParallel(t, ct.WithDefaultConfig(), func(s *agent.TestAgent) {
		expectGoodPackPlan(t, runTestPackCmd(t, s, []string{"plan", getTestPackPath(testPack)}))
	})
}

func TestCLI_JobPlan_BadJob(t *testing.T) {
	ct.HTTPTestParallel(t, ct.WithDefaultConfig(), func(s *agent.TestAgent) {
		result := runTestPackCmd(t, s, []string{"plan", "fake-job"})

		require.Empty(t, result.cmdErr.String(), "cmdErr should be empty, but was %q", result.cmdErr.String())
		require.Contains(t, result.cmdOut.String(), "Failed To Find Pack")
		require.Equal(t, 255, result.exitCode) // Should return 255 indicating an error
	})
}

// Confirm that another pack with the same job names but a different deployment name fails
func TestCLI_JobPlan_ConflictingDeployment(t *testing.T) {
	ct.HTTPTestParallel(t, ct.WithDefaultConfig(), func(s *agent.TestAgent) {
		regName, regPath := createTestRegistry(t)
		defer cleanTestRegistry(t, regPath)

		testRegFlag := "--registry=" + regName
		expectGoodPackDeploy(t, runTestPackCmd(t, s, []string{"run", testPack, testRegFlag}))

		result := runTestPackCmd(t, s, []string{"run", testPack, testRegFlag, testRefFlag})
		require.Empty(t, result.cmdErr.String(), "cmdErr should be empty, but was %q", result.cmdErr.String())
		require.Contains(t, result.cmdOut.String(), job.ErrExistsInDeployment{JobID: testPack, Deployment: testPack + "@latest"}.Error())
		require.Equal(t, 1, result.exitCode)
	})
}

// Check for conflict with non-pack job i.e. no meta
func TestCLI_JobPlan_ConflictingNonPackJob(t *testing.T) {
	ct.HTTPTestParallel(t, ct.WithDefaultConfig(), func(s *agent.TestAgent) {
		// Register non pack job
		err := ct.NomadRun(s, getTestNomadJobPath(testPack))
		require.NoError(t, err)

		// Now try to register the pack
		result := runTestPackCmd(t, s, []string{"plan", getTestPackPath(testPack)})
		require.Empty(t, result.cmdErr.String(), "cmdErr should be empty, but was %q", result.cmdErr.String())
		require.Contains(t, result.cmdOut.String(), job.ErrExistsNonPack{JobID: testPack}.Error())
		require.Equal(t, 255, result.exitCode) // Should return 255 indicating an error
	})
}

func TestCLI_PackPlan_OverrideExitCodes(t *testing.T) {
	ct.HTTPTest(t, ct.WithDefaultConfig(), func(s *agent.TestAgent) {
		// Plan against empty - should be makes-changes
		result := runTestPackCmd(t, s, []string{
			"plan",
			"--exit-code-makes-changes=91",
			"--exit-code-no-changes=90",
			"--exit-code-error=92",
			getTestPackPath(testPack),
		})
		require.Empty(t, result.cmdErr.String(), "cmdErr should be empty, but was %q", result.cmdErr.String())
		require.Contains(t, result.cmdOut.String(), "Plan succeeded\n")
		require.Equal(t, 91, result.exitCode) // Should return exit-code-makes-changes

		// Register non pack job
		err := ct.NomadRun(s, getTestNomadJobPath(testPack))
		require.NoError(t, err)

		// Now try to register the pack, should make error
		result = runTestPackCmd(t, s, []string{
			"plan",
			"--exit-code-makes-changes=91",
			"--exit-code-no-changes=90",
			"--exit-code-error=92",
			getTestPackPath(testPack),
		})
		require.Empty(t, result.cmdErr.String(), "cmdErr should be empty, but was %q", result.cmdErr.String())
		require.Contains(t, result.cmdOut.String(), job.ErrExistsNonPack{JobID: testPack}.Error())
		require.Equal(t, 92, result.exitCode) // Should exit-code-error

		err = ct.NomadPurge(s, testPack)
		require.NoError(t, err)

		isGone := func() bool {
			_, err = ct.NomadJobStatus(s, testPack)
			if err != nil {
				return err.Error() == "job not found"
			}
			return false
		}
		require.Eventually(t, isGone, 10*time.Second, 500*time.Millisecond, "test job failed to purge")

		result = runTestPackCmd(t, s, []string{"run", getTestPackPath(testPack)})
		require.Empty(t, result.cmdErr.String(), "cmdErr should be empty, but was %q", result.cmdErr.String())
		require.Contains(t, result.cmdOut.String(), "")
		require.Equal(t, 0, result.exitCode) // Should return 0
		isStarted := func() bool {
			j, err := ct.NomadJobStatus(s, testPack)
			if err != nil {
				return false
			}
			return j.GetStatus() == "running"
		}
		require.Eventually(t, isStarted, 30*time.Second, 500*time.Millisecond, "test job failed to start")

		// Plan against deployed - should be no-changes
		result = runTestPackCmd(t, s, []string{
			"plan",
			"--exit-code-makes-changes=91",
			"--exit-code-no-changes=90",
			"--exit-code-error=92",
			getTestPackPath(testPack),
		})
		require.Empty(t, result.cmdErr.String(), "cmdErr should be empty, but was %q", result.cmdErr.String())
		require.Contains(t, result.cmdOut.String(), "Plan succeeded\n")
		require.Equal(t, 90, result.exitCode) // Should return exit-code-no-changes
	})
}

func TestCLI_PackStop(t *testing.T) {
	ct.HTTPTestParallel(t, ct.WithDefaultConfig(), func(s *agent.TestAgent) {
		expectGoodPackDeploy(t, runTestPackCmd(t, s, []string{"run", getTestPackPath(testPack)}))

		result := runTestPackCmd(t, s, []string{"stop", getTestPackPath(testPack), "--purge=true"})
		require.Empty(t, result.cmdErr.String(), "cmdErr should be empty, but was %q", result.cmdErr.String())
		require.Contains(t, result.cmdOut.String(), `Pack "`+testPack+`" destroyed`)
		require.Equal(t, 0, result.exitCode)
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
		require.NoError(t, err)
		for _, tC := range testCases {
			t.Run(tC.desc, func(t *testing.T) {
				defer ct.NomadCleanup(s)

				if tC.namespace != "" {
					ct.MakeTestNamespaces(t, client)
				}

				// Create job
				if tC.nonPackJob {
					err = ct.NomadRun(s, getTestNomadJobPath(testPack))
					require.NoError(t, err)
				} else {
					deploymentName := fmt.Sprintf("--name=%s", tC.deploymentName)
					varJobName := fmt.Sprintf("--var=job_name=%s", tC.jobName)
					if tC.namespace != "" {
						namespaceFlag := fmt.Sprintf("--namespace=%s", tC.namespace)
						expectGoodPackDeploy(t, runTestPackCmd(t, s, []string{"run", getTestPackPath(testPack), deploymentName, varJobName, namespaceFlag}))
					} else {
						expectGoodPackDeploy(t, runTestPackCmd(t, s, []string{"run", getTestPackPath(testPack), deploymentName, varJobName}))
					}
				}

				// Try to stop job
				result := runTestPackCmd(t, s, []string{"stop", tC.packName})
				require.Equal(t, 1, result.exitCode)
			})
		}
	})
}

// Destroy is just an alias for stop --purge so we only need to
// test that specific functionality
func TestCLI_PackDestroy(t *testing.T) {
	ct.HTTPTestParallel(t, ct.WithDefaultConfig(), func(s *agent.TestAgent) {
		expectGoodPackDeploy(t, runTestPackCmd(t, s, []string{"run", getTestPackPath(testPack)}))

		result := runTestPackCmd(t, s, []string{"destroy", getTestPackPath(testPack)})
		require.Contains(t, result.cmdOut.String(), `Pack "`+testPack+`" destroyed`)
		require.Equal(t, 0, result.exitCode)

		// Assert job no longer queryable
		c, err := ct.NewTestClient(s)
		require.NoError(t, err)

		r, _, err := c.Jobs().GetJob(c.QueryOpts().Ctx(), testPack)
		require.Nil(t, r)
		require.EqualError(t, err, "job not found")
	})
}

// Test that destroy properly uses var overrides to target the job
func TestCLI_PackDestroy_WithOverrides(t *testing.T) {
	ct.HTTPTestParallel(t, ct.WithDefaultConfig(), func(s *agent.TestAgent) {
		c, err := ct.NewTestClient(s)
		require.NoError(t, err)
		// Because this test uses ref, it requires a populated pack cache.
		regName, regPath := createTestRegistry(t)
		defer cleanTestRegistry(t, regPath)

		// 	TODO: Table Testing
		// Create multiple jobs in the same pack deployment

		jobNames := []string{"foo", "bar"}
		for _, j := range jobNames {
			expectGoodPackDeploy(t, runTestPackCmd(
				t,
				s,
				[]string{
					"run",
					testPack,
					"--var=job_name=" + j,
					"--registry=" + regName,
				}))
		}

		// Stop nonexistent job
		result := runTestPackCmd(t, s, []string{"destroy", testPack, "--var=job_name=baz", "--registry=" + regName})
		require.Equal(t, 1, result.exitCode, "expected exitcode 1; got %v\ncmdOut:%v", result.exitCode, result.cmdOut.String())

		// Stop job with var override
		result = runTestPackCmd(t, s, []string{"destroy", testPack, "--var=job_name=foo", "--registry=" + regName})
		require.Equal(t, 0, result.exitCode, "expected exitcode 0; got %v\ncmdOut:%v", result.exitCode, result.cmdOut.String())

		// Assert job "bar" still exists
		tCtx, done := context.WithTimeout(context.TODO(), 5*time.Second)
		job, _, err := c.Jobs().GetJob(tCtx, "bar")
		done()
		require.NoError(t, err)
		require.NotNil(t, job)

		// Stop job with no overrides passed
		result = runTestPackCmd(t, s, []string{"destroy", testPack, "--registry=" + regName})
		require.Equal(t, 0, result.exitCode, "expected exitcode 0; got %v\ncmdOut:%v", result.exitCode, result.cmdOut.String())

		// Assert job bar is gone
		tCtx, done = context.WithTimeout(context.TODO(), 5*time.Second)
		job, _, err = c.Jobs().GetJob(tCtx, "bar")
		done()
		require.Error(t, err)
		require.Equal(t, "job not found", err.Error())
		require.Nil(t, job)
	})
}

func TestCLI_CLIFlag_NotDefined(t *testing.T) {
	t.Parallel() // nomad not required

	// There is no job flag. This tests that adding an unspecified flag does not
	// create an invalid memory address error
	// Posix case
	result := runPackCmd(t, []string{"run", "nginx", "--job=provided-but-not-defined"})
	require.Equal(t, 1, result.exitCode)

	// std go case
	result = runPackCmd(t, []string{"run", "-job=provided-but-not-defined", "nginx"})
	require.Equal(t, 1, result.exitCode)
}

func TestCLI_PackStatus(t *testing.T) {
	ct.HTTPTestParallel(t, ct.WithDefaultConfig(), func(s *agent.TestAgent) {
		result := runTestPackCmd(t, s, []string{"run", getTestPackPath(testPack)})
		require.Equal(t, 0, result.exitCode)

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
				require.Equal(t, 0, result.exitCode)
				require.Contains(t, result.cmdOut.String(), "simple_raw_exec | "+cache.DevRegistryName+" ")
			})
		}
	})
}

func TestCLI_PackStatus_Fails(t *testing.T) {
	ct.HTTPTestParallel(t, ct.WithDefaultConfig(), func(s *agent.TestAgent) {
		// test for status on missing pack
		result := runTestPackCmd(t, s, []string{"status", getTestPackPath(testPack)})
		require.Empty(t, result.cmdErr.String(), "cmdErr should be empty, but was %q", result.cmdErr.String())
		require.Contains(t, result.cmdOut.String(), "no jobs found for pack \""+getTestPackPath(testPack)+"\"")
		// FIXME: Should this have a non-success exit-code?
		require.Equal(t, 0, result.exitCode)

		// test flag validation for name flag without pack
		result = runTestPackCmd(t, s, []string{"status", "--name=foo"})
		require.Equal(t, 1, result.exitCode)
		require.Contains(t, result.cmdOut.String(), "--name can only be used if pack name is provided")
	})
}

func TestCLI_PackRender_MyAlias(t *testing.T) {
	t.Parallel()
	// This test has to do some extra shenanigans because dependent pack template
	// output is not guaranteed to be ordered. This requires that the test handle
	// either order.
	expected := []string{
		"child1/child1.nomad=child1",
		"child2/child2.nomad=child2",
		"deps_test/deps_test.nomad=deps_test",
	}

	result := runPackCmd(t, []string{
		"render",
		getTestPackPath("my_alias_test"),
	})
	require.Empty(t, result.cmdErr.String(), "cmdErr should be empty, but was %q", result.cmdErr.String())

	// Performing a little clever string manipulation on the render output to
	// prepare it for splitting into a slice of string enables us to use
	// require.ElementsMatch to validate goodness.
	outStr := strings.TrimSpace(result.cmdOut.String())
	outStr = strings.ReplaceAll(outStr, ":\n\n", "=")
	elems := strings.Split(outStr, "\n")

	require.ElementsMatch(t, expected, elems)
}

func TestCLI_CLIFlag_Namespace(t *testing.T) {
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
				require.NoError(t, err)

				ct.MakeTestNamespaces(t, c)

				result := runTestPackCmd(t, srv, append([]string{
					"run",
					getTestPackPath(testPack),
				},
					tC.args...),
				)
				require.Empty(t, result.cmdErr.String(), "cmdErr should be empty, but was %q", result.cmdErr.String())
				require.Contains(t, result.cmdOut.String(), "Pack successfully deployed", "Expected success message, received %q", result.cmdOut.String())

				for ns, count := range tC.expect {
					tOpt := c.QueryOpts()
					tOpt.Namespace = ns
					tJobs, _, err := c.Jobs().GetJobs(tOpt.Ctx())
					require.NoError(t, err)
					require.Equal(t, count, len(*tJobs), "Expected %v job(s) in %q namespace; found %v", count, ns, len(*tJobs))
				}
			})
		})
	}
}

func TestCLI_CLIFlag_Token(t *testing.T) {
	ct.HTTPTestWithACLParallel(t, ct.WithDefaultConfig(), func(srv *agent.TestAgent) {
		c, err := ct.NewTestClient(srv)
		require.NoError(t, err)

		ct.MakeTestNamespaces(t, c)

		result := runTestPackCmd(t, srv, []string{
			"run",
			getTestPackPath(testPack),
			"--token=bad00000-bad0-bad0-bad0-badbadbadbad",
		})
		require.Empty(t, result.cmdErr.String(), "cmdErr should be empty, but was %q", result.cmdErr.String())
		require.Contains(t, result.cmdOut.String(), "Error: ACL token not found", "Expected token not found error, received %q", result.cmdOut.String())

		result = runTestPackCmd(t, srv, []string{
			"run",
			getTestPackPath(testPack),
			"--token=" + srv.Config.Client.Meta["token"],
		})

		require.Empty(t, result.cmdErr.String(), "cmdErr should be empty, but was %q", result.cmdErr.String())
		require.Contains(t, result.cmdOut.String(), "Pack successfully deployed", "Expected success message, received %q", result.cmdOut.String())
	})
}

func TestCLI_EnvConfig_Token(t *testing.T) {
	ct.HTTPTestWithACL(t, ct.WithDefaultConfig(), func(srv *agent.TestAgent) {
		c, err := ct.NewTestClient(srv)
		require.NoError(t, err)

		// Garbage token - Should fail
		t.Setenv("NOMAD_TOKEN", badACLToken)

		result := runTestPackCmd(t, srv, []string{
			"run",
			getTestPackPath(testPack),
		})
		require.Empty(t, result.cmdErr.String(), "cmdErr should be empty, but was %q", result.cmdErr.String())
		require.Contains(t, result.cmdOut.String(), "Error: ACL token not found", "Expected token not found error, received %q", result.cmdOut.String())

		// Good token - Should run
		t.Setenv("NOMAD_TOKEN", c.QueryOpts().AuthToken)
		result = runTestPackCmd(t, srv, []string{
			"run",
			getTestPackPath(testPack),
		})

		require.Empty(t, result.cmdErr.String(), "cmdErr should be empty, but was %q", result.cmdErr.String())
		require.Contains(t, result.cmdOut.String(), "Pack successfully deployed", "Expected success message, received %q", result.cmdOut.String())
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
				require.NoError(t, err)

				ct.MakeTestNamespaces(t, c)

				// Always set the namespace environment variable
				t.Setenv("NOMAD_NAMESPACE", "env")
				result := runTestPackCmd(t, srv, append([]string{
					"run",
					getTestPackPath(testPack),
				},
					tC.args...),
				)
				require.Empty(t, result.cmdErr.String(), "cmdErr should be empty, but was %q", result.cmdErr.String())
				require.Contains(t, result.cmdOut.String(), "Pack successfully deployed", "Expected success message, received %q", result.cmdOut.String())

				for ns, count := range tC.expect {
					tOpt := c.QueryOpts()
					tOpt.Namespace = ns
					tJobs, _, err := c.Jobs().GetJobs(tOpt.Ctx())
					require.NoError(t, err)
					require.Equal(t, count, len(*tJobs), "Expected %v job(s) in %q namespace; found %v", count, ns, len(*tJobs))
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
	return []string{"--address", srv.HTTPAddr()}
}

func TLSConfigFromTestServer(srv *agent.TestAgent) []string {
	if srv.Config.TLSConfig == nil {
		return []string{}
	}
	return []string{
		"--client-cert", srv.Config.TLSConfig.CertFile,
		"--client-key", srv.Config.TLSConfig.KeyFile,
		"--ca-cert", srv.Config.TLSConfig.CAFile,
	}
}

func runTestPackCmd(t *testing.T, srv *agent.TestAgent, args []string) PackCommandResult {
	args = append(args, AddressFromTestServer(srv)...)
	return runPackCmd(t, args)
}

func runPackCmd(t *testing.T, args []string) PackCommandResult {
	cmdOut := bytes.NewBuffer(make([]byte, 0))
	cmdErr := bytes.NewBuffer(make([]byte, 0))

	// Build our cancellation context
	ctx, closer := WithInterrupt(context.Background())
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

	require.Empty(t, cmdErr.String(), "cmdErr should be empty, but was %q", cmdErr.String())

	return PackCommandResult{
		exitCode: exitCode,
		cmdOut:   cmdOut,
		cmdErr:   cmdErr,
	}
}

// Test Helper functions

// getTestPackPath returns the full path to a pack in the test fixtures folder.
func getTestPackPath(packname string) string {
	return path.Join(getTestPackRegistryPath(), "packs", packname)
}

// getTestPackRegistryPath returns the full path to a registry in the test
// fixtures folder.
func getTestPackRegistryPath() string {
	return path.Join(testFixturePath(), "test_registry")
}

// getTestNomadJobPath returns the full path to a pack in the test
// fixtures/jobspecs folder. The `.nomad` extension will be added
// for you.
func getTestNomadJobPath(job string) string {
	return path.Join(testFixturePath(), "jobspecs", job+".nomad")
}

func testFixturePath() string {
	// This is a function to prevent a massive refactor if this ever needs to be
	// dynamically determined.
	return "../../fixtures/"
}

// expectGoodPackDeploy bundles the test expectations that should be met when
// determining if the pack CLI successfully deployed a pack.
func expectGoodPackDeploy(t *testing.T, r PackCommandResult) {
	require.Empty(t, r.cmdErr.String(), "cmdErr should be empty, but was %q", r.cmdErr.String())
	require.Contains(t, r.cmdOut.String(), "Pack successfully deployed", "Expected success message, received %q", r.cmdOut.String())
	require.Equal(t, 0, r.exitCode)
}

// expectGoodPackPlan bundles the test expectations that should be met when
// determining if the pack CLI successfully planned a pack.
func expectGoodPackPlan(t *testing.T, r PackCommandResult) {
	require.Empty(t, r.cmdErr.String(), "cmdErr should be empty, but was %q", r.cmdErr.String())
	require.Contains(t, r.cmdOut.String(), "Plan succeeded", "Expected success message, received %q", r.cmdOut.String())
	require.Equal(t, 1, r.exitCode) // exitcode 1 means that an allocation will be created
}

func createTestRegistry(t *testing.T) (regName, regDir string) {
	// Fake a clone
	regDir = path.Join(cache.DefaultCachePath(), fmt.Sprintf("test-%v", time.Now().UnixMilli()))
	err := filesystem.MaybeCreateDestinationDir(regDir)

	require.NoError(t, err)
	regName = path.Base(regDir)

	err = filesystem.CopyDir(getTestPackPath(testPack), path.Join(regDir, testPack+"@latest"), logging.Default())
	require.NoError(t, err)
	err = filesystem.CopyDir(getTestPackPath(testPack), path.Join(regDir, testPack+"@"+testRef), logging.Default())
	require.NoError(t, err)

	return
}

func cleanTestRegistry(t *testing.T, regPath string) {
	os.RemoveAll(regPath)
}
