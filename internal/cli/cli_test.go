package cli

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path"
	"testing"

	"github.com/hashicorp/nomad-pack/internal/pkg/cache"
	"github.com/hashicorp/nomad-pack/internal/pkg/logging"
	"github.com/stretchr/testify/require"
)

// These tests currently require the nomad agent -dev to be running.

// TODO: Test job run with diffs
// TODO: Test job run plan with diffs
// TODO: Test multi-region plan without conflicts
// TODO: Test multi-region plan with conflicts
// TODO: Test outputPlannedJob that returns non-zero exit code

func TestVersion(t *testing.T) {
	testInit(t)
	exitCode := Main([]string{"nomad-pack", "-v"})
	require.Equal(t, 0, exitCode)
	reset()
}

func TestJobRun(t *testing.T) {
	testInit(t)

	defer func() {
		clearJob(t, &cache.PackConfig{Name: testPack})
	}()

	exitCode := runCmd().Run([]string{testPack})
	require.Equal(t, 0, exitCode)
}

// Confirm that another pack with the same job names but a different deployment name fails
func TestJobRunConflictingDeployment(t *testing.T) {
	testInit(t)

	// Register the initial pack
	exitCode := runCmd().Run([]string{testPack})
	defer func() {
		clearJob(t, &cache.PackConfig{Name: testPack})
	}()

	// deploymentName := runCmd.deploymentName
	require.Equal(t, 0, exitCode)

	exitCode = runCmd().Run([]string{testPack, "--name=with-name"})
	require.Equal(t, 1, exitCode)

	// Confirm that it's still possible to update the existing pack
	exitCode = runCmd().Run([]string{testPack})
	require.Equal(t, 0, exitCode)
}

// Check for conflict with non-pack job i.e. no meta
func TestJobRunConflictingNonPackJob(t *testing.T) {
	testInit(t)

	// Register non pack job
	nomadExec(t, "run", "../fixtures/simple.nomad")

	defer func() {
		clearJob(t, &cache.PackConfig{Name: testPack})
	}()

	// Now try to register the pack
	exitCode := runCmd().Run([]string{testPack})
	require.Equal(t, 1, exitCode)
}

// Check for conflict with job that has meta
func TestJobRunConflictingJobWithMeta(t *testing.T) {
	testInit(t)

	defer func() {
		clearJob(t, &cache.PackConfig{Name: testPack})
	}()

	nomadExec(t, "run", "../fixtures/simple-with-meta.nomad")

	// Now try to register
	exitCode := runCmd().Run([]string{testPack})
	require.Equal(t, 1, exitCode)
}

func TestJobRunFails(t *testing.T) {
	testInit(t)

	exitCode := runCmd().Run([]string{"fake-job"})
	require.Equal(t, 1, exitCode)

	reset()
}

func TestJobPlan(t *testing.T) {
	testInit(t)

	exitCode := planCmd().Run([]string{testPack})
	// Should return 1 indicating an allocation will be placed
	require.Equal(t, 1, exitCode)

	reset()
}

// Confirm that another pack with the same job names but a different deployment name fails
func TestJobPlanConflictingDeployment(t *testing.T) {
	testInit(t)

	defer func() {
		clearJob(t, &cache.PackConfig{Name: testPack})
	}()

	// Register the initial pack
	exitCode := runCmd().Run([]string{testPack})
	require.Equal(t, 0, exitCode)

	exitCode = runCmd().Run([]string{testPack, testRefFlag})
	require.Equal(t, 1, exitCode)

	reset()
}

// Check for conflict with non-pack job i.e. no meta
func TestJobPlanConflictingNonPackJob(t *testing.T) {
	testInit(t)
	defer func() {
		clearJob(t, &cache.PackConfig{Name: testPack})
	}()

	// Register non pack job
	nomadExec(t, "run", "../fixtures/simple.nomad")

	// Now try to plan the pack
	exitCode := planCmd().Run([]string{testPack})
	require.Equal(t, 255, exitCode)

	reset()
}

func TestJobStop(t *testing.T) {
	testInit(t)

	exitCode := runCmd().Run([]string{testPack})
	require.Equal(t, 0, exitCode)

	exitCode = stopCmd().Run([]string{testPack, "--purge=true"})
	require.Equal(t, 0, exitCode)

	reset()
}

func TestJobStopConflicts(t *testing.T) {
	testInit(t)

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
			defer func() {
				// Purge job. Use nomad command since it'll work for all jobs
				nomadExec(t, "stop", "-purge", c.jobName)
			}()

			// Create job
			if c.nonPackJob {
				nomadExec(t, "run", "../fixtures/simple.nomad")
			} else {
				deploymentName := fmt.Sprintf("--name=%s", c.deploymentName)
				varJobName := fmt.Sprintf("--var=job_name=%s", c.jobName)
				exitCode := runCmd().Run([]string{c.packName, deploymentName, varJobName})
				require.Equal(t, 0, exitCode)
			}

			// Try to stop job
			exitCode := stopCmd().Run([]string{c.packName})
			require.Equal(t, 1, exitCode)
		})
	}

	reset()
}

// Destroy is just an alias for stop --purge so we only need to
// test that specific functionality
func TestJobDestroy(t *testing.T) {
	testInit(t)

	exitCode := runCmd().Run([]string{testPack})
	require.Equal(t, 0, exitCode)

	exitCode = destroyCmd().Run([]string{testPack})
	require.Equal(t, 0, exitCode)

	// Assert job no longer queryable
	nomadExpectErr(t, "status", testPack)

	reset()
}

// Test that destroy properly uses var overrides to target the job
func TestJobDestroyWithOverrides(t *testing.T) {
	testInit(t)

	// 	TODO: Table Testing
	// Create multiple jobs in the same pack deployment

	jobNames := []string{"foo", "bar"}
	for _, j := range jobNames {
		exitCode := runCmd().Run([]string{testPack, `--var=job_name=` + j})
		require.Equal(t, 0, exitCode)
	}

	// Stop nonexistent job
	exitCode := destroyCmd().Run([]string{testPack, "--var=job_name=baz"})
	require.Equal(t, 1, exitCode)

	// Stop job with var override
	exitCode = destroyCmd().Run([]string{testPack, "--var=job_name=foo"})
	require.Equal(t, 0, exitCode)

	// Assert job "bar" still exists
	nomadExec(t, "status", "bar")

	// Stop job with no overrides passed
	exitCode = destroyCmd().Run([]string{testPack})
	require.Equal(t, 0, exitCode)

	// Assert job bar is gone
	nomadExpectErr(t, "status", "bar")

	reset()
}

func TestFlagProvidedButNotDefined(t *testing.T) {
	testInit(t)

	// There is no job flag. This tests that adding an unspecified flag does not
	// create an invalid memory address error
	// Posix case
	exitCode := runCmd().Run([]string{"nginx", "--job=provided-but-not-defined"})
	require.Equal(t, 1, exitCode)

	// std go case
	exitCode = runCmd().Run([]string{"-job=provided-but-not-defined", "nginx"})
	require.Equal(t, 1, exitCode)

	reset()
}

func TestStatus(t *testing.T) {
	testInit(t)
	defer func() {
		clearJob(t, &cache.PackConfig{Name: testPack})
	}()

	exitCode := runCmd().Run([]string{testPack})
	require.Equal(t, 0, exitCode)

	cases := []struct {
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

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			exitCode = statusCmd().Run(c.args)
			require.Equal(t, 0, exitCode)
		})
	}

	reset()
}

func TestStatusFails(t *testing.T) {
	testInit(t)

	statusCmd := &StatusCommand{baseCommand: baseCmd()}

	exitCode := statusCmd.Run([]string{"--name=foo"})
	require.Equal(t, 1, exitCode)

	reset()
}

var nomadAddr string
var testPack = "simple_service"
var testRefFlag = "--ref=48eb7d5"

// reduce boilerplate copy pasta with a factory method.
func baseCmd() *baseCommand {
	return &baseCommand{Ctx: context.Background()}
}

func planCmd() *PlanCommand {
	return &PlanCommand{baseCommand: baseCmd()}
}

func runCmd() *RunCommand {
	return &RunCommand{baseCommand: baseCmd()}
}

func destroyCmd() *DestroyCommand {
	return &DestroyCommand{&StopCommand{baseCommand: baseCmd()}}
}

func statusCmd() *StatusCommand {
	return &StatusCommand{baseCommand: baseCmd()}
}

func stopCmd() *StopCommand {
	return &StopCommand{baseCommand: baseCmd()}
}

// Save the current machine's NOMAD_ADDR so that tests can reset developer.
// environment. Added to every test to allow one of ad hoc testing.
func testInit(t *testing.T) {
	nomadAddr = os.Getenv("NOMAD_ADDR")
	_ = os.Setenv("NOMAD_ADDR", "http://127.0.0.1:4646")

	// Make sure the alternate ref registry is loaded to the environment.
	_, err := os.Stat(path.Join(cache.DefaultCachePath(), cache.DefaultRegistryName, "simple_service@48eb7d5"))
	if err != nil && os.IsNotExist(err) {
		var c *cache.Cache
		c, err = cache.NewCache(&cache.CacheConfig{
			Path:   cache.DefaultCachePath(),
			Eager:  false,
			Logger: logging.Default(),
		})
		require.NoError(t, err)
		_, err = c.Add(&cache.AddOpts{
			RegistryName: cache.DefaultRegistryName,
			Source:       cache.DefaultRegistrySource,
			Ref:          "48eb7d5",
		})
		require.NoError(t, err)
	}
}

// Reset NOMAD_ADDR after test. Added to every test to allow one of ad hoc testing.
func reset() {
	_ = os.Setenv("NOMAD_ADDR", nomadAddr)
}

// deferable func to ensure tests don't leave nomad dev agent with running job
// that can affect other tests.
func clearJob(t *testing.T, cfg *cache.PackConfig) {
	nomadExec(t, "job", "stop", "-purge", cfg.Name)
	reset()
}

func nomadExec(t *testing.T, args ...string) {
	nomadPath, err := exec.LookPath("nomad")
	require.NoError(t, err)
	nomadCmd := exec.Command(nomadPath, args...)
	err = nomadCmd.Run()
	require.NoError(t, err)
}

func nomadExpectErr(t *testing.T, args ...string) {
	nomadPath, err := exec.LookPath("nomad")
	require.NoError(t, err)
	nomadCmd := exec.Command(nomadPath, args...)
	err = nomadCmd.Run()
	require.Error(t, err)
}
