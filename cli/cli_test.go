package cli

import (
	"context"
	"fmt"
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

// TODO: Integrate test agent and solve envar dependency
// this currently requires nomad agent -dev to be running and the
// NOMAD_ADDR envar to be set.
type testUtil struct {
	nomadAddr   string
	baseCommand *baseCommand
}

func (u *testUtil) setup() {
	u.nomadAddr = os.Getenv("NOMAD_ADDR")
	os.Setenv("NOMAD_ADDR", "http://127.0.0.1:4646")

	u.baseCommand = &baseCommand{
		Ctx: context.Background(),
		Log: hclog.Default(),
	}
}

func (u *testUtil) reset() {
	os.Setenv("NOMAD_ADDR", u.nomadAddr)
}

func TestVersion(t *testing.T) {
	u := testUtil{"", nil}
	u.setup()
	defer u.reset()

	exitCode := Main([]string{"nomad-pack", "-v"})
	require.Equal(t, 0, exitCode)
}

//func TestRepoList(t *testing.T) {
//	req := require.New(t)
//
//	exitCode := Main([]string{"nom", "repo", "list"})
//
//	req.Equal(0, exitCode)
//}

func TestJobRun(t *testing.T) {
	u := testUtil{"", nil}
	u.setup()
	defer u.reset()

	c := RunCommand{baseCommand: u.baseCommand}

	exitCode := c.Run([]string{"nomad_example"})
	require.Equal(t, 0, exitCode)

	// TODO: add var overrides when fixed
}

// Confirm that another pack with the same job names but a different deployment name fails
func TestJobRunConflictingDeployment(t *testing.T) {
	u := testUtil{"", nil}
	u.setup()
	defer u.reset()

	runCommand := RunCommand{baseCommand: u.baseCommand}

	// Register the initial pack
	exitCode := runCommand.Run([]string{"nomad_example"})
	require.Equal(t, 0, exitCode)

	exitCode = runCommand.Run([]string{"nomad_example", "--name=with-name"})
	require.Equal(t, 1, exitCode)

	// Confirm that it's still possible to update the existing pack
	exitCode = runCommand.Run([]string{"nomad_example"})
	require.Equal(t, 0, exitCode)

	// Delete the pack
	stopCommand := StopCommand{baseCommand: u.baseCommand}
	exitCode = stopCommand.Run([]string{"nomad_example", "--purge=true"})
	require.Equal(t, 0, exitCode)
}

// Check for conflict with non-pack job i.e. no meta
func TestJobRunConflictingNonPackJob(t *testing.T) {
	u := testUtil{"", nil}
	u.setup()
	defer u.reset()

	// Register non pack job
	nomadPath, err := exec.LookPath("nomad")
	require.NoError(t, err)
	nomadCommand := exec.Command(nomadPath, "run", "../fixtures/example.nomad")
	err = nomadCommand.Run()
	require.NoError(t, err)

	runCommand := RunCommand{baseCommand: u.baseCommand}

	// Now try to register the pack
	exitCode := runCommand.Run([]string{"nomad_example"})
	require.Equal(t, 1, exitCode)

	// cleanup job
	nomadCommand = exec.Command(nomadPath, "job", "stop", "-purge", "nomad_example")
	err = nomadCommand.Run()
	require.NoError(t, err)
}

// Check for conflict with job that has meta, but no deployment key
func TestJobRunConflictingJobWithMetaButNoDeploymentKey(t *testing.T) {
	u := testUtil{"", nil}
	u.setup()
	defer u.reset()

	runCommand := RunCommand{baseCommand: u.baseCommand}

	nomadPath, err := exec.LookPath("nomad")
	require.NoError(t, err)

	nomadCommand := exec.Command(nomadPath, "run", "../fixtures/example-with-meta.nomad")
	err = nomadCommand.Run()
	require.NoError(t, err)

	// Now try to register
	exitCode := runCommand.Run([]string{"nomad_example"})
	require.Equal(t, 1, exitCode)

	// cleanup job
	nomadCommand = exec.Command(nomadPath, "job", "stop", "-purge", "nomad_example")
	err = nomadCommand.Run()
	require.NoError(t, err)
}

func TestJobRunFails(t *testing.T) {
	// Fails with unavailable packs
	u := testUtil{"", nil}
	u.setup()
	defer u.reset()

	c := &RunCommand{baseCommand: u.baseCommand}

	exitCode := c.Run([]string{"fake-example"})
	require.Equal(t, 1, exitCode)
}

func TestJobPlan(t *testing.T) {
	u := testUtil{"", nil}
	u.setup()
	defer u.reset()

	c := &PlanCommand{baseCommand: u.baseCommand}
	exitCode := c.Run([]string{"nomad_example"})

	// Should return 1 indicating an allocation will be placed
	require.Equal(t, 1, exitCode)
}

// Confirm that another pack with the same job names but a different deployment name fails
func TestJobPlanConflictingDeployment(t *testing.T) {
	u := testUtil{"", nil}
	u.setup()
	defer u.reset()

	runCommand := RunCommand{baseCommand: u.baseCommand}

	// Register the initial pack
	exitCode := runCommand.Run([]string{"nomad_example"})
	require.Equal(t, 0, exitCode)

	// Plan another pack
	planCommand := PlanCommand{baseCommand: u.baseCommand}
	exitCode = planCommand.Run([]string{"nomad_example"}) // works because pack name above gets version appended.
	require.Equal(t, 255, exitCode)

	// Delete the pack
	stopCommand := StopCommand{baseCommand: u.baseCommand}
	exitCode = stopCommand.Run([]string{runCommand.deploymentName, "--purge=true"})
	require.Equal(t, 0, exitCode)
}

// Check for conflict with non-pack job i.e. no meta
func TestJobPlanConflictingNonPackJob(t *testing.T) {
	u := testUtil{"", nil}
	u.setup()
	defer u.reset()

	// Register non pack job
	nomadPath, err := exec.LookPath("nomad")
	require.NoError(t, err)
	nomadCommand := exec.Command(nomadPath, "run", "../fixtures/example.nomad")
	err = nomadCommand.Run()
	require.NoError(t, err)

	planCommand := PlanCommand{baseCommand: u.baseCommand}

	// Now try to plan the pack
	exitCode := planCommand.Run([]string{"nomad_example"})
	require.Equal(t, 255, exitCode)

	// cleanup job
	nomadCommand = exec.Command(nomadPath, "job", "stop", "-purge", "nomad_example")
	err = nomadCommand.Run()
	require.NoError(t, err)
}

// Check for conflict with job that has meta, but no deployment key
func TestJobPlanConflictingJobWithMetaButNoDeploymentKey(t *testing.T) {
	u := testUtil{"", nil}
	u.setup()
	defer u.reset()

	nomadPath, err := exec.LookPath("nomad")
	require.NoError(t, err)

	nomadCommand := exec.Command(nomadPath, "run", "../fixtures/example-with-meta.nomad")
	err = nomadCommand.Run()
	require.NoError(t, err)

	// Now try to register
	planCommand := PlanCommand{baseCommand: u.baseCommand}
	exitCode := planCommand.Run([]string{"nomad_example"})
	require.Equal(t, 255, exitCode)

	// cleanup job
	nomadCommand = exec.Command(nomadPath, "job", "stop", "-purge", "nomad_example")
	err = nomadCommand.Run()
	require.NoError(t, err)
}

func TestJobStop(t *testing.T) {
	u := testUtil{"", nil}
	u.setup()
	defer u.reset()

	jobName := "nomad_example"
	runCommand := &RunCommand{baseCommand: u.baseCommand}
	exitCode := runCommand.Run([]string{"nomad_example"})
	require.Equal(t, 0, exitCode)

	// Test without purging
	d := &StopCommand{baseCommand: u.baseCommand}
	exitCode = d.Run([]string{runCommand.packName})
	require.Equal(t, 0, exitCode)

	// Assert the job is still queryable
	nomadPath, err := exec.LookPath("nomad")
	require.NoError(t, err)
	nomadCommand := exec.Command(nomadPath, "status", jobName)
	err = nomadCommand.Run()
	require.NoError(t, err)

	// Purge the job
	exitCode = d.Run([]string{runCommand.packName, "--purge=true"})
	require.Equal(t, 0, exitCode)

	// Assert the job no longer exists
	nomadCommand = exec.Command(nomadPath, "status", jobName)
	err = nomadCommand.Run()
	require.Error(t, err)
}

func TestJobStopConflicts(t *testing.T) {
	u := testUtil{"", nil}
	u.setup()
	defer u.reset()

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
				r := &RunCommand{baseCommand: u.baseCommand}
				deploymentName := fmt.Sprintf("--name=%s", c.deploymentName)
				varJobName := fmt.Sprintf("--var=job_name=%s", c.jobName)
				exitCode := r.Run([]string{c.packName, deploymentName, varJobName})
				require.Equal(t, 0, exitCode)
			}

			// Try to stop job
			s := &StopCommand{baseCommand: u.baseCommand}
			exitCode := s.Run([]string{c.packName})
			require.Equal(t, 1, exitCode)

			// Purge job. Use nomad command since it'll work for all jobs
			nomadCommand := exec.Command(nomadPath, "stop", "-purge", c.jobName)
			err = nomadCommand.Run()
			require.NoError(t, err)
		})
	}
}

// TODO: need to figure this out re. templatized job names.
// Right now, if you pass a job name as a var override, stop/destroy totally ignores
// it, i.e. you can say destroy --name foo --var job_name=foo and if you have other
// jobs in deployment foo, destroy will just destroy all of them. In fact, even if
// no job with the name foo exists in deployment foo, it will still destroy everything
// in that pack and deployment because that's the only things it checks, which is not
// great.
func TestJobStopWithVarOverrides(t *testing.T) {

}

// Destroy is just an alias for stop --purge so we only need to
// test that specific functionality
func TestJobDestroy(t *testing.T) {
	u := testUtil{"", nil}
	u.setup()
	defer u.reset()

	r := &RunCommand{baseCommand: u.baseCommand}
	r.Run([]string{"nomad_example"})

	d := &DestroyCommand{&StopCommand{baseCommand: u.baseCommand}}
	d.Run([]string{"nomad_example"})

	// Assert job no longer queryable
	nomadPath, err := exec.LookPath("nomad")
	require.NoError(t, err)

	nomadCommand := exec.Command(nomadPath, "status", "nomad_example")
	err = nomadCommand.Run()
	require.NoError(t, err)
}

func TestJobDestroyFails(t *testing.T) {
	// Check you can't pass --purge flag to destroy command since
	// that doesn't make sense
	u := testUtil{"", nil}
	u.setup()
	defer u.reset()

	r := &RunCommand{baseCommand: u.baseCommand}
	exitCode := r.Run([]string{"nomad_example"})
	require.Equal(t, 0, exitCode)

	d := &DestroyCommand{&StopCommand{baseCommand: u.baseCommand}}
	exitCode = d.Run([]string{"nomad_example", "destroy", "--purge"})
	require.Equal(t, 1, exitCode)
}

func TestFlagProvidedButNotDefined(t *testing.T) {
	u := testUtil{"", nil}
	u.setup()
	defer u.reset()

	r := &RunCommand{baseCommand: u.baseCommand}
	// There is no job flag. This tests that adding an unspecified flag does not
	// create an invalid memory address error
	// Posix case
	exitCode := r.Run([]string{"nomad_example", "--job=provided-but-not-defined"})
	require.Equal(t, 1, exitCode)

	// std go case
	exitCode = r.Run([]string{"-job=provided-but-not-defined", "example"})
	require.Equal(t, 1, exitCode)
}
