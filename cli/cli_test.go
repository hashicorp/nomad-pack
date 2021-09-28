package cli

import (
	"context"
	"os"
	"os/exec"
	"testing"

	"github.com/hashicorp/go-hclog"
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

	exitCode := Main([]string{"nom", "-v"})
	require.Equal(t, 0, exitCode)
	os.Setenv("NOMAD_ADDR", nomadAddr)
}

//func TestRepoList(t *testing.T) {
//	req := require.New(t)
//
//	exitCode := Main([]string{"nom", "repo", "list"})
//
//	req.Equal(0, exitCode)
//}

func TestJobRun(t *testing.T) {
	// TODO: Integrate test agent and solve envar dependency
	// this currently requires nomad agent -dev to be running and the
	// NOMAD_ADDR envar to be set.
	nomadAddr := os.Getenv("NOMAD_ADDR")
	os.Setenv("NOMAD_ADDR", "http://127.0.0.1:4646")

	baseCommand := &baseCommand{
		Ctx: context.Background(),
		Log: hclog.Default(),
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
		Log: hclog.Default(),
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
	destroyCommand := DestroyCommand{baseCommand: baseCommand}
	exitCode = destroyCommand.Run([]string{runCommand.deploymentName, "--purge=true"})
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
		Log: hclog.Default(),
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
		Log: hclog.Default(),
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
		Log: hclog.Default(),
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
		Log: hclog.Default(),
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
		Log: hclog.Default(),
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
	destroyCommand := DestroyCommand{baseCommand: baseCommand}
	exitCode = destroyCommand.Run([]string{runCommand.deploymentName, "--purge=true"})
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
		Log: hclog.Default(),
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
		Log: hclog.Default(),
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

func TestJobDestroy(t *testing.T) {
	// TODO: Integrate test agent and solve envar dependency
	// this currently requires nomad agent -dev to be running and the
	// NOMAD_ADDR envar to be set.
	nomadAddr := os.Getenv("NOMAD_ADDR")
	os.Setenv("NOMAD_ADDR", "http://127.0.0.1:4646")

	baseCommand := &baseCommand{
		Ctx: context.Background(),
		Log: hclog.Default(),
	}

	runCommand := &RunCommand{baseCommand: baseCommand}
	exitCode := runCommand.Run([]string{"example"})

	require.Equal(t, 0, exitCode)

	d := &DestroyCommand{baseCommand: baseCommand}
	exitCode = d.Run([]string{runCommand.deploymentName, "--purge=true"})
	require.Equal(t, 0, exitCode)

	os.Setenv("NOMAD_ADDR", nomadAddr)
}

func TestJobDestroyConflicts(t *testing.T) {
	// TODO: Integrate test agent and solve envar dependency
	// this currently requires nomad agent -dev to be running and the
	// NOMAD_ADDR envar to be set.
	nomadAddr := os.Getenv("NOMAD_ADDR")
	os.Setenv("NOMAD_ADDR", "http://127.0.0.1:4646")

	baseCommand := &baseCommand{
		Ctx: context.Background(),
		Log: hclog.Default(),
	}

	runCommand := &RunCommand{baseCommand: baseCommand}
	exitCode := runCommand.Run([]string{"example"})

	require.Equal(t, 0, exitCode)

	d := &DestroyCommand{baseCommand: baseCommand}
	exitCode = d.Run([]string{runCommand.deploymentName, "--purge=true"})
	require.Equal(t, 0, exitCode)

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
		Log: hclog.Default(),
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
