package cli

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"testing"

	"github.com/stretchr/testify/require"
)

// TODO: Test job run with diffs
// TODO: Test job run plan with diffs
// TODO: Test multi-region plan without conflicts
// TODO: Test multi-region plan with conflicts
// TODO: Test outputPlannedJob that returns non-zero exit code

func TestVersion(t *testing.T) {
	nomadAddr := os.Getenv("NOMAD_ADDR")
	os.Setenv("NOMAD_ADDR", "http://127.0.0.1:4646")

	exitCode := Main([]string{"nomad-pack", "-v"})
	require.Equal(t, 0, exitCode)
	os.Setenv("NOMAD_ADDR", nomadAddr)
}

func TestJobRun(t *testing.T) {
	// TODO: Integrate test agent and solve envar dependency
	// this currently requires nomad agent -dev to be running and the
	// NOMAD_ADDR envar to be set.
	nomadAddr := os.Getenv("NOMAD_ADDR")
	os.Setenv("NOMAD_ADDR", "http://127.0.0.1:4646")

	baseCommand := &baseCommand{
		Ctx: context.Background(),
	}

	c := RunCommand{baseCommand: baseCommand}

	exitCode := c.Run([]string{"example"})
	require.Equal(t, 0, exitCode)

	os.Setenv("NOMAD_ADDR", nomadAddr)
	// TODO: add var overrides when fixed
}

// Confirm that another pack with the same job names but a different deployment name fails
func TestJobRunConflictingDeployment(t *testing.T) {
	// TODO: Integrate test agent and solve envar dependency
	// this currently requires nomad agent -dev to be running and the
	// NOMAD_ADDR envar to be set.
	nomadAddr := os.Getenv("NOMAD_ADDR")
	os.Setenv("NOMAD_ADDR", "http://127.0.0.1:4646")

	baseCommand := &baseCommand{
		Ctx: context.Background(),
	}

	runCommand := RunCommand{baseCommand: baseCommand}

	// Register the initial pack
	exitCode := runCommand.Run([]string{"example"})
	require.Equal(t, 0, exitCode)

	exitCode = runCommand.Run([]string{"example", "--name=with-name"})
	require.Equal(t, 1, exitCode)

	// Confirm that it's still possible to update the existing pack
	exitCode = runCommand.Run([]string{"example"})
	require.Equal(t, 0, exitCode)

	// Delete the pack
	stopCommand := StopCommand{baseCommand: baseCommand}
	exitCode = stopCommand.Run([]string{runCommand.deploymentName, "--purge=true"})
	require.Equal(t, 0, exitCode)

	os.Setenv("NOMAD_ADDR", nomadAddr)
}

// Check for conflict with non-pack job i.e. no meta
func TestJobRunConflictingNonPackJob(t *testing.T) {
	// TODO: Integrate test agent and solve envar dependency
	// this currently requires nomad agent -dev to be running and the
	// NOMAD_ADDR envar to be set.
	nomadAddr := os.Getenv("NOMAD_ADDR")
	os.Setenv("NOMAD_ADDR", "http://127.0.0.1:4646")

	// Register non pack job
	nomadPath, err := exec.LookPath("nomad")
	require.NoError(t, err)
	nomadCommand := exec.Command(nomadPath, "run", "../fixtures/example.nomad")
	err = nomadCommand.Run()
	require.NoError(t, err)

	baseCommand := &baseCommand{
		Ctx: context.Background(),
	}

	runCommand := RunCommand{baseCommand: baseCommand}

	// Now try to register the pack
	exitCode := runCommand.Run([]string{"example"})
	require.Equal(t, 1, exitCode)

	// cleanup job
	nomadCommand = exec.Command(nomadPath, "job", "stop", "-purge", "example")
	err = nomadCommand.Run()
	require.NoError(t, err)
	os.Setenv("NOMAD_ADDR", nomadAddr)
}

// Check for conflict with job that has meta, but no deployment key
func TestJobRunConflictingJobWithMetaButNoDeploymentKey(t *testing.T) {
	// TODO: Integrate test agent and solve envar dependency
	// this currently requires nomad agent -dev to be running and the
	// NOMAD_ADDR envar to be set.
	nomadAddr := os.Getenv("NOMAD_ADDR")
	os.Setenv("NOMAD_ADDR", "http://127.0.0.1:4646")

	baseCommand := &baseCommand{
		Ctx: context.Background(),
	}

	runCommand := RunCommand{baseCommand: baseCommand}

	nomadPath, err := exec.LookPath("nomad")
	require.NoError(t, err)

	nomadCommand := exec.Command(nomadPath, "run", "../fixtures/example-with-meta.nomad")
	err = nomadCommand.Run()
	require.NoError(t, err)

	// Now try to register
	exitCode := runCommand.Run([]string{"example"})
	require.Equal(t, 1, exitCode)

	// cleanup job
	nomadCommand = exec.Command(nomadPath, "job", "stop", "-purge", "example")
	err = nomadCommand.Run()
	require.NoError(t, err)

	os.Setenv("NOMAD_ADDR", nomadAddr)
}

func TestJobRunFails(t *testing.T) {
	// Fails with unavailable packs
	nomadAddr := os.Getenv("NOMAD_ADDR")
	os.Setenv("NOMAD_ADDR", "http://127.0.0.1:4646")

	baseCommand := &baseCommand{
		Ctx: context.Background(),
	}

	c := &RunCommand{baseCommand: baseCommand}

	exitCode := c.Run([]string{"fake-example"})
	require.Equal(t, 1, exitCode)
	os.Setenv("NOMAD_ADDR", nomadAddr)
}

func TestJobPlan(t *testing.T) {
	// TODO: Integrate test agent and solve envar dependency
	// this currently requires nomad agent -dev to be running and the
	// NOMAD_ADDR envar to be set.
	nomadAddr := os.Getenv("NOMAD_ADDR")
	os.Setenv("NOMAD_ADDR", "http://127.0.0.1:4646")

	baseCommand := &baseCommand{
		Ctx: context.Background(),
	}

	c := &PlanCommand{baseCommand: baseCommand}
	exitCode := c.Run([]string{"example"})

	// Should return 1 indicating an allocation will be placed
	require.Equal(t, 1, exitCode)
	os.Setenv("NOMAD_ADDR", nomadAddr)
}

// Confirm that another pack with the same job names but a different deployment name fails
func TestJobPlanConflictingDeployment(t *testing.T) {
	// TODO: Integrate test agent and solve envar dependency
	// this currently requires nomad agent -dev to be running and the
	// NOMAD_ADDR envar to be set.
	nomadAddr := os.Getenv("NOMAD_ADDR")
	os.Setenv("NOMAD_ADDR", "http://127.0.0.1:4646")

	baseCommand := &baseCommand{
		Ctx: context.Background(),
	}

	runCommand := RunCommand{baseCommand: baseCommand}

	// Register the initial pack
	exitCode := runCommand.Run([]string{"example"})
	require.Equal(t, 0, exitCode)

	// Plan another pack
	planCommand := PlanCommand{baseCommand: baseCommand}
	exitCode = planCommand.Run([]string{"example"}) // works because pack name above gets version appended.
	require.Equal(t, 255, exitCode)

	// Delete the pack
	stopCommand := StopCommand{baseCommand: baseCommand}
	exitCode = stopCommand.Run([]string{runCommand.deploymentName, "--purge=true"})
	require.Equal(t, 0, exitCode)

	os.Setenv("NOMAD_ADDR", nomadAddr)
}

// Check for conflict with non-pack job i.e. no meta
func TestJobPlanConflictingNonPackJob(t *testing.T) {
	// TODO: Integrate test agent and solve envar dependency
	// this currently requires nomad agent -dev to be running and the
	// NOMAD_ADDR envar to be set.
	nomadAddr := os.Getenv("NOMAD_ADDR")
	os.Setenv("NOMAD_ADDR", "http://127.0.0.1:4646")

	// Register non pack job
	nomadPath, err := exec.LookPath("nomad")
	require.NoError(t, err)
	nomadCommand := exec.Command(nomadPath, "run", "../fixtures/example.nomad")
	err = nomadCommand.Run()
	require.NoError(t, err)

	baseCommand := &baseCommand{
		Ctx: context.Background(),
	}

	planCommand := PlanCommand{baseCommand: baseCommand}

	// Now try to plan the pack
	exitCode := planCommand.Run([]string{"example"})
	require.Equal(t, 255, exitCode)

	// cleanup job
	nomadCommand = exec.Command(nomadPath, "job", "stop", "-purge", "example")
	err = nomadCommand.Run()
	require.NoError(t, err)
	os.Setenv("NOMAD_ADDR", nomadAddr)
}

// Check for conflict with job that has meta, but no deployment key
func TestJobPlanConflictingJobWithMetaButNoDeploymentKey(t *testing.T) {
	// TODO: Integrate test agent and solve envar dependency
	// this currently requires nomad agent -dev to be running and the
	// NOMAD_ADDR envar to be set.
	nomadAddr := os.Getenv("NOMAD_ADDR")
	os.Setenv("NOMAD_ADDR", "http://127.0.0.1:4646")

	baseCommand := &baseCommand{
		Ctx: context.Background(),
	}

	nomadPath, err := exec.LookPath("nomad")
	require.NoError(t, err)

	nomadCommand := exec.Command(nomadPath, "run", "../fixtures/example-with-meta.nomad")
	err = nomadCommand.Run()
	require.NoError(t, err)

	// Now try to register
	planCommand := PlanCommand{baseCommand: baseCommand}
	exitCode := planCommand.Run([]string{"example"})
	require.Equal(t, 255, exitCode)

	// cleanup job
	nomadCommand = exec.Command(nomadPath, "job", "stop", "-purge", "example")
	err = nomadCommand.Run()
	require.NoError(t, err)

	os.Setenv("NOMAD_ADDR", nomadAddr)
}

func TestJobStop(t *testing.T) {
	// TODO: Integrate test agent and solve envar dependency
	// this currently requires nomad agent -dev to be running and the
	// NOMAD_ADDR envar to be set.
	nomadAddr := os.Getenv("NOMAD_ADDR")
	os.Setenv("NOMAD_ADDR", "http://127.0.0.1:4646")

	baseCommand := &baseCommand{
		Ctx: context.Background(),
	}

	runCommand := &RunCommand{baseCommand: baseCommand}
	exitCode := runCommand.Run([]string{"example"})

	require.Equal(t, 0, exitCode)

	d := &StopCommand{baseCommand: baseCommand}
	exitCode = d.Run([]string{runCommand.packConfig.Name, "--purge=true"})
	require.Equal(t, 0, exitCode)

	os.Setenv("NOMAD_ADDR", nomadAddr)
}

func TestJobStopConflicts(t *testing.T) {
	nomadAddr := os.Getenv("NOMAD_ADDR")
	os.Setenv("NOMAD_ADDR", "http://127.0.0.1:4646")

	baseCommand := &baseCommand{
		Ctx: context.Background(),
	}

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
			packName:       "nomad_example",
			deploymentName: "",
			jobName:        "nomad_example",
		},
		{
			name:           "same-pack-diff-deploy",
			nonPackJob:     false,
			packName:       "nomad_example",
			deploymentName: "foo",
			jobName:        "job2",
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			// Create job
			nomadPath, err := exec.LookPath("nomad")
			require.NoError(t, err)

			if c.nonPackJob {
				nomadCommand := exec.Command(nomadPath, "run", "../fixtures/example.nomad")
				err = nomadCommand.Run()
				require.NoError(t, err)
			} else {
				r := &RunCommand{baseCommand: baseCommand}
				deploymentName := fmt.Sprintf("--name=%s", c.deploymentName)
				varJobName := fmt.Sprintf("--var=job_name=%s", c.jobName)
				exitCode := r.Run([]string{c.packName, deploymentName, varJobName})
				require.Equal(t, 0, exitCode)
			}

			// Try to stop job
			s := &StopCommand{baseCommand: baseCommand}
			exitCode := s.Run([]string{c.packName})
			require.Equal(t, 1, exitCode)

			// Purge job. Use nomad command since it'll work for all jobs
			nomadCommand := exec.Command(nomadPath, "stop", "-purge", c.jobName)
			err = nomadCommand.Run()
			require.NoError(t, err)
		})
	}
	os.Setenv("NOMAD_ADDR", nomadAddr)
}

// Destroy is just an alias for stop --purge so we only need to
// test that specific functionality
func TestJobDestroy(t *testing.T) {
	nomadAddr := os.Getenv("NOMAD_ADDR")
	os.Setenv("NOMAD_ADDR", "http://127.0.0.1:4646")

	baseCommand := &baseCommand{
		Ctx: context.Background(),
	}

	r := &RunCommand{baseCommand: baseCommand}
	r.Run([]string{"nomad_example"})

	d := &DestroyCommand{&StopCommand{baseCommand: baseCommand}}
	d.Run([]string{"nomad_example"})

	// Assert job no longer queryable
	nomadPath, err := exec.LookPath("nomad")
	require.NoError(t, err)

	nomadCommand := exec.Command(nomadPath, "status", "nomad_example")
	err = nomadCommand.Run()
	require.NoError(t, err)

	os.Setenv("NOMAD_ADDR", nomadAddr)
}

func TestJobDestroyFails(t *testing.T) {
	// Check you can't pass --purge flag to destroy command since
	// that doesn't make sense
	nomadAddr := os.Getenv("NOMAD_ADDR")
	os.Setenv("NOMAD_ADDR", "http://127.0.0.1:4646")

	baseCommand := &baseCommand{
		Ctx: context.Background(),
	}

	r := &RunCommand{baseCommand: baseCommand}
	exitCode := r.Run([]string{"nomad_example"})
	require.Equal(t, 0, exitCode)

	d := &DestroyCommand{&StopCommand{baseCommand: baseCommand}}
	exitCode = d.Run([]string{"nomad_example", "destroy", "--purge"})
	require.Equal(t, 1, exitCode)

	os.Setenv("NOMAD_ADDR", nomadAddr)
}

// Test that destroy properly uses var overrides to target the job
func TestJobDestroyWithOverrides(t *testing.T) {
	nomadAddr := os.Getenv("NOMAD_ADDR")
	os.Setenv("NOMAD_ADDR", "http://127.0.0.1:4646")

	baseCommand := &baseCommand{
		Ctx: context.Background(),
	}

	// Create multiple jobs in the same pack deployment
	r := &RunCommand{baseCommand: baseCommand}
	deployName := "--name=test"
	jobNames := []string{"foo", "bar"}
	for _, j := range jobNames {
		exitCode := r.Run([]string{"nomad_example", deployName, `--var=job_name=` + j})
		require.Equal(t, 0, exitCode)
	}

	// Stop nonexistent job
	d := &DestroyCommand{StopCommand: &StopCommand{baseCommand: baseCommand}}
	exitCode := d.Run([]string{r.packConfig.Name, deployName, "--var=job_name=baz"})
	require.Equal(t, 1, exitCode)

	// Stop job with var override
	exitCode = d.Run([]string{r.packConfig.Name, deployName, "--var=job_name=foo"})
	require.Equal(t, 0, exitCode)

	// Assert job "bar" still exists
	nomadPath, err := exec.LookPath("nomad")
	require.NoError(t, err)
	nomadCmd := exec.Command(nomadPath, "status", "bar")
	err = nomadCmd.Run()
	require.NoError(t, err)

	// Stop job with no overrides passed
	exitCode = d.Run([]string{r.packConfig.Name, deployName})
	require.Equal(t, 0, exitCode)

	// Assert job bar is gone
	nomadCmd = exec.Command(nomadPath, "status", "bar")
	err = nomadCmd.Run()
	require.Error(t, err)

	os.Setenv("NOMAD_ADDR", nomadAddr)
}

func TestFlagProvidedButNotDefined(t *testing.T) {
	// TODO: Integrate test agent and solve envar dependency
	// this currently requires nomad agent -dev to be running and the
	// NOMAD_ADDR envar to be set.
	nomadAddr := os.Getenv("NOMAD_ADDR")
	os.Setenv("NOMAD_ADDR", "http://127.0.0.1:4646")

	baseCommand := &baseCommand{
		Ctx: context.Background(),
	}

	r := &RunCommand{baseCommand: baseCommand}
	// There is no job flag. This tests that adding an unspecified flag does not
	// create an invalid memory address error
	// Posix case
	exitCode := r.Run([]string{"example", "--job=provided-but-not-defined"})
	require.Equal(t, 1, exitCode)

	// std go case
	exitCode = r.Run([]string{"-job=provided-but-not-defined", "example"})
	require.Equal(t, 1, exitCode)

	os.Setenv("NOMAD_ADDR", nomadAddr)
}

func TestStatus(t *testing.T) {
	// TODO: Integrate test agent and solve envar dependency
	// this currently requires nomad agent -dev to be running and the
	// NOMAD_ADDR envar to be set.
	nomadAddr := os.Getenv("NOMAD_ADDR")
	os.Setenv("NOMAD_ADDR", "http://127.0.0.1:4646")

	baseCommand := &baseCommand{
		Ctx: context.Background(),
	}

	s := &StatusCommand{baseCommand: baseCommand}
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
			args: []string{"nomad_example"},
		},
		{
			name: "with-pack-and-deploy-name",
			args: []string{"nomad_example", "--name=foo"},
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			exitCode := s.Run(c.args)
			require.Equal(t, 0, exitCode)
		})
	}
	os.Setenv("NOMAD_ADDR", nomadAddr)
}

func TestStatusFails(t *testing.T) {
	// TODO: Integrate test agent and solve envar dependency
	// this currently requires nomad agent -dev to be running and the
	// NOMAD_ADDR envar to be set.
	nomadAddr := os.Getenv("NOMAD_ADDR")
	os.Setenv("NOMAD_ADDR", "http://127.0.0.1:4646")

	baseCommand := &baseCommand{
		Ctx: context.Background(),
	}
	s := &StatusCommand{baseCommand: baseCommand}

	exitCode := s.Run([]string{"--name=foo"})
	require.Equal(t, 1, exitCode)

	os.Setenv("NOMAD_ADDR", nomadAddr)
}
