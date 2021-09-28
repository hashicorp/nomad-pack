package cli

import (
	"fmt"
	"os"
	"regexp"
	"strings"
	"time"

	"github.com/hashicorp/nom/flag"
	"github.com/hashicorp/nom/internal/pkg/errors"
	"github.com/hashicorp/nom/internal/pkg/version"
	v1client "github.com/hashicorp/nomad-openapi/clients/go/v1"
	v1 "github.com/hashicorp/nomad-openapi/v1"
	"github.com/hashicorp/nomad/helper"
)

type RunCommand struct {
	*baseCommand
	packName          string
	repoName          string
	deploymentName    string
	checkIndex        uint64
	consulToken       string
	consulNamespace   string
	vaultToken        string
	vaultNamespace    string
	enableRollback    bool
	hcl1              bool
	preserveCounts    bool
	policyOverride    bool
	deploymentExists  bool
	processedPackJobs []*v1client.Job
	Validation        ValidationFn
}

func (c *RunCommand) Run(args []string) int {
	c.cmdKey = "run" // Add cmd key here so help text is available in Init
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


	repoPath, err := getRepoPath(repoName, c.ui, errorContext)
	if err != nil {
		return 1
	}

	// Add the path to the pack on the error context.
	errorContext.Add(errors.UIContextPrefixPackPath, repoPath)

	// verify packs exist before running jobs
	if err = verifyPackExist(c.ui, c.packName, repoPath, errorContext); err != nil {
		return 1
	}

	// get pack git version
	// TODO: Get this from pack metadata.
	packVersion, err := version.PackVersion(repoPath)
	if err != nil {
		c.ui.ErrorWithContext(err, "failed to determine pack version", errorContext.GetAll()...)
	}

	// Add the path to the pack on the error context.
	errorContext.Add(errors.UIContextPrefixPackVersion, packVersion)

	// If no deploymentName set default to pack@version
	c.deploymentName = getDeploymentName(c.baseCommand, c.packName, packVersion)
	errorContext.Add(errors.UIContextPrefixDeploymentName, c.deploymentName)

	// create the http client
	client, err := v1.NewClient()
	if err != nil {
		c.ui.ErrorWithContext(err, "failed to initialize client", errorContext.GetAll()...)
		return 1
	}

	packManager := generatePackManager(c.baseCommand, client, repoPath, c.packName)

	// render the pack
	r, err := renderPack(packManager, c.baseCommand.ui, errorContext)
	if err != nil {
		return 255
	}

	// Commands that render templates are required to render at least one
	// parent template.
	if r.LenParentRenders() < 1 {
		c.ui.ErrorWithContext(errors.ErrNoTemplatesRendered, "no templates rendered", errorContext.GetAll()...)
		return 1
	}

	// set a timestamp to be set on all pack elements
	timestamp := time.Now().UTC().String()

	// set a local variable to the JobsApi
	jobsApi := client.Jobs()

	for tplName, tpl := range r.ParentRenders() {

		// tplErrorContext forms the basis for error output context as is
		// appended to when new information becomes available.
		tplErrorContext := errorContext.Copy()
		tplErrorContext.Add(errors.UIContextPrefixTemplateName, tplName)

		// get job struct from template
		// TODO: Should we add an hcl1 flag?
		job, err := parseJob(c.ui, tpl, false, tplErrorContext)
		if err != nil {
			c.rollback(jobsApi)
			return 1
		}

		// Add the jobID to the error context.
		tplErrorContext.Add(errors.UIContextPrefixJobName, job.GetName())

		// check to see if job already exists
		err = c.checkForConflict(jobsApi, job)
		if err != nil {
			c.ui.ErrorWithContext(err, "job conflict", tplErrorContext.GetAll()...)
			c.rollback(jobsApi)
			return 1
		}

		// Set Consul and Vault tokens
		c.handleConsulAndVault(job)

		// Set job metadata
		c.setJobMeta(job, timestamp, packVersion)

		// Submit the job
		result, _, err := jobsApi.Register(newWriteOptsFromJob(job).Ctx(), job, &v1.RegisterOpts{
			EnforceIndex:   c.checkIndex > 0,
			ModifyIndex:    c.checkIndex,
			PolicyOverride: c.policyOverride,
			PreserveCounts: c.preserveCounts,
		})
		if err != nil {
			i, done := c.handleRegisterError(err, tplErrorContext)
			if done {
				c.rollback(jobsApi)
				return i
			}
			c.rollback(jobsApi)
			return 1
		}

		// Print any warnings if there are any
		if result.Warnings != nil && *result.Warnings != "" {
			c.ui.Warning(fmt.Sprintf("Job Warnings:\n%s[reset]\n", *result.Warnings))
		}

		// Handle output formatting based on job configuration
		if jobsApi.IsPeriodic(job) && !jobsApi.IsParameterized(job) {
			c.handlePeriodicJobResponse(jobsApi, job)
		} else if !jobsApi.IsParameterized(job) {
			c.ui.Info(fmt.Sprintf("Evaluation ID: %s", *result.EvalID))
		}

		c.processedPackJobs = append(c.processedPackJobs, job)
		c.ui.Info(fmt.Sprintf("Job '%s' in pack deployment '%s' registered successfully", *job.ID, c.deploymentName))
	}

	c.ui.Success(fmt.Sprintf("Pack successfully deployed. Use --name=%s to manage this this deployed instance with run, plan, or destroy", c.deploymentName))

	output, err := packManager.ProcessOutputTemplate()
	if err != nil {
		c.ui.ErrorWithContext(err, "failed to render output template", "Pack Name: "+c.packName)
		return 1
	}

	if output != "" {
		c.ui.Output(fmt.Sprintf("\n%s", output))
	}
	return 0
}

// rollback begins a thought experiment about how to handle failures. It is not
// targeted for the initial release, but will be plumbed for experimentation. The
// flag is currently hidden and defaults to false.
func (c *RunCommand) rollback(jobsApi *v1.Jobs) {
	if !c.enableRollback {
		return
	}

	c.ui.Info("attempting rollback...")

	for _, job := range c.processedPackJobs {
		c.ui.Info(fmt.Sprintf("attempting rollback of job '%s'", *job.ID))
		_, _, err := jobsApi.Delete(newWriteOptsFromJob(job).Ctx(), *job.ID, true, true)
		if err != nil {
			c.ui.ErrorWithContext(err, fmt.Sprintf("rollback failed for job '%s'", *job.ID))
		} else {
			c.ui.Info(fmt.Sprintf("rollback of job '%s' succeeded", *job.ID))
		}
	}
}

func (c *RunCommand) checkForConflict(jobsApi *v1.Jobs, job *v1client.Job) error {
	existing, _, err := getJob(jobsApi, *job.Name, newQueryOptsFromJob(job))
	if err != nil {
		openAPIErr, ok := err.(v1client.GenericOpenAPIError)
		if !ok || string(openAPIErr.Body()) != "job not found" {
			return fmt.Errorf("error checking if job '%s' already exists: %s", *job.ID, err)
		}
	}

	// If no existing job, no possible error condition.
	if existing == nil {
		return nil
	}

	// if there is a job with this name, that has no meta, it was
	// created by something other than the package manager and this
	// process should fail.
	if existing.Meta == nil {
		return fmt.Errorf("job with id '%s' already exists and is not manage by nomad pack", *existing.ID)
	}

	meta := *existing.Meta
	existingDeploymentName, ok := meta[packDeploymentNameKey]
	// if there is a job with this ID, that has no pack-deployment-name meta, it was
	// created by something other than the package manager and this process should abort.
	if !ok {
		return fmt.Errorf("job with id '%s' already exists and is not manage by nomad pack", *existing.ID)
	}

	// If there is a job with this ID, and a different deployment name, this process should abort.
	if existingDeploymentName != c.deploymentName {
		return fmt.Errorf("job with id '%s' already exists and is part of deployment '%s'", *existing.ID, existingDeploymentName)
	}

	// If the job exists with the same deployment name, inform the user that
	// existing allocations will be cycled.
	c.ui.Info(fmt.Sprintf("Updating job with id '%s' . Existing job allocations will be updated. No new job created.", *existing.ID))

	return nil
}

// determines next launch time and outputs to terminal
func (c *RunCommand) handlePeriodicJobResponse(jobsApi *v1.Jobs, job *v1client.Job) {
	loc, err := jobsApi.GetLocation(job)
	if err == nil {
		now := time.Now().In(loc)
		next, err := jobsApi.Next(job.Periodic, now)
		if err != nil {
			c.ui.ErrorWithContext(err, "error determining next launch time")
		} else {
			c.ui.Warning(fmt.Sprintf("Approximate next launch time: %s (%s from now)",
				formatTime(&next), formatTimeDifference(now, next, time.Second)))
		}
	}
}

// formats and prints registration error output
func (c *RunCommand) handleRegisterError(err error, errCtx *errors.UIErrorContext) (int, bool) {
	if strings.Contains(err.Error(), v1.RegisterEnforceIndexErrPrefix) {
		// Format the error specially if the error is due to index
		// enforcement
		matches := enforceIndexRegex.FindStringSubmatch(err.Error())
		if len(matches) == 2 {
			c.ui.Warning(matches[1]) // The matched group
			c.ui.Warning("Job not updated")
			return 1, true
		}
	}

	c.ui.ErrorWithContext(err, "failed to register job", errCtx.GetAll()...)
	return 0, false
}

// add metadata to the job for in cluster querying and management
func (c *RunCommand) setJobMeta(job *v1client.Job, timestamp string, packVersion string) {
	jobMeta := make(map[string]string)

	// If current job meta isn't nil, use that instead
	if job.Meta != nil {
		jobMeta = *job.Meta
	}

	// Add the Nomad Pack custom metadata.
	jobMeta[packKey], _ = getPackPath(c.repoName, c.packName)
	jobMeta[packDeploymentNameKey] = c.deploymentName
	jobMeta[packJobKey] = *job.Name
	jobMeta[packDeploymentTimestampKey] = timestamp
	jobMeta[packVersionKey] = packVersion

	// Replace the job metadata with our modified version.
	job.Meta = &jobMeta
}

// handles resolving Consul and Vault options overrides with environment variables,
// if present, and then set the values on the job instance.
func (c *RunCommand) handleConsulAndVault(job *v1client.Job) {
	// Parse the Consul token
	if c.consulToken == "" {
		// Check the environment variable
		c.consulToken = os.Getenv("CONSUL_HTTP_TOKEN")
	}

	if c.consulToken != "" {
		job.ConsulToken = helper.StringToPtr(c.consulToken)
	}

	if c.consulNamespace != "" {
		job.ConsulNamespace = helper.StringToPtr(c.consulNamespace)
	}

	// Parse the Vault token
	if c.vaultToken == "" {
		// Check the environment variable
		c.vaultToken = os.Getenv("VAULT_TOKEN")
	}

	if c.vaultToken != "" {
		job.VaultToken = helper.StringToPtr(c.vaultToken)
	}

	if c.vaultNamespace != "" {
		job.VaultNamespace = helper.StringToPtr(c.vaultNamespace)
	}
}

// Flags defines the flag.Sets for the operation.
func (c *RunCommand) Flags() *flag.Sets {
	return c.flagSet(flagSetOperation, func(set *flag.Sets) {
		f := set.NewSet("Run Options")

		f.Uint64Var(&flag.Uint64Var{
			Name:    "check-index",
			Target:  &c.checkIndex,
			Default: 0,
			Usage: `If set, the job is only registered or updated if the passed 
                   job modify index matches the server side version. If a check-index
                   value of zero is passed, the job is only registered if it does
                   not yet exist. If a non-zero value is passed, it ensures that
                   the job is being updated from a known state. The use of this
                   flag is most common in conjunction with job plan command.`,
		})

		f.StringVar(&flag.StringVar{
			Name:    "consul-token",
			Target:  &c.consulToken,
			Default: "",
			Usage: `If set, the passed Consul token is stored in the job before
                      sending to the Nomad servers. This allows passing the Consul
                      token without storing it in the job file. This overrides the
                      token found in the $CONSUL_HTTP_TOKEN environment variable
                      and that found in the job.`,
		})

		f.StringVar(&flag.StringVar{
			Name:    "consul-namespace",
			Target:  &c.consulNamespace,
			Default: "",
			Usage: `If set, any services in the job will be registered into the 
                    specified Consul namespace. Any template stanza reading from 
                    Consul KV will be scoped to the the specified Consul namespace. 
                    If Consul ACLs are enabled and the allow_unauthenticated Nomad 
                    server Consul configuration is not enabled, then a Consul token 
                    must be supplied with appropriate service and kv Consul ACL 
                    policy permissions.`,
		})

		f.StringVar(&flag.StringVar{
			Name:    "vault-token",
			Target:  &c.vaultToken,
			Default: "",
			Usage: `If set, the passed Vault token is stored in the job before
                      sending to the Nomad servers. This allows passing the Vault 
                      token without storing it in the job file. This overrides the 
                      token found in the $VAULT_TOKEN environment variable and 
                      that found in the job.`,
		})

		f.StringVar(&flag.StringVar{
			Name:    "vault-namespace",
			Target:  &c.vaultNamespace,
			Default: "",
			Usage: `If set, the passed Vault namespace is stored in the job before 
                    sending to the Nomad servers.`,
		})

		f.BoolVar(&flag.BoolVar{
			Name:    "policy-override",
			Target:  &c.policyOverride,
			Default: false,
			Usage: `Sets the flag to force override any soft mandatory Sentinel 
                      policies.`,
		})

		f.BoolVar(&flag.BoolVar{
			Name:    "preserve-counts",
			Target:  &c.preserveCounts,
			Default: false,
			Usage: `If set, the existing task group counts will be preserved 
                      when updating a job.`,
		})

		f.BoolVar(&flag.BoolVar{
			Name:    "hcl1",
			Target:  &c.hcl1,
			Default: false,
			Usage:   `If set, the hcl V1 parser will be used to parse the job file.`,
		})

		f.BoolVar(&flag.BoolVar{
			Name:    "rollback",
			Hidden:  true,
			Target:  &c.enableRollback,
			Default: false,
			Usage: `EXPERIMENTAL. If set, any pack failure will cause nomad pack
                       to attempt to rollback the entire deployment.`,
		})
	})
}

func (c *RunCommand) Help() string {
	// TODO: do we want to version example? Have another different example that has
	// a version num instead of git sha?
	c.Example = `
	# Run an example pack with the default deployment name "example@86a9235" (default is <pack-name>@version)
	nomad-pack run example

	# Run an example pack with the specified deployment name "dev"
	nomad-pack run example --name=dev 

	# Run an example pack with override variables in a variable file
	nomad-pack run example --var-file="./overrides.hcl"

	# Run an example pack with cli variable overrides
	nomad-pack run example --var="redis_image_version=latest" --var="redis_resources={"cpu": "1000", "memory": "512"}"
	`

	return formatHelp(`
	Usage: nomad-pack run <pack-name> [options]

	Install the specified Nomad Pack to a configured Nomad cluster.

` + c.GetExample() + c.Flags().Help())
}

// Synopsis satisfies the Synopsis function of the cli.Command interface.
func (c *RunCommand) Synopsis() string {
	return "Run a new pack or update an existing pack"
}

var (
	// enforceIndexRegex is a regular expression which extracts the enforcement error
	enforceIndexRegex = regexp.MustCompile(`\((Enforcing job modify index.*)\)`)
)

var (
	packKey                    = "pack"
	packDeploymentNameKey      = "pack-deployment-name"
	packJobKey                 = "pack-job"
	packDeploymentTimestampKey = "pack-deployment-timestamp"
	packVersionKey             = "pack-version"
)
