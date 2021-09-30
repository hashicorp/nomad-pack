package job

import (
	"fmt"
	"os"
	"time"

	"github.com/hashicorp/nom/internal/deploy"
	"github.com/hashicorp/nom/internal/pkg/errors"
	"github.com/hashicorp/nom/terminal"
	v1client "github.com/hashicorp/nomad-openapi/clients/go/v1"
	v1 "github.com/hashicorp/nomad-openapi/v1"
	"github.com/hashicorp/nomad/helper"
)

// Deployer is the job implementation of the deploy.Deployer interface.
type Deployer struct {
	cfg         *CLIConfig
	deployerCfg *deploy.DeployerConfig

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
func NewDeployer(client *v1.Client, cfg *CLIConfig) deploy.Deployer {
	return &Deployer{
		client:          client,
		clientQueryOpts: &v1.QueryOpts{},
		cfg:             cfg,
		rawTemplates:    make(map[string]string),
		parsedTemplates: make(map[string]*v1client.Job),
	}
}

// CanonicalizeTemplates satisfies the CanonicalizeTemplates function of the
// deploy.Deployer interface.
func (d *Deployer) CanonicalizeTemplates() []*deploy.DeployerError {

	if len(d.parsedTemplates) < 1 {
		if err := d.ParseTemplates(); err != nil {
			return err
		}
	}

	for _, jobSpec := range d.parsedTemplates {
		d.handleConsulAndVault(jobSpec)
		d.setJobMeta(jobSpec)
	}

	return nil
}

// GetParsedTemplates satisfies the GetParsedTemplates function of the
// deploy.Deployer interface.
func (d *Deployer) GetParsedTemplates() interface{} { return d.parsedTemplates }

// Name satisfies the Name function of the deploy.Deployer interface.
func (d *Deployer) Name() string { return "job" }

// Deploy satisfies the Deploy function of the deploy.Deployer interface.
func (d *Deployer) Deploy(ui terminal.UI, errorContext *errors.UIErrorContext) *deploy.DeployerError {

	for tplName, jobSpec := range d.parsedTemplates {

		// tplErrorContext forms the basis for error output context as is
		// appended to when new information becomes available.
		tplErrorContext := errorContext.Copy()
		tplErrorContext.Add(errors.UIContextPrefixTemplateName, tplName)

		registerOpts := v1.RegisterOpts{
			EnforceIndex:   d.cfg.RunConfig.CheckIndex > 0,
			ModifyIndex:    d.cfg.RunConfig.CheckIndex,
			PolicyOverride: d.cfg.RunConfig.PolicyOverride,
			PreserveCounts: d.cfg.RunConfig.PreserveCounts,
		}

		// Submit the job
		result, _, err := d.client.Jobs().Register(newWriteOptsFromJob(jobSpec).Ctx(), jobSpec, &registerOpts)
		if err != nil {
			d.rollback(ui)
			return generateRegisterError(err, tplErrorContext, jobSpec.GetName())
		}

		// Print any warnings if there are any
		if result.Warnings != nil && *result.Warnings != "" {
			ui.Warning(fmt.Sprintf("Job Warnings:\n%s[reset]\n", *result.Warnings))
		}

		// Handle output formatting based on job configuration
		if d.client.Jobs().IsPeriodic(jobSpec) && !d.client.Jobs().IsParameterized(jobSpec) {
			d.handlePeriodicJobResponse(ui, jobSpec)
		} else if !d.client.Jobs().IsParameterized(jobSpec) {
			ui.Info(fmt.Sprintf("Evaluation ID: %s", *result.EvalID))
		}

		d.deployedJobs = append(d.deployedJobs, jobSpec)
		ui.Info(fmt.Sprintf("Job '%s' in pack deployment '%s' registered successfully",
			*jobSpec.ID, d.deployerCfg.DeploymentName))
	}

	return nil
}

// rollback begins a thought experiment about how to handle failures. It is not
// targeted for the initial release, but will be plumbed for experimentation.
// The flag is currently hidden and defaults to false.
func (d *Deployer) rollback(ui terminal.UI) {

	if !d.cfg.RunConfig.EnableRollback {
		return
	}

	ui.Info("attempting rollback...")

	for _, job := range d.deployedJobs {
		ui.Info(fmt.Sprintf("attempting rollback of job '%s'", *job.ID))
		_, _, err := d.client.Jobs().Delete(newWriteOptsFromJob(job).Ctx(), *job.ID, true, true)
		if err != nil {
			ui.ErrorWithContext(err, fmt.Sprintf("rollback failed for job '%s'", *job.ID))
		} else {
			ui.Info(fmt.Sprintf("rollback of job '%s' succeeded", *job.ID))
		}
	}
}

// SetDeploymentConfig satisfies the SetDeploymentConfig function of the
// deploy.Deployer interface.
func (d *Deployer) SetDeploymentConfig(cfg *deploy.DeployerConfig) { d.deployerCfg = cfg }

// SetTemplates satisfies the SetTemplates function of the deploy.Deployer
// interface.
func (d *Deployer) SetTemplates(templates map[string]string) {
	d.rawTemplates = templates
}

// handles resolving Consul and Vault options overrides with environment
// variables, if present, and then set the values on the job instance.
func (d *Deployer) handleConsulAndVault(job *v1client.Job) {

	// If the user didn't set a Consul token, check the environment to see if
	// there is one.
	if d.cfg.RunConfig.ConsulToken == "" {
		d.cfg.RunConfig.ConsulToken = os.Getenv("CONSUL_HTTP_TOKEN")
	}

	if d.cfg.RunConfig.ConsulToken != "" {
		job.ConsulToken = helper.StringToPtr(d.cfg.RunConfig.ConsulToken)
	}

	if d.cfg.RunConfig.ConsulNamespace != "" {
		job.ConsulNamespace = helper.StringToPtr(d.cfg.RunConfig.ConsulNamespace)
	}

	// If the user didn't set a Vault token, check the environment to see if
	// there is one.
	if d.cfg.RunConfig.VaultToken == "" {
		d.cfg.RunConfig.VaultToken = os.Getenv("VAULT_TOKEN")
	}

	if d.cfg.RunConfig.VaultToken != "" {
		job.VaultToken = helper.StringToPtr(d.cfg.RunConfig.VaultToken)
	}

	if d.cfg.RunConfig.VaultNamespace != "" {
		job.VaultNamespace = helper.StringToPtr(d.cfg.RunConfig.VaultNamespace)
	}
}

// determines next launch time and outputs to terminal
func (d *Deployer) handlePeriodicJobResponse(ui terminal.UI, job *v1client.Job) {
	loc, err := d.client.Jobs().GetLocation(job)
	if err == nil {
		now := time.Now().In(loc)
		next, err := d.client.Jobs().Next(job.Periodic, now)
		if err != nil {
			ui.ErrorWithContext(err, "failed to determine next launch time")
		} else {
			ui.Warning(fmt.Sprintf("Approximate next launch time: %s (%s from now)",
				formatTime(&next), formatTimeDifference(now, next, time.Second)))
		}
	}
}

// ParseTemplates satisfies the ParseTemplates function of the deploy.Deployer
// interface.
func (d *Deployer) ParseTemplates() []*deploy.DeployerError {

	// outputErrors collects all encountered error during the validation run.
	var outputErrors []*deploy.DeployerError

	for tplName, tpl := range d.rawTemplates {

		job, err := d.client.Jobs().Parse(d.clientQueryOpts.Ctx(), tpl, true, d.cfg.RunConfig.HCL1)
		if err != nil {
			outputErrors = append(outputErrors, newValidationDeployerError(err, validationSubjParseFailed, tplName))
			continue
		}

		// Store the parsed job file. This means we do not have to do this
		// again when moving onto the actual deployment.
		d.parsedTemplates[tplName] = job
	}

	return outputErrors
}
