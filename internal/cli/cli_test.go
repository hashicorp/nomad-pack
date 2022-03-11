package cli

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"

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
	testPack     = "simple_raw_exec"
	testRef      = "48eb7d5"
	testRefFlag  = "--ref=" + testRef
	testLogLevel = "ERROR"
)

func TestCreateTestRegistry(t *testing.T) {
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

func TestVersion(t *testing.T) {
	t.Parallel()
	// This test doesn't require a Nomad cluster.
	exitCode := Main([]string{"nomad-pack", "-v"})
	require.Equal(t, 0, exitCode)
}

func TestJobRun(t *testing.T) {
	httpTest(t, WithDefaultConfig(), func(s *agent.TestAgent) {
		expectGoodPackDeploy(t, runTestPackCmd(t, s, []string{"run", getTestPackPath(testPack)}))
	})
}

// Confirm that another pack with the same job names but a different deployment name fails
func TestJobRunConflictingDeployment(t *testing.T) {
	httpTest(t, WithDefaultConfig(), func(s *agent.TestAgent) {
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
func TestJobRunConflictingNonPackJob(t *testing.T) {
	httpTest(t, WithDefaultConfig(), func(s *agent.TestAgent) {
		// Register non pack job
		nomadExpectNoErr(t, s, []string{"run"}, []string{"-detach"}, getTestNomadJobPath(testPack))

		// Now try to register the pack
		result := runTestPackCmd(t, s, []string{"run", getTestPackPath(testPack)})

		require.Equal(t, 1, result.exitCode)
		require.Empty(t, result.cmdErr.String(), "cmdErr should be empty, but was %q", result.cmdErr.String())
		require.Contains(t, result.cmdOut.String(), job.ErrExistsNonPack{JobID: testPack}.Error())
	})
}

// Check for conflict with job that has meta
func TestJobRunConflictingJobWithMeta(t *testing.T) {
	httpTest(t, WithDefaultConfig(), func(s *agent.TestAgent) {
		// Register non pack job
		nomadExpectNoErr(t, s, []string{"run"}, []string{"-detach"}, getTestNomadJobPath("simple_raw_exec_with_meta"))

		// Now try to register the pack
		result := runTestPackCmd(t, s, []string{"run", getTestPackPath(testPack)})
		require.Equal(t, 1, result.exitCode)
		require.Empty(t, result.cmdErr.String(), "cmdErr should be empty, but was %q", result.cmdErr.String())
		require.Contains(t, result.cmdOut.String(), job.ErrExistsNonPack{JobID: testPack}.Error())
	})
}

func TestJobRunFails(t *testing.T) {
	t.Parallel()
	// This test doesn't require a Nomad cluster.
	result := runPackCmd(t, []string{"run", "fake-job"})

	require.Equal(t, 1, result.exitCode)
	require.Empty(t, result.cmdErr.String(), "cmdErr should be empty, but was %q", result.cmdErr.String())
	require.Contains(t, result.cmdOut.String(), "Failed To Find Pack")
}

func TestJobPlan(t *testing.T) {
	httpTest(t, WithDefaultConfig(), func(s *agent.TestAgent) {
		expectGoodPackPlan(t, runTestPackCmd(t, s, []string{"plan", getTestPackPath(testPack)}))
	})
}

func TestJobPlan_BadJob(t *testing.T) {
	httpTest(t, WithDefaultConfig(), func(s *agent.TestAgent) {
		result := runTestPackCmd(t, s, []string{"plan", "fake-job"})

		require.Empty(t, result.cmdErr.String(), "cmdErr should be empty, but was %q", result.cmdErr.String())
		require.Contains(t, result.cmdOut.String(), "Failed To Find Pack")
		require.Equal(t, 255, result.exitCode) // Should return 255 indicating an error
	})
}

// Confirm that another pack with the same job names but a different deployment name fails
func TestJobPlanConflictingDeployment(t *testing.T) {
	httpTest(t, WithDefaultConfig(), func(s *agent.TestAgent) {
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
func TestJobPlanConflictingNonPackJob(t *testing.T) {
	httpTest(t, WithDefaultConfig(), func(s *agent.TestAgent) {
		// Register non pack job
		nomadExpectNoErr(t, s, []string{"run"}, []string{"-detach"}, getTestNomadJobPath(testPack))

		// Now try to register the pack
		result := runTestPackCmd(t, s, []string{"plan", getTestPackPath(testPack)})
		require.Empty(t, result.cmdErr.String(), "cmdErr should be empty, but was %q", result.cmdErr.String())
		require.Contains(t, result.cmdOut.String(), job.ErrExistsNonPack{JobID: testPack}.Error())
		require.Equal(t, 255, result.exitCode) // Should return 255 indicating an error
	})
}

func TestJobStop(t *testing.T) {
	httpTest(t, WithDefaultConfig(), func(s *agent.TestAgent) {
		expectGoodPackDeploy(t, runTestPackCmd(t, s, []string{"run", getTestPackPath(testPack)}))

		result := runTestPackCmd(t, s, []string{"stop", getTestPackPath(testPack), "--purge=true"})
		require.Empty(t, result.cmdErr.String(), "cmdErr should be empty, but was %q", result.cmdErr.String())
		require.Contains(t, result.cmdOut.String(), `Pack "`+testPack+`" destroyed`)
		require.Equal(t, 0, result.exitCode)
	})
}

func TestJobStopConflicts(t *testing.T) {
	httpTest(t, WithDefaultConfig(), func(s *agent.TestAgent) {

		cases := []struct {
			name           string
			nonPackJob     bool
			packName       string
			deploymentName string
			jobName        string
		}{
			// Give these each different job names so there's no conflicts
			// between the different tests cases when running
			{
				name:           "non-pack-job",
				nonPackJob:     true,
				packName:       testPack,
				deploymentName: "",
				jobName:        testPack,
			},
			{
				name:           "same-pack-diff-deploy",
				nonPackJob:     false,
				packName:       testPack,
				deploymentName: "foo",
				jobName:        "job2",
			},
		}

		for _, c := range cases {
			t.Run(c.name, func(t *testing.T) {
				defer nomadExec(t, s, []string{"stop"}, []string{"-purge"}, c.jobName)
				// Create job
				if c.nonPackJob {
					nomadExpectNoErr(t, s, []string{"run"}, []string{"-detach"}, getTestNomadJobPath(testPack))
				} else {
					deploymentName := fmt.Sprintf("--name=%s", c.deploymentName)
					varJobName := fmt.Sprintf("--var=job_name=%s", c.jobName)
					expectGoodPackDeploy(t, runTestPackCmd(t, s, []string{"run", getTestPackPath(testPack), deploymentName, varJobName}))
				}

				// Try to stop job
				result := runTestPackCmd(t, s, []string{"stop", c.packName})
				require.Equal(t, 1, result.exitCode)
			})
		}
	})
}

// Destroy is just an alias for stop --purge so we only need to
// test that specific functionality
func TestJobDestroy(t *testing.T) {
	httpTest(t, WithDefaultConfig(), func(s *agent.TestAgent) {
		expectGoodPackDeploy(t, runTestPackCmd(t, s, []string{"run", getTestPackPath(testPack)}))

		result := runTestPackCmd(t, s, []string{"destroy", getTestPackPath(testPack)})
		require.Contains(t, result.cmdOut.String(), `Pack "`+testPack+`" destroyed`)
		require.Equal(t, 0, result.exitCode)

		// Assert job no longer queryable
		nomadExpectErr(t, s, []string{"status"}, []string{}, testPack)
	})
}

// Test that destroy properly uses var overrides to target the job
func TestJobDestroyWithOverrides(t *testing.T) {
	httpTest(t, WithDefaultConfig(), func(s *agent.TestAgent) {
		c, err := NewTestClient(s)
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

func TestFlagProvidedButNotDefined(t *testing.T) {
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

func TestStatus(t *testing.T) {
	httpTest(t, WithDefaultConfig(), func(s *agent.TestAgent) {
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

func TestStatusFails(t *testing.T) {
	httpTest(t, WithDefaultConfig(), func(s *agent.TestAgent) {
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

func TestRenderMyAlias(t *testing.T) {
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

// Test Helpers for calling the Nomad command.
// TODO: Replace with API client calls.
func nomadExpectNoErr(t *testing.T, srv *agent.TestAgent, commands []string, flags []string, args ...string) {
	err := nomadExec(t, srv, commands, flags, args...)
	if err != nil {
		execErr, _ := err.(ErrNomadExec)
		require.NoError(t, err, "stdout: %q \nstderr: %q", execErr.stdout, execErr.stderr)
	}
}

func nomadExpectErr(t *testing.T, srv *agent.TestAgent, commands []string, flags []string, args ...string) {
	err := nomadExec(t, srv, commands, flags, args...)
	require.Error(t, err)
}

func nomadExec(t *testing.T, srv *agent.TestAgent, commands []string, flags []string, args ...string) error {
	t.Helper()

	flags = append(flags, AddressFromTestServer(srv)...)

	cmdArgs := make([]string, 0, len(commands)+len(flags)+len(args))
	cmdArgs = append(cmdArgs, commands...)
	cmdArgs = append(cmdArgs, flags...)
	cmdArgs = append(cmdArgs, args...)

	var outb, errb bytes.Buffer
	nomadPath, err := exec.LookPath("nomad")
	require.NoError(t, err)

	nomadCmd := exec.Command(nomadPath, cmdArgs...)
	nomadCmd.Stdout = &outb
	nomadCmd.Stderr = &errb
	err = nomadCmd.Run()
	if err != nil {
		return &ErrNomadExec{err: err, stdout: outb.String(), stderr: errb.String()}
	}
	return nil
}

// ErrNomadExec is returned when calls to the Nomad CLI do not run as expected.
// stdout and stderr contain the output from the corresponding streams
type ErrNomadExec struct {
	err    error
	stdout string
	stderr string
}

// Error fulfills the error interface and provides a nicely formatted version of
// a ErrNomadExec struct
func (e ErrNomadExec) Error() string {
	return fmt.Sprintf("nomad exec error: %s \n  stdout: \n%s \n  stderr: \n%s", e.err.Error(), e.stdout, e.stderr)
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

// func nomadCleanupJob(t *testing.T, s *agent.TestAgent) {
// 	c, _ := NewTestClient(s)

// 	wo := v1.DefaultWriteOpts()

// 	wCtx, done := context.WithTimeout(wo.Ctx(), 5*time.Second)
// 	resp, wMeta, err := c.Jobs().Delete(wCtx, jobName, purge, false)
// 	done()

// 	qCtx, done := context.WithTimeout(q.Ctx(), 5*time.Second)
// 	resp, wMeta, err := c.Jobs().Delete(wCtx, jobName, purge, false)
// 	done()

// 	require.Nil(t, job)
// }
