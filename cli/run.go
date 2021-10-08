package cli

import (
	"fmt"
	"path"

	v1 "github.com/hashicorp/nomad-openapi/v1"
	"github.com/hashicorp/nomad-pack/flag"
	"github.com/hashicorp/nomad-pack/internal/pkg/errors"
	"github.com/hashicorp/nomad-pack/internal/pkg/registry"
	"github.com/hashicorp/nomad-pack/internal/runner"
	"github.com/hashicorp/nomad-pack/internal/runner/job"
	"github.com/posener/complete"
)

type RunCommand struct {
	*baseCommand
	packName     string
	packVersion  string
	registryName string
	jobConfig    *job.CLIConfig
	Validation   ValidationFn
}

func (c *RunCommand) Run(args []string) int {
	var err error
	c.cmdKey = "run" // Add cmdKey here to print out helpUsageMessage on Init error
	// Initialize. If we fail, we just exit since Init handles the UI.
	if err := c.Init(
		WithExactArgs(1, args),
		WithFlags(c.Flags()),
		WithNoConfig(),
	); err != nil {
		c.ui.ErrorWithContext(err, ErrParsingArgsOrFlags)
		c.ui.Info(c.helpUsageMessage())
		return 1
	}

	packRepoName := c.args[0]

	// Generate our UI error context.
	errorContext := errors.NewUIErrorContext()

	c.registryName, c.packName, err = parseRegistryAndPackName(packRepoName)
	if err != nil {
		c.ui.ErrorWithContext(err, "failed to parse pack name", errorContext.GetAll()...)
		return 1
	}

	errorContext.Add(errors.UIContextPrefixPackName, c.packName)
	errorContext.Add(errors.UIContextPrefixRegistryName, c.registryName)

	registryPath, err := getRegistryPath(c.registryName, c.ui, errorContext)
	if err != nil {
		return 1
	}

	// Add the path to the pack on the error context.
	errorContext.Add(errors.UIContextPrefixPackPath, registryPath)

	// verify packs exist before running jobs
	if err = verifyPackExist(c.ui, c.packName, registryPath, errorContext); err != nil {
		return 1
	}

	// split pack name and version
	// TODO: Move this parsing function a shared package.
	c.packName, c.packVersion, err = registry.ParsePackNameAndVersion(c.packName)
	if err != nil {
		c.ui.ErrorWithContext(err, "failed to determine pack version", errorContext.GetAll()...)
	}

	// Add the path to the pack on the error context.
	errorContext.Add(errors.UIContextPrefixPackVersion, c.packVersion)

	// If no deploymentName set default to pack@version
	c.deploymentName = getDeploymentName(c.baseCommand, c.packName, c.packVersion)
	errorContext.Add(errors.UIContextPrefixDeploymentName, c.deploymentName)

	// create the http client
	client, err := v1.NewClient()
	if err != nil {
		c.ui.ErrorWithContext(err, "failed to initialize client", errorContext.GetAll()...)
		return 1
	}

	// @@@@ Temp fix to allow loading packs without a version until we talk about
	// adding version to the pack package.
	packTarget := c.packName
	if c.packVersion != "" {
		packTarget = fmt.Sprintf("%s@%s", packTarget, c.packVersion)
	}
	packManager := generatePackManager(c.baseCommand, client, registryPath, packTarget)

	// Render the pack now, before creating the deployer. If we get an error
	// we won't make it to the deployer.
	r, err := renderPack(packManager, c.baseCommand.ui, errorContext)
	if err != nil {
		return 255
	}

	renderedParents := r.ParentRenders()

	depConfig := runner.Config{
		PackName:       c.packName,
		PathPath:       path.Join(registryPath, c.packName),
		PackVersion:    c.packVersion,
		DeploymentName: c.deploymentName,
		RegistryName:   c.registryName,
	}

	// TODO(jrasell) come up with a better way to pass the appropriate config.
	runDeployer, err := generateRunner(client, "job", c.jobConfig, &depConfig)
	if err != nil {
		c.ui.ErrorWithContext(err, "failed to generate deployer", errorContext.GetAll()...)
		return 1
	}

	// Set the rendered templates on the job deployer.
	runDeployer.SetTemplates(renderedParents)

	// Parse the templates. If we have any error, output this and exit.
	if validateErrs := runDeployer.ParseTemplates(); validateErrs != nil {
		for _, validateErr := range validateErrs {
			validateErr.Context.Append(errorContext)
			c.ui.ErrorWithContext(validateErr.Err, validateErr.Subject, validateErr.Context.GetAll()...)
		}
		return 1
	}

	// Canonicalize the templates. If we have any error, output this and exit.
	if canonicalizeErrs := runDeployer.CanonicalizeTemplates(); canonicalizeErrs != nil {
		for _, canonicalizeErr := range canonicalizeErrs {
			canonicalizeErr.Context.Append(errorContext)
			c.ui.ErrorWithContext(canonicalizeErr.Err, canonicalizeErr.Subject, canonicalizeErr.Context.GetAll()...)
		}
		return 1
	}

	if conflictErrs := runDeployer.CheckForConflicts(errorContext); conflictErrs != nil {
		for _, conflictErr := range conflictErrs {
			c.ui.ErrorWithContext(conflictErr.Err, conflictErr.Subject, conflictErr.Context.GetAll()...)
		}
		return 1
	}

	// Deploy the rendered template. If we have any error, output this and
	// exit.
	if deployErr := runDeployer.Deploy(c.ui, errorContext); deployErr != nil {
		c.ui.ErrorWithContext(deployErr.Err, deployErr.Subject, deployErr.Context.GetAll()...)
		return 1
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

// Flags defines the flag.Sets for the operation.
func (c *RunCommand) Flags() *flag.Sets {
	return c.flagSet(flagSetOperation, func(set *flag.Sets) {
		f := set.NewSet("Run Options")

		c.jobConfig = &job.CLIConfig{
			RunConfig: &job.RunCLIConfig{},
		}

		f.Uint64Var(&flag.Uint64Var{
			Name:    "check-index",
			Target:  &c.jobConfig.RunConfig.CheckIndex,
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
			Target:  &c.jobConfig.RunConfig.ConsulToken,
			Default: "",
			Usage: `If set, the passed Consul token is stored in the job before
                      sending to the Nomad servers. This allows passing the Consul
                      token without storing it in the job file. This overrides the
                      token found in the $CONSUL_HTTP_TOKEN environment variable
                      and that found in the job.`,
		})

		f.StringVar(&flag.StringVar{
			Name:    "consul-namespace",
			Target:  &c.jobConfig.RunConfig.ConsulNamespace,
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
			Target:  &c.jobConfig.RunConfig.VaultToken,
			Default: "",
			Usage: `If set, the passed Vault token is stored in the job before
                      sending to the Nomad servers. This allows passing the Vault 
                      token without storing it in the job file. This overrides the 
                      token found in the $VAULT_TOKEN environment variable and 
                      that found in the job.`,
		})

		f.StringVar(&flag.StringVar{
			Name:    "vault-namespace",
			Target:  &c.jobConfig.RunConfig.VaultNamespace,
			Default: "",
			Usage: `If set, the passed Vault namespace is stored in the job before 
                    sending to the Nomad servers.`,
		})

		f.BoolVar(&flag.BoolVar{
			Name:    "policy-override",
			Target:  &c.jobConfig.RunConfig.PolicyOverride,
			Default: false,
			Usage: `Sets the flag to force override any soft mandatory Sentinel 
                      policies.`,
		})

		f.BoolVar(&flag.BoolVar{
			Name:    "preserve-counts",
			Target:  &c.jobConfig.RunConfig.PreserveCounts,
			Default: false,
			Usage: `If set, the existing task group counts will be preserved 
                      when updating a job.`,
		})

		f.BoolVar(&flag.BoolVar{
			Name:    "hcl1",
			Target:  &c.jobConfig.RunConfig.HCL1,
			Default: false,
			Usage:   `If set, the hcl V1 parser will be used to parse the job file.`,
		})

		f.BoolVar(&flag.BoolVar{
			Name:    "rollback",
			Hidden:  true,
			Target:  &c.jobConfig.RunConfig.EnableRollback,
			Default: false,
			Usage: `EXPERIMENTAL. If set, any pack failure will cause nomad pack
                       to attempt to rollback the entire deployment.`,
		})
	})
}

func (c *RunCommand) AutocompleteArgs() complete.Predictor {
	return complete.PredictNothing
}

func (c *RunCommand) AutocompleteFlags() complete.Flags {
	return c.Flags().Completions()
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
	packDeploymentNameKey = "pack_deployment_name"
)
