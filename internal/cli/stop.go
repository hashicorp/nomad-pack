// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package cli

import (
	"context"
	"fmt"
	"time"

	"github.com/hashicorp/nomad/api"
	"github.com/posener/complete"

	"github.com/hashicorp/nomad-pack/internal/pkg/cache"
	"github.com/hashicorp/nomad-pack/internal/pkg/errors"
	"github.com/hashicorp/nomad-pack/internal/pkg/flag"
	"github.com/hashicorp/nomad-pack/internal/pkg/helper"
	"github.com/hashicorp/nomad-pack/internal/pkg/renderer"
)

type StopCommand struct {
	*baseCommand
	packConfig *cache.PackConfig
	purge      bool
	global     bool
	Validation ValidationFn
}

func (c *StopCommand) Run(args []string) int {
	var err error

	c.cmdKey = "stop" // Add cmd key here so help text is available in Init
	// Initialize. If we fail, we just exit since Init handles the UI.
	if err = c.Init(
		WithExactArgs(1, args),
		WithFlags(c.Flags()),
		WithNoConfig(),
	); err != nil {
		c.ui.ErrorWithUsageAndContext(err, ErrParsingArgsOrFlags, c)
		return 1
	}

	// Since we call this command from destroy, set up the correct verbiage
	// for nicer output
	var (
		stopOrDestroy        = "stop"
		stoppingOrDestroying = "stopping"
		stoppedOrDestroyed   = "stopped"
	)
	if c.purge {
		stopOrDestroy = "destroy"
		stoppingOrDestroying = "destroying"
		stoppedOrDestroyed = "destroyed"
	}

	c.packConfig.Name = c.args[0]

	// Set the packConfig defaults if necessary and generate our UI error context.
	errorContext := initPackCommand(c.packConfig)

	client, err := c.getAPIClient()
	if err != nil {
		c.ui.ErrorWithContext(err, "failed to initialize client", errorContext.GetAll()...)
		return 1
	}

	if c.deploymentName == "" {
		// Add the path to the pack on the error context.
		errorContext.Add(errors.UIContextPrefixPackPath, c.packConfig.Path)

		// Add the path to the pack on the error context.
		errorContext.Add(errors.UIContextPrefixPackRef, c.packConfig.Ref)

		// If no deploymentName set default to pack@ref
		c.deploymentName = getDeploymentName(c.baseCommand, c.packConfig)
	}
	errorContext.Add(errors.UIContextPrefixDeploymentName, c.deploymentName)

	var jobs []*api.Job

	// Get job names if var overrides are passed
	if hasVarOverrides(c.baseCommand) {
		packManager := generatePackManager(c.baseCommand, client, c.packConfig)

		var r *renderer.Rendered

		// render the pack
		r, err = renderPack(
			packManager,
			c.baseCommand.ui,
			false,
			false,
			c.baseCommand.ignoreMissingVars,
			errorContext,
		)
		if err != nil {
			return 255
		}

		// Commands that render templates are required to render at least one
		// parent template.
		if r.LenParentRenders() < 1 {
			c.ui.ErrorWithContext(errors.ErrNoTemplatesRendered, "no templates rendered", errorContext.GetAll()...)
			return 1
		}

		for tplName, tpl := range r.ParentRenders() {

			// tplErrorContext forms the basis for error output context as is
			// appended to when new information becomes available.
			tplErrorContext := errorContext.Copy()
			tplErrorContext.Add(errors.UIContextPrefixTemplateName, tplName)

			// get job struct from template
			// TODO: Should we add an hcl1 flag?
			var job *api.Job
			job, err = parseJob(c.baseCommand, tpl, false, tplErrorContext)
			if err != nil {
				// err output is handled by parseJob
				return 1
			}

			// Add the jobID to the error context.
			tplErrorContext.Add(errors.UIContextPrefixJobName, *job.Name)
			jobs = append(jobs, job)
		}
	} else {
		// If no job names are specified, get all jobs belonging to the pack and deployment
		jobs, err = getPackJobsByDeploy(client, c.packConfig, c.deploymentName)
		if err != nil {
			c.ui.ErrorWithContext(err, "failed to find jobs for pack", errorContext.GetAll()...)
			return 1
		}

		if len(jobs) == 0 {
			c.ui.Warning(fmt.Sprintf("no jobs found for pack %q", c.packConfig.Name))
			return 1
		}
	}

	var errs []error
	for _, job := range jobs {
		err = c.checkForConflicts(client, job)

		if err != nil {
			errs = append(errs, err)
			c.ui.Warning(fmt.Sprintf("skipping job %q - conflict check failed with err: %s", *job.ID, err))
			continue
		}

		// TODO: add interactive support
		if !c.confirmStop() {
			c.ui.Info(fmt.Sprintf("%s job %q aborted by user", helper.Title(stopOrDestroy), *job.ID))
			continue
		}

		// Invoke the stop
		_, _, err := client.Jobs().DeregisterOpts(*job.ID, &api.DeregisterOptions{
			Purge:  c.purge,
			Global: c.global,
		}, &api.WriteOptions{})
		if err != nil {
			errs = append(errs, err)
			c.ui.ErrorWithContext(err, fmt.Sprintf("error deregistering job: %q", *job.ID))
			continue
		}

		c.ui.Success(fmt.Sprintf("Job %q %s", *job.Name, stoppedOrDestroyed))
	}

	if len(errs) > 0 {
		c.ui.Warning(fmt.Sprintf("Pack %q %s complete with errors", c.packConfig.Name, stopOrDestroy))
		for _, err := range errs {
			msg := fmt.Sprintf("error %s pack", stoppingOrDestroying)
			c.ui.ErrorWithContext(err, msg, errorContext.GetAll()...)
		}
		return 1
	}

	c.ui.Success(fmt.Sprintf("Pack %q %s", c.packConfig.Name, stoppedOrDestroyed))
	return 0
}

func (c *StopCommand) checkForConflicts(client *api.Client, job *api.Job) error {
	queryOpts := &api.QueryOptions{}
	if job.Namespace != nil {
		queryOpts.Namespace = *job.Namespace
	}

	queryOpts.Prefix = *job.ID
	jobsApi := client.Jobs()

	jobs, _, err := jobsApi.List(queryOpts.WithContext(context.Background()))
	if err != nil {
		return fmt.Errorf("error checking for conflicts for job %q: %s", *job.Name, err)
	}

	if len(jobs) == 0 {
		return fmt.Errorf("no job(s) with prefix or id %q found", *job.Name)
	}

	if len(jobs) > 1 {
		return fmt.Errorf("prefix matched multiple jobs\n\n%s", createStatusListOutput(jobs, c.allNamespaces()))
	}

	return nil
}

// TODO: Add interactive support
func (c *StopCommand) confirmStop() bool {
	// TODO: Confirm the stop if the job was a prefix match
	// TODO: Confirm we want to stop only a single region of a multiregion job
	return true
}

func (c *StopCommand) Flags() *flag.Sets {
	return c.flagSet(flagSetOperation|flagSetNomadClient, func(set *flag.Sets) {
		c.packConfig = &cache.PackConfig{}

		f := set.NewSet("Stop Options")
		f.StringVar(&flag.StringVar{
			Name:    "registry",
			Target:  &c.packConfig.Registry,
			Default: "",
			Usage:   `Specific registry name containing the pack to be stopped.`,
		})

		f.StringVar(&flag.StringVar{
			Name:    "ref",
			Target:  &c.packConfig.Ref,
			Default: "",
			Usage: `Specific git ref of the pack to be stopped.
					Supports tags, SHA, and latest. If no ref is specified,
					defaults to latest.

					Using ref with a file path is not supported.`,
		})

		f.BoolVar(&flag.BoolVar{
			Name:    "purge",
			Target:  &c.purge,
			Default: false,
			Usage: `Purge is used to stop packs and purge them from the system.
					If not set, packs will still be queryable and will be purged
					by the garbage collector.`,
		})

		f.BoolVar(&flag.BoolVar{
			Name:    "global",
			Target:  &c.global,
			Default: false,
			Usage: `Stop multi-region pack in all its regions. By default, pack
					stop will stop only a single region at a time. Ignored for
					single-region jobs.`,
		})
	})
}

func (c *StopCommand) AutocompleteArgs() complete.Predictor {
	return complete.PredictNothing
}

func (c *StopCommand) AutocompleteFlags() complete.Flags {
	return c.Flags().Completions()
}

func (c *StopCommand) Help() string {
	c.Example = `
	# Stop an example pack in deployment "dev"
	nomad-pack stop example --name=dev

	# Stop an example pack in deployment "dev" and purge it from the system
	nomad-pack stop example --name=dev --purge

	# Stop an example pack in deployment "dev" that has a job named "test"
	# If the same pack has been installed in deployment "dev" but overriding the
	# job name to "hello", only "test" will be stopped
	nomad-pack stop example --name=dev --var=job_name=test
	`
	return formatHelp(`
	Usage: nomad-pack stop <pack name> [options]

	Stop the specified Nomad Pack in the configured Nomad cluster. To delete the
	pack from the cluster, specify "--purge", or use the "nomad-pack destroy"
	command.

	By default, the stop command will stop ALL jobs in the pack deployment. If a
	pack was run using variable overrides to specify the job name(s), the same
	variable overrides MUST be provided when stopping the pack to guarantee that
	nomad-pack targets the correct job(s) in the pack deployment.

` + c.GetExample() + c.Flags().Help())
}

// Synopsis satisfies the Synopsis function of the cli.Command interface.
func (c *StopCommand) Synopsis() string {
	return "Stop a running pack"
}

func (c *StopCommand) allNamespaces() bool {
	// TODO: Wire into common CommandOpts when available.
	return true
}

// list general information about a list of jobs
func createStatusListOutput(jobs []*api.JobListStub, displayNS bool) string {
	out := make([]string, len(jobs)+1)
	if displayNS {
		out[0] = "ID|Namespace|Type|Priority|Status|Submit Date"
		for i, job := range jobs {
			// TODO: Fix this demo hack
			t := time.Now()
			if job.SubmitTime != 0 {
				t = time.Unix(0, job.SubmitTime)
			}
			out[i+1] = fmt.Sprintf("%s|%s|%s|%d|%s|%s",
				job.ID,
				job.JobSummary.Namespace,
				getTypeString(*job),
				job.Priority,
				getStatusString(job.Status, job.Stop), formatTime(t))
		}
	} else {
		out[0] = "ID|Type|Priority|Status|Submit Date"
		for i, job := range jobs {
			// TODO: Fix this demo hack
			t := time.Now()
			if job.SubmitTime != 0 {
				t = time.Unix(0, job.SubmitTime)
			}
			out[i+1] = fmt.Sprintf("%s|%s|%d|%s|%s",
				job.ID,
				getTypeString(*job),
				job.Priority,
				getStatusString(job.Status, job.Stop), formatTime(t))
		}
	}
	return formatList(out)
}

func getTypeString(job api.JobListStub) string {
	t := job.Type

	if job.Periodic {
		t += "/periodic"
	}

	if job.ParameterizedJob {
		t += "/parameterized"
	}

	return t
}

func getStatusString(status string, stop bool) string {
	if status == "" {
		return ""
	}
	if stop {
		return fmt.Sprintf("%s (stopped)", status)
	}
	return status
}
