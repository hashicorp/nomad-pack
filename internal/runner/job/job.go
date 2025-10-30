// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package job

import (
	"fmt"
	"regexp"
	"time"

	"github.com/hashicorp/nomad/api"

	"github.com/hashicorp/nomad-pack/internal/pkg/errors"
	"github.com/hashicorp/nomad-pack/internal/runner"
	"github.com/hashicorp/nomad-pack/terminal"
)

// Runner is the job implementation of the runner.Runner interface.
type Runner struct {
	cfg       *CLIConfig
	runnerCfg *runner.Config

	// client is used when calling the Nomad API.
	client *api.Client

	// rawTemplates contains the rendered templates from the renderer. Once
	// these have been parsed, store them within parsedTemplates, so we don't
	// have to do this again when deploying the jobs. There is no concurrent,
	// multi-routine access to these maps, so we don't need a lock.
	rawTemplates    map[string]string
	parsedTemplates map[string]ParsedTemplate

	// deployedJobs tracks the jobs that have successfully been deployed to
	// Nomad so that in the event of a failure, we can attempt to rollback.
	deployedJobs []ParsedTemplate
}

type ParsedTemplate struct {
	original  *api.Job
	canonical *api.Job
}

func (p *ParsedTemplate) GetName() string {
	return *p.canonical.Name
}

func (p *ParsedTemplate) HasRegion() bool {
	return p.original.Region != nil
}

func (p *ParsedTemplate) HasNamespace() bool {
	return p.original.Namespace != nil
}

func (p *ParsedTemplate) Job() *api.Job {
	return p.canonical
}

// NewDeployer returns the job implementation of deploy.Deployer. This is
// responsible for handling packs that contain job specifications.
//
// TODO(jrasell): design a nice method to have the QueryOpts setup once and
// available to all subsystems that use a Nomad client.
func NewDeployer(client *api.Client, cfg *CLIConfig) runner.Runner {
	return &Runner{
		client:          client,
		cfg:             cfg,
		rawTemplates:    make(map[string]string),
		parsedTemplates: make(map[string]ParsedTemplate),
	}
}

// CanonicalizeTemplates satisfies the CanonicalizeTemplates function of the
// runner.Runner interface.
func (r *Runner) CanonicalizeTemplates() []*errors.WrappedUIContext {

	if len(r.parsedTemplates) < 1 {
		if err := r.ParseTemplates(); err != nil {
			return err
		}
	}

	return nil
}

// ParsedTemplates satisfies the GetParsedTemplates function of the
// runner.Runner interface.
func (r *Runner) ParsedTemplates() any { return r.parsedTemplates }

// Name satisfies the Name function of the runner.Runner interface.
func (r *Runner) Name() string { return "job" }

// Deploy satisfies the Deploy function of the runner.Runner interface.
func (r *Runner) Deploy(ui terminal.UI, errorContext *errors.UIErrorContext) *errors.WrappedUIContext {

	for tplName, jobSpec := range r.parsedTemplates {

		// tplErrorContext forms the basis for error output context as is
		// appended to when new information becomes available.
		tplErrorContext := errorContext.Copy()
		tplErrorContext.Add(errors.UIContextPrefixTemplateName, tplName)

		// submit the source of the job to Nomad, too
		submission := &api.JobSubmission{
			Source: r.rawTemplates[tplName],
			Format: "hcl2",
		}

		registerOpts := api.RegisterOptions{
			EnforceIndex:      r.cfg.RunConfig.CheckIndex > 0,
			ModifyIndex:       r.cfg.RunConfig.CheckIndex,
			PolicyOverride:    r.cfg.RunConfig.PolicyOverride,
			PreserveCounts:    r.cfg.RunConfig.PreserveCounts,
			PreserveResources: r.cfg.RunConfig.PreserveResources,
			Submission:        submission,
		}

		// Submit the job
		result, _, err := r.client.Jobs().RegisterOpts(jobSpec.Job(), &registerOpts, r.newWriteOptsFromJob(jobSpec))
		if err != nil {
			r.rollback(ui)
			return generateRegisterError(err, tplErrorContext, jobSpec.GetName())
		}

		// Print any warnings if there are any
		if result.Warnings != "" {
			ui.Warning(fmt.Sprintf("Job Warnings:\n%s[reset]\n", result.Warnings))
		}

		// Handle output formatting based on job configuration
		if jobSpec.Job().IsPeriodic() && !jobSpec.Job().IsParameterized() {
			r.handlePeriodicJobResponse(ui, jobSpec.Job())
		} else if !jobSpec.Job().IsParameterized() {
			ui.Info(fmt.Sprintf("Evaluation ID: %s", result.EvalID))
		}

		r.deployedJobs = append(r.deployedJobs, jobSpec)
		ui.Info(fmt.Sprintf("Job '%s' in pack deployment '%s' registered successfully",
			*jobSpec.Job().ID, r.runnerCfg.DeploymentName))
	}

	return nil
}

// rollback begins a thought experiment about how to handle failures. It is not
// targeted for the initial release, but will be plumbed for experimentation.
// The flag is currently hidden and defaults to false.
func (r *Runner) rollback(ui terminal.UI) {

	if !r.cfg.RunConfig.EnableRollback {
		return
	}

	ui.Info("attempting rollback...")

	for _, job := range r.deployedJobs {
		ui.Info(fmt.Sprintf("attempting rollback of job '%s'", *job.Job().ID))
		_, _, err := r.client.Jobs().DeregisterOpts(*job.Job().ID, &api.DeregisterOptions{Purge: true, Global: true}, r.newWriteOptsFromJob(job))
		if err != nil {
			ui.ErrorWithContext(err, fmt.Sprintf("rollback failed for job '%s'", *job.Job().ID))
		} else {
			ui.Info(fmt.Sprintf("rollback of job '%s' succeeded", *job.Job().ID))
		}
	}
}

// SetRunnerConfig satisfies the SetRunnerConfig function of the runner.Runner
// interface.
func (r *Runner) SetRunnerConfig(cfg *runner.Config) { r.runnerCfg = cfg }

// SetTemplates satisfies the SetTemplates function of the runner.Runner
// interface.
func (r *Runner) SetTemplates(templates map[string]string) {
	for n, tpl := range templates {
		r.rawTemplates[n] = r.setHCLMeta(tpl)
	}
}

// determines next launch time and outputs to terminal
func (r *Runner) handlePeriodicJobResponse(ui terminal.UI, job *api.Job) {
	if job.Periodic != nil && job.Periodic.TimeZone != nil {
		loc, err := time.LoadLocation(*job.Periodic.TimeZone)
		if err != nil {
			now := time.Now().In(loc)
			next, err := job.Periodic.Next(now)
			if err != nil {
				ui.ErrorWithContext(err, "failed to determine next launch time")
			} else {
				ui.Warning(fmt.Sprintf("Approximate next launch time: %s (%s from now)",
					formatTime(next), formatTimeDifference(now, next, time.Second)))
			}
		}
	}
}

// ParseTemplates satisfies the ParseTemplates function of the deploy.Deployer
// interface.
func (r *Runner) ParseTemplates() []*errors.WrappedUIContext {
	// outputErrors collects all encountered error during the validation run.
	var outputErrors []*errors.WrappedUIContext

	for tplName, tpl := range r.rawTemplates {
		// if a template contains region or namespace information, it needs to be passed
		// to the client before calling the parse methods, otherwise they might fail in
		// case ACL restricts our permissions

		// Remove template blocks (data = <<EOF ... EOF) before checking for namespace/region
		// to avoid false positives from heredoc content.
		// NOTE: This does not do proper HEREDOC parsing, as we cannot use back-references
		// to match the word after `<<`, e.g. 'EOF'.
		templateBlockRe := regexp.MustCompile(`(?s)template\s*\{.*?data\s*=\s*<<-?\w+\n.*?\n\s*\w+\s*\n.*?\}`)
		tplFiltered := templateBlockRe.ReplaceAllString(tpl, "")

		namespaceRe := regexp.MustCompile(`(?m)namespace = \"([\w-]+)`)
		regionRe := regexp.MustCompile(`(?m)region = \"([\w-]+)`)

		if nsRes := namespaceRe.FindStringSubmatch(tplFiltered); len(nsRes) > 1 {
			r.client.SetNamespace(nsRes[1])
		}
		if regRes := regionRe.FindStringSubmatch(tplFiltered); len(regRes) > 1 {
			r.client.SetRegion(regRes[1])
		}

		ncJob, err := r.client.Jobs().ParseHCLOpts(&api.JobsParseRequest{
			JobHCL:       tpl,
			Canonicalize: false,
		})
		if err != nil {
			outputErrors = append(
				outputErrors,
				newValidationDeployerError(err, validationSubjParseFailed, tplName),
			)
			continue
		}

		job, err := r.client.Jobs().ParseHCLOpts(&api.JobsParseRequest{
			JobHCL:       tpl,
			Canonicalize: true,
		})
		if err != nil {
			outputErrors = append(
				outputErrors,
				newValidationDeployerError(err, validationSubjParseFailed, tplName),
			)
			continue
		}

		// Store the parsed job file. This means we do not have to do this
		// again when moving onto the actual deployment. Keeping the original
		// and the canonicalized version of the job allows us to inspect the
		// original spec's region and namespace.

		// This could probably be a leaner object, but this will provide the
		// highest resolution view of the original parsed job.
		r.parsedTemplates[tplName] = ParsedTemplate{
			original:  ncJob,
			canonical: job,
		}
	}

	return outputErrors
}
