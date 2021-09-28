package cli

import (
	"fmt"
	"time"

	"github.com/hashicorp/nom/flag"
	"github.com/hashicorp/nom/internal/pkg/errors"
	"github.com/hashicorp/nom/internal/pkg/version"
	v1client "github.com/hashicorp/nomad-openapi/clients/go/v1"
	v1 "github.com/hashicorp/nomad-openapi/v1"
)

type DestroyCommand struct {
	*baseCommand
	packName   string
	repoName   string
	serverURL  string
	purge      bool
	global     bool
	Validation ValidationFn
}

func (c *DestroyCommand) Run(args []string) int {
	c.cmdKey = "destroy" // Add cmd key here so help text is available in Init
	// Initialize. If we fail, we just exit since Init handles the UI.
	if err := c.Init(
		WithExactArgs(1, args),
		WithFlags(c.Flags()),
		WithNoConfig(),
	); err != nil {
		return 1
	}

	packRepoName := c.args[0]

	// Generate our UI error context.
	errorContext := errors.NewUIErrorContext()

	repoName, packName, err := parseRepoFromPackName(packRepoName)
	if err != nil {
		c.ui.ErrorWithContext(err, "unable to parse pack name", errorContext.GetAll()...)
	}
	c.packName = packName
	c.repoName = repoName
	errorContext.Add(errors.UIContextPrefixPackName, c.packName)
	errorContext.Add(errors.UIContextPrefixPackName, c.repoName)

	client, err := v1.NewClient()
	if err != nil {
		c.ui.ErrorWithContext(err, "failed to initialize client", errorContext.GetAll()...)
		return 1
	}

	// set a local variable to the JobsApi
	jobsApi := client.Jobs()

	if c.deploymentName == "" {
		tempRepoPath, err := getRepoPath(c.repoName, c.ui, errorContext)
		if err != nil {
			return 1
		}

		// Add the path to the pack on the error context.
		errorContext.Add(errors.UIContextPrefixPackPath, tempRepoPath)

		// get pack git version
		// TODO: Get this from pack metadata.
		packVersion, err := version.PackVersion(tempRepoPath)
		if err != nil {
			c.ui.ErrorWithContext(err, "failed to determine pack version", errorContext.GetAll()...)
		}

		// Add the path to the pack on the error context.
		errorContext.Add(errors.UIContextPrefixPackVersion, packVersion)

		// If no deploymentName set default to pack@version
		c.deploymentName = getDeploymentName(c.baseCommand, c.packName, packVersion)
		errorContext.Add(errors.UIContextPrefixDeploymentName, c.deploymentName)
	}

	jobs, err := getDeployedPackJobs(jobsApi, c.packName, c.deploymentName)
	if err != nil {
		c.ui.ErrorWithContext(err, "failed to find jobs for pack", errorContext.GetAll()...)
		return 1
	}

	if len(jobs) == 0 {
		c.ui.Warning(fmt.Sprintf("no jobs found for pack %s", c.packName))
		return 1
	}

	hasErrs := false
	for _, job := range jobs {
		err = c.checkForConflicts(jobsApi, *job.ID)
		if err != nil {
			hasErrs = true
			c.ui.Warning(fmt.Sprintf("skipping job %s - conflict check failed with err: %s", *job.ID, err))
			continue
		}

		// TODO: add interactive support
		if !c.confirmDestroy() {
			c.ui.Info(fmt.Sprintf("Destroy for job %s aborted by user", *job.ID))
			continue
		}

		// Invoke the stop
		writeOpts := &v1.WriteOpts{
			Region:    *job.Region,
			Namespace: *job.Namespace,
		}
		result, _, err := client.Jobs().Delete(writeOpts.Ctx(), *job.Name, c.purge, c.global)
		if err != nil {
			hasErrs = true
			c.ui.ErrorWithContext(err, fmt.Sprintf("error deregistering job: %s", *job.ID))
			continue
		}

		// If we are stopping a periodic job there won't be an evalID.
		if result.EvalID != nil && *result.EvalID == "" {
			c.ui.Info(fmt.Sprintf("EvalID: %s", *result.EvalID))
		}

		c.ui.Success(fmt.Sprintf("Job %s destroyed", *job.Name))
	}

	if hasErrs {
		c.ui.Warning(fmt.Sprintf("Pack %s destroy complete with errors", c.packName))
		return 1
	}

	c.ui.Success(fmt.Sprintf("Pack %s destroyed", c.packName))
	return 0
}

func (c *DestroyCommand) checkForConflicts(jobsApi *v1.Jobs, jobName string) error {
	queryOpts := &v1.QueryOpts{
		Prefix: jobName,
	}
	jobs, _, err := jobsApi.GetJobs(queryOpts.Ctx())
	if err != nil {
		return fmt.Errorf("error checking for conflicts for job %s: %s", jobName, err)
	}

	if len(jobs) == 0 {
		return fmt.Errorf("no job(s) with prefix or id %s found", jobName)
	}

	if len(jobs) > 1 {
		return fmt.Errorf("prefix matched multiple jobs\n\n%s", createStatusListOutput(jobs, c.allNamespaces()))
	}

	return nil
}

// TODO: Add interactive support
func (c *DestroyCommand) confirmDestroy() bool {
	return true
	//getConfirmation := func(question string) (int, bool) {
	//	answer, err := c.ui.Input(question)
	//	if err != nil {
	//		c.ui.Output(fmt.Sprintf("Failed to parse answer: %v", err))
	//		return 1, false
	//	}
	//
	//	if answer == "" || strings.ToLower(answer)[0] == 'n' {
	//		// No case
	//		c.ui.Output("Cancelling job stop")
	//		return 0, false
	//	} else if strings.ToLower(answer)[0] == 'y' && len(answer) > 1 {
	//		// Non-exact match yes
	//		c.ui.Output("For confirmation, an exact ‘y’ is required.")
	//		return 0, false
	//	} else if answer != "y" {
	//		c.ui.Output("No confirmation detected. For confirmation, an exact 'y' is required.")
	//		return 1, false
	//	}
	//	return 0, true
	//}

	// Confirm the stop if the job was a prefix match
	// TODO: Add interactive support
	//if c.jobName != *job.ID {
	//	question := fmt.Sprintf("Are you sure you want to stop job %q? [y/N]", *job.ID)
	//	code, confirmed := getConfirmation(question)
	//	if !confirmed {
	//		return code
	//	}
	//}

	// Confirm we want to stop only a single region of a multiregion job
	// TODO: Add interactive support
	//	question := fmt.Sprintf(
	//		"Are you sure you want to stop multi-region job %q in a single region? [y/N]", *job.ID)
	//	code, confirmed := getConfirmation(question)
	//	if !confirmed {
	//		return code
	//	}
	//}
}

func (c *DestroyCommand) Flags() *flag.Sets {
	return c.flagSet(flagSetOperation, func(set *flag.Sets) {
		set.HideUnusedFlags("Operation Options", []string{"var", "var-file"})

		f := set.NewSet("Destroy Options")

		f.BoolVar(&flag.BoolVar{
			Name:    "purge",
			Target:  &c.purge,
			Default: false,
			Usage: `Purge is used to stop packs and purge them from the system. 
					If not set, packs will still be queryable and will be purged by the garbage collector.`,
		})

		f.BoolVar(&flag.BoolVar{
			Name:    "global",
			Target:  &c.global,
			Default: false,
			Usage: `Stop multi-region pack in all its regions. By default, pack 
					destroy will destroy only a single region at a time. Ignored for single-region jobs.`,
		})
	})
}

func (c *DestroyCommand) Help() string {
	c.Example = `
	# Destroy an example pack named "dev"
	nomad-pack destroy example --name=dev

	# Destroy an example pack named "dev" and purge it from the system
	nomad-pack destroy example --name=dev --purge
	`
	return formatHelp(`
	Usage: nomad-pack destroy <pack name> [options]

	Destroy the specified Nomad Pack from the configured Nomad cluster.
	
` + c.GetExample() + c.Flags().Help())
}

// Synopsis satisfies the Synopsis function of the cli.Command interface.
func (c *DestroyCommand) Synopsis() string {
	return "Stop an existing pack"
}

func (c *DestroyCommand) allNamespaces() bool {
	// TODO: Wire into common CommandOpts when available.
	return true
}

// list general information about a list of jobs
func createStatusListOutput(jobs []v1client.JobListStub, displayNS bool) string {
	out := make([]string, len(jobs)+1)
	if displayNS {
		out[0] = "ID|Namespace|Type|Priority|Status|Submit Date"
		for i, job := range jobs {
			// TODO: Fix this demo hack
			t := time.Now()
			if job.SubmitTime != nil {
				t = time.Unix(0, *job.SubmitTime)
			}
			out[i+1] = fmt.Sprintf("%s|%s|%s|%d|%s|%s",
				*job.ID,
				*job.JobSummary.Namespace,
				getTypeString(&job),
				job.Priority,
				getStatusString(job.Status, job.Stop), formatTime(&t))
		}
	} else {
		out[0] = "ID|Type|Priority|Status|Submit Date"
		for i, job := range jobs {
			// TODO: Fix this demo hack
			t := time.Now()
			if job.SubmitTime != nil {
				t = time.Unix(0, *job.SubmitTime)
			}
			out[i+1] = fmt.Sprintf("%s|%s|%d|%s|%s",
				*job.ID,
				getTypeString(&job),
				job.Priority,
				getStatusString(job.Status, job.Stop), formatTime(&t))
		}
	}
	return formatList(out)
}

func getTypeString(job *v1client.JobListStub) string {
	t := *job.Type

	if job.Periodic != nil && *job.Periodic {
		t += "/periodic"
	}

	if job.ParameterizedJob != nil && *job.ParameterizedJob {
		t += "/parameterized"
	}

	return t
}

func getStatusString(status *string, stop *bool) string {
	if status == nil {
		return ""
	}
	if stop != nil && *stop {
		return fmt.Sprintf("%s (stopped)", *status)
	}
	return *status
}
