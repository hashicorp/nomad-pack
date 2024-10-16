// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package job

import (
	"fmt"
	"os"
	"regexp"
	"time"

	"github.com/hashicorp/nomad/api"

	"github.com/hashicorp/nomad-pack/internal/pkg/errors"
	"github.com/hashicorp/nomad-pack/internal/pkg/helper/pointer"
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

	for _, jobSpec := range r.parsedTemplates {
		r.handleConsulAndVault(jobSpec.Job())
		r.setJobMeta(jobSpec.Job())
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
			EnforceIndex:   r.cfg.RunConfig.CheckIndex > 0,
			ModifyIndex:    r.cfg.RunConfig.CheckIndex,
			PolicyOverride: r.cfg.RunConfig.PolicyOverride,
			PreserveCounts: r.cfg.RunConfig.PreserveCounts,
			Submission:     submission,
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
	r.rawTemplates = templates
}

// handles resolving Consul and Vault options overrides with environment
// variables, if present, and then set the values on the job instance.
func (r *Runner) handleConsulAndVault(job *api.Job) {

	// If the user didn't set a Consul token, check the environment to see if
	// there is one.
	if r.cfg.RunConfig.ConsulToken == "" {
		r.cfg.RunConfig.ConsulToken = os.Getenv("CONSUL_HTTP_TOKEN")
	}

	if r.cfg.RunConfig.ConsulToken != "" {
		job.ConsulToken = pointer.Of(r.cfg.RunConfig.ConsulToken)
	}

	if r.cfg.RunConfig.ConsulNamespace != "" {
		job.ConsulNamespace = pointer.Of(r.cfg.RunConfig.ConsulNamespace)
	}

	// If the user didn't set a Vault token, check the environment to see if
	// there is one.
	if r.cfg.RunConfig.VaultToken == "" {
		r.cfg.RunConfig.VaultToken = os.Getenv("VAULT_TOKEN")
	}

	if r.cfg.RunConfig.VaultToken != "" {
		job.VaultToken = pointer.Of(r.cfg.RunConfig.VaultToken)
	}

	if r.cfg.RunConfig.VaultNamespace != "" {
		job.VaultNamespace = pointer.Of(r.cfg.RunConfig.VaultNamespace)
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
		namespaceRe := regexp.MustCompile(`(?m)namespace = \"(\w+)`)
		regionRe := regexp.MustCompile(`(?m)region = \"(\w+)`)

		if nsRes := namespaceRe.FindStringSubmatch(tpl); len(nsRes) > 1 {
			r.client.SetNamespace(nsRes[1])
		}

		if regRes := regionRe.FindStringSubmatch(tpl); len(regRes) > 1 {
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
