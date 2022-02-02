package job

import (
	"fmt"
	"os"
	"time"

	v1client "github.com/hashicorp/nomad-openapi/clients/go/v1"
	v1 "github.com/hashicorp/nomad-openapi/v1"
	"github.com/hashicorp/nomad-pack/internal/pkg/errors"
	intHelper "github.com/hashicorp/nomad-pack/internal/pkg/helper"
	"github.com/hashicorp/nomad-pack/internal/runner"
	"github.com/hashicorp/nomad-pack/sdk/helper"
	"github.com/hashicorp/nomad-pack/terminal"
)

// Runner is the job implementation of the runner.Runner interface.
type Runner struct {
	cfg       *CLIConfig
	runnerCfg *runner.Config

	// client and clientQueryOpts are used when calling the Nomad API.
	client          *v1.Client
	clientQueryOpts *v1.QueryOpts

	// rawTemplates contains the rendered templates from the renderer. Once
	// these have been parsed, store them within parsedTemplates, so we don't
	// have to do this again when deploying the jobs. There is no concurrent,
	// multi-routine access to these maps, so we don't need a lock.
	rawTemplates    map[string]string
	parsedTemplates map[string]*v1client.Job

	// deployedJobs tracks the jobs that have successfully been deployed to
	// Nomad so that in the event of a failure, we can attempt to rollback.
	deployedJobs []*v1client.Job
}

// NewDeployer returns the job implementation of deploy.Deployer. This is
// responsible for handling packs that contain job specifications.
//
// TODO(jrasell): design a nice method to have the QueryOpts setup once and
//  available to all subsystems that use a Nomad client.
func NewDeployer(client *v1.Client, cfg *CLIConfig) runner.Runner {
	return &Runner{
		client:          client,
		clientQueryOpts: newQueryOpts(),
		cfg:             cfg,
		rawTemplates:    make(map[string]string),
		parsedTemplates: make(map[string]*v1client.Job),
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
		r.handleConsulAndVault(jobSpec)
		r.setJobMeta(jobSpec)
	}

	return nil
}

// ParsedTemplates satisfies the GetParsedTemplates function of the
// runner.Runner interface.
func (r *Runner) ParsedTemplates() interface{} { return r.parsedTemplates }

// Name satisfies the Name function of the runner.Runner interface.
func (r *Runner) Name() string { return "job" }

// Deploy satisfies the Deploy function of the runner.Runner interface.
func (r *Runner) Deploy(ui terminal.UI, errorContext *errors.UIErrorContext) *errors.WrappedUIContext {

	for tplName, jobSpec := range r.parsedTemplates {

		// tplErrorContext forms the basis for error output context as is
		// appended to when new information becomes available.
		tplErrorContext := errorContext.Copy()
		tplErrorContext.Add(errors.UIContextPrefixTemplateName, tplName)

		registerOpts := v1.RegisterOpts{
			EnforceIndex:   r.cfg.RunConfig.CheckIndex > 0,
			ModifyIndex:    r.cfg.RunConfig.CheckIndex,
			PolicyOverride: r.cfg.RunConfig.PolicyOverride,
			PreserveCounts: r.cfg.RunConfig.PreserveCounts,
		}

		// Submit the job
		result, _, err := r.client.Jobs().Register(newWriteOptsFromJob(jobSpec).Ctx(), jobSpec, &registerOpts)
		if err != nil {
			r.rollback(ui)
			return generateRegisterError(intHelper.UnwrapAPIError(err), tplErrorContext, jobSpec.GetName())
		}

		// Print any warnings if there are any
		if result.Warnings != nil && *result.Warnings != "" {
			ui.Warning(fmt.Sprintf("Job Warnings:\n%s[reset]\n", *result.Warnings))
		}

		// Handle output formatting based on job configuration
		if r.client.Jobs().IsPeriodic(jobSpec) && !r.client.Jobs().IsParameterized(jobSpec) {
			r.handlePeriodicJobResponse(ui, jobSpec)
		} else if !r.client.Jobs().IsParameterized(jobSpec) {
			ui.Info(fmt.Sprintf("Evaluation ID: %s", *result.EvalID))
		}

		r.deployedJobs = append(r.deployedJobs, jobSpec)
		ui.Info(fmt.Sprintf("Job '%s' in pack deployment '%s' registered successfully",
			*jobSpec.ID, r.runnerCfg.DeploymentName))
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
		ui.Info(fmt.Sprintf("attempting rollback of job '%s'", *job.ID))
		_, _, err := r.client.Jobs().Delete(newWriteOptsFromJob(job).Ctx(), *job.ID, true, true)
		if err != nil {
			ui.ErrorWithContext(intHelper.UnwrapAPIError(err), fmt.Sprintf("rollback failed for job '%s'", *job.ID))
		} else {
			ui.Info(fmt.Sprintf("rollback of job '%s' succeeded", *job.ID))
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
func (r *Runner) handleConsulAndVault(job *v1client.Job) {

	// If the user didn't set a Consul token, check the environment to see if
	// there is one.
	if r.cfg.RunConfig.ConsulToken == "" {
		r.cfg.RunConfig.ConsulToken = os.Getenv("CONSUL_HTTP_TOKEN")
	}

	if r.cfg.RunConfig.ConsulToken != "" {
		job.ConsulToken = helper.StringToPtr(r.cfg.RunConfig.ConsulToken)
	}

	if r.cfg.RunConfig.ConsulNamespace != "" {
		job.ConsulNamespace = helper.StringToPtr(r.cfg.RunConfig.ConsulNamespace)
	}

	// If the user didn't set a Vault token, check the environment to see if
	// there is one.
	if r.cfg.RunConfig.VaultToken == "" {
		r.cfg.RunConfig.VaultToken = os.Getenv("VAULT_TOKEN")
	}

	if r.cfg.RunConfig.VaultToken != "" {
		job.VaultToken = helper.StringToPtr(r.cfg.RunConfig.VaultToken)
	}

	if r.cfg.RunConfig.VaultNamespace != "" {
		job.VaultNamespace = helper.StringToPtr(r.cfg.RunConfig.VaultNamespace)
	}
}

// determines next launch time and outputs to terminal
func (r *Runner) handlePeriodicJobResponse(ui terminal.UI, job *v1client.Job) {
	loc, err := r.client.Jobs().GetLocation(job)
	if err == nil {
		now := time.Now().In(loc)
		next, err := r.client.Jobs().Next(job.Periodic, now)
		if err != nil {
			ui.ErrorWithContext(intHelper.UnwrapAPIError(err), "failed to determine next launch time")
		} else {
			ui.Warning(fmt.Sprintf("Approximate next launch time: %s (%s from now)",
				formatTime(&next), formatTimeDifference(now, next, time.Second)))
		}
	}
}

// ParseTemplates satisfies the ParseTemplates function of the deploy.Deployer
// interface.
func (r *Runner) ParseTemplates() []*errors.WrappedUIContext {

	// outputErrors collects all encountered error during the validation run.
	var outputErrors []*errors.WrappedUIContext

	for tplName, tpl := range r.rawTemplates {

		job, err := r.client.Jobs().Parse(r.clientQueryOpts.Ctx(), tpl, true, r.cfg.RunConfig.HCL1)
		if err != nil {
			outputErrors = append(outputErrors, newValidationDeployerError(intHelper.UnwrapAPIError(err), validationSubjParseFailed, tplName))
			continue
		}

		// Store the parsed job file. This means we do not have to do this
		// again when moving onto the actual deployment.
		r.parsedTemplates[tplName] = job
	}

	return outputErrors
}
