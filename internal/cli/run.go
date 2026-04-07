// Copyright IBM Corp. 2023, 2026
// SPDX-License-Identifier: MPL-2.0

package cli

import (
	"fmt"
	"os"

	"github.com/posener/complete"

	consulapi "github.com/hashicorp/consul/api"
	"github.com/hashicorp/nomad-pack/internal/pkg/caching"
	"github.com/hashicorp/nomad-pack/internal/pkg/errors"
	"github.com/hashicorp/nomad-pack/internal/pkg/flag"
	"github.com/hashicorp/nomad-pack/internal/runner"
	"github.com/hashicorp/nomad-pack/internal/runner/job"
)

type RunCommand struct {
	*baseCommand
	packConfig *caching.PackConfig
	jobConfig  *job.CLIConfig
	Validation ValidationFn

	// Consul KV configuration for template functions
	consulAddress   string
	consulToken     string
	consulNamespace string
	// TLS fields:
	consulCACert        string
	consulClientCert    string
	consulClientKey     string
	consulTLSSkipVerify bool
	consulTLSServerName string
}

func (c *RunCommand) Run(args []string) int {
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
	return c.run()
}

// run is the implementation of this command. It is used to ensure the args are
// pulled from the RunCommand as these are parsed with the Run.
func (c *RunCommand) run() int {

	c.packConfig.Name = c.args[0]

	// Set the packConfig defaults if necessary and generate our UI error context.
	errorContext := initPackCommand(c.packConfig)

	// verify packs exist before running jobs
	err := caching.VerifyPackExists(c.packConfig, errorContext, c.ui)
	if err != nil {
		return 1
	}

	// If no deploymentName set default to pack@ref
	c.deploymentName = getDeploymentName(c.baseCommand, c.packConfig)
	errorContext.Add(errors.UIContextPrefixDeploymentName, c.deploymentName)

	// create the http client
	client, err := c.getAPIClient()
	if err != nil {
		c.ui.ErrorWithContext(err, "failed to initialize client", errorContext.GetAll()...)
		return 1
	}

	//initialize Consul client for KV template functions if address is provided
	var consulClient *consulapi.Client
	if c.consulAddress != "" {
		consulConfig := consulapi.DefaultConfig()
		consulConfig.Address = c.consulAddress

		// support environment variables with CLI flag priority
		if c.consulToken == "" {
			c.consulToken = os.Getenv("CONSUL_HTTP_TOKEN")
		}
		if c.consulToken != "" {
			consulConfig.Token = c.consulToken
		}

		if c.consulNamespace == "" {
			c.consulNamespace = os.Getenv("CONSUL_NAMESPACE")
		}
		if c.consulNamespace != "" {
			consulConfig.Namespace = c.consulNamespace
		}

		// TLS Configuration with environment variable fallback
		if c.consulCACert == "" {
			c.consulCACert = os.Getenv("CONSUL_CACERT")
		}
		if c.consulClientCert == "" {
			c.consulClientCert = os.Getenv("CONSUL_CLIENT_CERT")
		}
		if c.consulClientKey == "" {
			c.consulClientKey = os.Getenv("CONSUL_CLIENT_KEY")
		}
		if !c.consulTLSSkipVerify {
			// check env var for skip verify (inverted logic)
			if os.Getenv("CONSUL_HTTP_SSL_VERIFY") == "false" {
				c.consulTLSSkipVerify = true
			}
		}
		if c.consulTLSServerName == "" {
			c.consulTLSServerName = os.Getenv("CONSUL_TLS_SERVER_NAME")
		}

		// Apply TLS configuration
		if c.consulCACert != "" || c.consulClientCert != "" || c.consulClientKey != "" {
			consulConfig.TLSConfig.CAFile = c.consulCACert
			consulConfig.TLSConfig.CertFile = c.consulClientCert
			consulConfig.TLSConfig.KeyFile = c.consulClientKey
			consulConfig.TLSConfig.InsecureSkipVerify = c.consulTLSSkipVerify

			if c.consulTLSServerName != "" {
				consulConfig.TLSConfig.Address = c.consulTLSServerName
			}
		}

		consulClient, err = consulapi.NewClient(consulConfig)
		if err != nil {
			c.ui.ErrorWithContext(err, "failed to create Consul client for KV operations", errorContext.GetAll()...)
			return 1
		}
	}

	packManager := generatePackManager(c.baseCommand, client, c.packConfig, consulClient)

	// Render the pack now, before creating the deployer. If we get an error
	// we won't make it to the deployer.
	r, err := renderPack(
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

	renderedParents := r.ParentRenders()
	renderedDeps := r.DependentRenders()

	// TODO: Refactor to use PackConfig. Maybe PackConfig should be in a more common
	// pkg than cache, or maybe it's ok for runner to depend on the cache.
	// Need to discuss with jrasell.
	depConfig := runner.Config{
		PackName:       c.packConfig.Name,
		PathPath:       c.packConfig.Path,
		PackRef:        c.packConfig.Ref,
		DeploymentName: c.deploymentName,
		RegistryName:   c.packConfig.Registry,
	}

	// TODO(jrasell) come up with a better way to pass the appropriate config.
	runDeployer, err := generateRunner(client, "job", c.jobConfig, &depConfig)
	if err != nil {
		c.ui.ErrorWithContext(err, "failed to generate deployer", errorContext.GetAll()...)
		return 1
	}

	// Set the rendered templates on the job deployer.
	templates := make(map[string]string, r.LenDependentRenders()+r.LenParentRenders())
	for dn, ds := range renderedDeps {
		templates[dn] = ds
	}
	for pn, ps := range renderedParents {
		templates[pn] = ps
	}
	runDeployer.SetTemplates(templates)

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

	// Monitor deployments unless detach flag is set
	if !c.jobConfig.RunConfig.Detach {
		evalIDs := runDeployer.EvalIDs()
		length := shortId
		if c.jobConfig.RunConfig.Verbose {
			length = fullId
		}
		mon := newMonitor(c.Ctx, c.ui, client, length)
		if exitCode := mon.monitor(evalIDs); exitCode != 0 {
			return exitCode
		}
	}

	if c.packConfig.Registry == caching.DevRegistryName {
		c.ui.Success(fmt.Sprintf("Pack successfully deployed. Use %s to manage this deployed instance with plan, stop, destroy, or info", c.packConfig.SourcePath))
	} else {
		c.ui.Success(fmt.Sprintf("Pack successfully deployed. Use %s with --ref=%s to manage this deployed instance with plan, stop, destroy, or info", c.packConfig.Name, c.packConfig.Ref))
	}

	output, err := packManager.ProcessOutputTemplate()
	if err != nil {
		c.ui.ErrorWithContext(err, "failed to render output template", "Pack Name: "+c.packConfig.Name)
		return 1
	}

	if output != "" {
		c.ui.Output(fmt.Sprintf("\n%s", output))
	}
	return 0
}

// Flags defines the flag.Sets for the operation.
func (c *RunCommand) Flags() *flag.Sets {
	return c.flagSet(flagSetOperation|flagSetNomadClient, func(set *flag.Sets) {
		f := set.NewSet("Run Options")

		c.packConfig = &caching.PackConfig{}

		c.jobConfig = &job.CLIConfig{
			RunConfig: &job.RunCLIConfig{},
		}

		f.StringVar(&flag.StringVar{
			Name:    "registry",
			Target:  &c.packConfig.Registry,
			Default: "",
			Usage:   `Specific registry name containing the pack to be run.`,
		})

		f.StringVar(&flag.StringVar{
			Name:    "ref",
			Target:  &c.packConfig.Ref,
			Default: "",
			Usage: `Specific git ref of the pack to be run.
					Supports tags, SHA, and latest. If no ref is specified,
					defaults to latest.

					Using ref with a file path is not supported.`,
		})

		f.Uint64Var(&flag.Uint64Var{
			Name:    "check-index",
			Target:  &c.jobConfig.RunConfig.CheckIndex,
			Default: 0,
			Usage: `If set, the job is only registered or updated if the passed
					job modify index matches the server side version. If a
					check-index value of zero is passed, the job is only
					registered if it does not yet exist. If a non-zero value is
					passed, it ensures that the job is being updated from a
					known state. The use of this flag is most common in
					conjunction with job plan command.`,
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
					specified Consul namespace. Any template block reading from
					Consul KV will be scoped to the the specified Consul
					namespace. If Consul ACLs are enabled and the
					allow_unauthenticated Nomad server Consul configuration is
					not enabled, then a Consul token must be supplied with
					appropriate service and KV Consul ACL policy permissions.`,
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
			Usage: `If set, the passed Vault namespace is stored in the job
					before sending to the Nomad servers.`,
		})

		f.StringVar(&flag.StringVar{
			Name:    "consul-kv-address",
			Target:  &c.consulAddress,
			Default: "",
			Usage: `Address of the Consul agent for KV template operations. If not set, 
			        Consul KV template functions (consulKey, consulKeys) will not be 
					available. Example: http://127.0.0.1:8500`,
		})
		f.StringVar(&flag.StringVar{
			Name:    "consul-kv-token",
			Target:  &c.consulToken,
			Default: "",
			Usage: `Consul ACL token for KV template operations. If not provided, 
			        uses the token from CONSUL_HTTP_TOKEN environment variable or 
			        anonymous access if ACLs are not enabled.`,
		})

		f.StringVar(&flag.StringVar{
			Name:    "consul-kv-namespace",
			Target:  &c.consulNamespace,
			Default: "",
			Usage: `Consul namespace for KV template operations (Consul Enterprise only). 
			        If not provided, uses the default namespace.`,
		})

		f.StringVar(&flag.StringVar{
			Name:    "consul-kv-ca-cert",
			Target:  &c.consulCACert,
			Default: "",
			Usage: `Path to a PEM encoded CA cert file to verify the Consul server 
                    SSL certificate. Overrides CONSUL_CACERT environment variable.`,
		})

		f.StringVar(&flag.StringVar{
			Name:    "consul-kv-client-cert",
			Target:  &c.consulClientCert,
			Default: "",
			Usage: `Path to a PEM encoded client certificate for TLS authentication 
                    to Consul. Must also specify --consul-kv-client-key. Overrides 
                    CONSUL_CLIENT_CERT environment variable.`,
		})

		f.StringVar(&flag.StringVar{
			Name:    "consul-kv-client-key",
			Target:  &c.consulClientKey,
			Default: "",
			Usage: `Path to an unencrypted PEM encoded private key matching the 
                    client certificate. Overrides CONSUL_CLIENT_KEY environment variable.`,
		})

		f.BoolVar(&flag.BoolVar{
			Name:    "consul-kv-tls-skip-verify",
			Target:  &c.consulTLSSkipVerify,
			Default: false,
			Usage: `Do not verify TLS certificate. Not recommended for production. 
                    Overrides CONSUL_HTTP_SSL_VERIFY environment variable.`,
		})

		f.StringVar(&flag.StringVar{
			Name:    "consul-kv-tls-server-name",
			Target:  &c.consulTLSServerName,
			Default: "",
			Usage: `Server name to use as SNI host when connecting via TLS. 
                    Overrides CONSUL_TLS_SERVER_NAME environment variable.`,
		})

		f.BoolVar(&flag.BoolVar{
			Name:    "deploy-override",
			Target:  &c.jobConfig.RunConfig.DeployOverride,
			Default: false,
			Usage: `Sets the flag to force deploy over currently deployed job (even 
					externally deployed jobs).`,
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
			Name:    "preserve-resources",
			Target:  &c.jobConfig.RunConfig.PreserveResources,
			Default: false,
			Usage: `If set, the existing task group resource definitions will be preserved
					when updating a job.`,
		})

		f.BoolVar(&flag.BoolVar{
			Name:    "detach",
			Target:  &c.jobConfig.RunConfig.Detach,
			Default: false,
			Usage: `If set, deployment monitoring will be skipped and the command
					will return immediately after registration.`,
		})

		f.BoolVar(&flag.BoolVar{
			Name:    "verbose",
			Target:  &c.jobConfig.RunConfig.Verbose,
			Default: false,
			Usage: `If set, deployment monitoring will show verbose output
					including allocation details.`,
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
	return predictPackName
}

func (c *RunCommand) AutocompleteFlags() complete.Flags {
	return c.Flags().Completions()
}

func (c *RunCommand) Help() string {
	// TODO: do we want to ref example?
	c.Example = `
	# Run an example pack with the default deployment name "example".
	nomad-pack run example

	# Run an example pack with the specified deployment name "dev"
	nomad-pack run example --name=dev

	# Run an example pack with override variables in a variable file
	nomad-pack run example --var-file="./overrides.hcl"

	# Run an example pack with cli variable overrides
	nomad-pack run example --var="redis_image_version=latest" --var="redis_resources={"cpu": "1000", "memory": "512"}"

	# Run a pack under development from the filesystem - supports current
	# working directory or relative path
	nomad-pack run .
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
