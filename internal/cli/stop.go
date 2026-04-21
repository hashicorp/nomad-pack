// Copyright IBM Corp. 2023, 2026
// SPDX-License-Identifier: MPL-2.0

package cli

import (
	"context"
	"fmt"
	"time"

	"github.com/hashicorp/nomad/api"
	"github.com/posener/complete"

	"github.com/hashicorp/nomad-pack/internal/pkg/caching"
	"github.com/hashicorp/nomad-pack/internal/pkg/errors"
	"github.com/hashicorp/nomad-pack/internal/pkg/flag"
	"github.com/hashicorp/nomad-pack/internal/pkg/helper"
	"github.com/hashicorp/nomad-pack/internal/pkg/renderer"
)

type StopCommand struct {
	*baseCommand
	packConfig *caching.PackConfig
	purge      bool
	global     bool
	detach     bool
	verbose    bool
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
		c.ui.ErrorWithContext(err, ErrParsingArgsOrFlags)
		c.ui.Info(c.helpUsageMessage())
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

	packManager := generatePackManager(c.baseCommand, client, c.packConfig, nil)

	var r *renderer.Rendered

	// render the pack to get the jobs to stop.
	r, err = renderPack(
		packManager,
		c.ui,
		false,
		false,
		c.ignoreMissingVars,
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
		var job *api.Job
		job, err = parseJob(c.baseCommand, tpl, tplErrorContext)
		if err != nil {
			// err output is handled by parseJob
			return 1
		}

		// Add the jobID to the error context.
		tplErrorContext.Add(errors.UIContextPrefixJobName, *job.Name)
		jobs = append(jobs, job)
	}

	// Filter the rendered jobs to only those matching the deployment name.
	// This is necessary because the IDs of rendered pack jobs may match with jobs
	// from other deployments or non-pack jobs.
	jobs, err = getPackJobsByDeploy(jobs, client, c.packConfig, c.deploymentName)
	if err != nil {
		c.ui.ErrorWithContext(err, "failed to find jobs for pack", errorContext.GetAll()...)
		return 1
	}

	if len(jobs) == 0 {
		c.ui.Warning(fmt.Sprintf("no jobs found for pack %q", c.packConfig.Name))
		return 1
	}

	var errs []error
	var evalIDs []string
	var stoppedJobs []string

	for _, job := range jobs {
		err = c.checkForConflicts(client, job)

		if err != nil {
			errs = append(errs, err)
			c.ui.Warning(fmt.Sprintf("skipping job %q - job validation failed with err: %s", *job.ID, err))
			continue
		}

		// TODO: add interactive support
		if !c.confirmStop() {
			c.ui.Info(fmt.Sprintf("%s job %q aborted by user", helper.Title(stopOrDestroy), *job.ID))
			continue
		}

		// Build write options with namespace from the job template.
		// Only use job.Namespace if it was explicitly set in the template
		// (i.e., not the "default" that Canonicalize sets when no namespace is specified).
		// This allows --namespace flag to work when --var=namespace is not provided.
		writeOpts := &api.WriteOptions{}
		if job.Namespace != nil && *job.Namespace != "" && *job.Namespace != api.DefaultNamespace {
			writeOpts.Namespace = *job.Namespace
		}

		// Invoke the stop
		evalID, _, err := client.Jobs().DeregisterOpts(*job.ID, &api.DeregisterOptions{
			Purge:  c.purge,
			Global: c.global,
		}, writeOpts)
		if err != nil {
			errs = append(errs, err)
			c.ui.ErrorWithContext(err, fmt.Sprintf("error deregistering job: %q", *job.ID))
			continue
		}

		if evalID != "" {
			c.ui.Info(fmt.Sprintf("Evaluation %q submitted for job %q", evalID, *job.ID))
			evalIDs = append(evalIDs, evalID)
		}

		stoppedJobs = append(stoppedJobs, *job.Name)
	}

	// after all jobs are stopped, delete Nomad Variables if purging
	if c.purge && r.ParsedVariables() != nil {
		if err := deleteNomadVariables(r.ParsedVariables(), client, c.ui); err != nil {
			c.ui.ErrorWithContext(err, "failed to delete Nomad Variables", errorContext.GetAll()...)
			// don't return error - jobs are already stopped
		}
	}

	monitorExitCode := 0
	// Monitor all evaluations in parallel unless --detach is specified
	if !c.detach && len(evalIDs) > 0 {
		mon := newMonitor(c.Ctx, c.ui, client, c.lengthForVerbose())
		monitorExitCode = mon.monitor(evalIDs)

	}
	// Print success messages for stopped jobs
	for _, jobName := range stoppedJobs {
		c.ui.Success(fmt.Sprintf("Job %q %s", jobName, stoppedOrDestroyed))
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
	return monitorExitCode
}

func (c *StopCommand) checkForConflicts(client *api.Client, job *api.Job) error {
	// Only use job.Namespace if it was explicitly set in the template
	// (i.e., not the "default" that Canonicalize sets when no namespace is specified).
	// This allows --namespace flag to work when --var=namespace is not provided.
	queryOpts := &api.QueryOptions{}
	if job.Namespace != nil && *job.Namespace != "" && *job.Namespace != api.DefaultNamespace {
		queryOpts.Namespace = *job.Namespace
	}

	prefix := *job.ID
	queryOpts.Prefix = prefix
	jobsApi := client.Jobs()

	// List jobs matching the prefix and namespace. Multiple jobs may be returned
	// if the prefix matches multiple job IDs, or if the same job ID exists in
	// different namespaces when using '*' namespace. We error if the prefix doesn't
	// give an exact match or if the same job ID exists in multiple namespaces.
	jobs, _, err := jobsApi.List(queryOpts.WithContext(context.Background()))
	if err != nil {
		return fmt.Errorf("error querying job prefix %q: %s", prefix, err)
	}

	if len(jobs) == 0 {
		return fmt.Errorf("no job(s) with prefix or id %q found", *job.Name)
	}

	if len(jobs) > 1 {
		exactMatch := prefix == jobs[0].ID
		matchInMultipleNamespaces := c.allNamespaces() && jobs[0].ID == jobs[1].ID

		if !exactMatch || matchInMultipleNamespaces {
			return fmt.Errorf("prefix matched multiple jobs\n\n%s", createStatusListOutput(jobs, c.allNamespaces()))
		}
	}

	return nil
}

// TODO: Add interactive support
func (c *StopCommand) confirmStop() bool {
	// TODO: Confirm the stop if the job was a prefix match
	// TODO: Confirm we want to stop only a single region of a multiregion job
	return true
}

// lengthForVerbose returns the ID length to use based on verbose flag
func (c *StopCommand) lengthForVerbose() int {
	if c.verbose {
		return fullId
	}
	return shortId
}

func (c *StopCommand) Flags() *flag.Sets {
	return c.flagSet(flagSetOperation|flagSetNomadClient, func(set *flag.Sets) {
		c.packConfig = &caching.PackConfig{}

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

		f.BoolVar(&flag.BoolVar{
			Name:    "detach",
			Target:  &c.detach,
			Default: false,
			Usage: `Return immediately instead of monitoring the evaluation.
					A new evaluation ID will be output which can be used to
					examine the evaluation using "nomad eval status".`,
		})

		f.BoolVar(&flag.BoolVar{
			Name:    "verbose",
			Target:  &c.verbose,
			Default: false,
			Usage:   `Display full information during evaluation monitoring.`,
		})
	})
}

func (c *StopCommand) AutocompleteArgs() complete.Predictor {
	return predictPackName
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
