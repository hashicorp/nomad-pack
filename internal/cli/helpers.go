// Copyright IBM Corp. 2023, 2026
// SPDX-License-Identifier: MPL-2.0

package cli

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	consulapi "github.com/hashicorp/consul/api"
	pkgflag "github.com/hashicorp/nomad-pack/internal/pkg/flag"
	"github.com/hashicorp/nomad/api"
	"github.com/posener/complete"

	"github.com/hashicorp/nomad-pack/internal/pkg/caching"
	"github.com/hashicorp/nomad-pack/internal/pkg/errors"
	"github.com/hashicorp/nomad-pack/internal/pkg/manager"
	"github.com/hashicorp/nomad-pack/internal/pkg/renderer"
	"github.com/hashicorp/nomad-pack/internal/pkg/variable/parser"
	"github.com/hashicorp/nomad-pack/internal/runner"
	"github.com/hashicorp/nomad-pack/internal/runner/job"
	"github.com/hashicorp/nomad-pack/terminal"
)

// get an initialized error context for a command that accepts pack args.
func initPackCommand(cfg *caching.PackConfig) (errorContext *errors.UIErrorContext) {
	cfg.Init()

	// Generate our UI error context.
	errorContext = errors.NewUIErrorContext()
	errorContext.Add(errors.UIContextPrefixRegistryName, cfg.Registry)
	errorContext.Add(errors.UIContextPrefixPackName, cfg.Name)
	errorContext.Add(errors.UIContextPrefixPackRef, cfg.Ref)
	if cfg.Registry == caching.DevRegistryName && cfg.Ref == caching.DevRef {
		errorContext.Add(errors.UIContextPrefixPackPath, cfg.Path)
	}
	return
}

// generatePackManager is used to generate the pack manager for this Nomad Pack run.
func generatePackManager(c *baseCommand, client *api.Client, packCfg *caching.PackConfig, consulClient *consulapi.Client) *manager.PackManager {
	// TODO: Refactor to have manager use cache.
	cfg := manager.Config{
		Path:            packCfg.Path,
		VariableFiles:   c.varFiles,
		VariableCLIArgs: c.vars,
		VariableEnvVars: c.envVars,
		AllowUnsetVars:  c.allowUnsetVars,
		UseParserV1:     c.useParserV1,
	}
	return manager.NewPackManager(&cfg, client, consulClient)
}

// predictPackName is a complete.Predictor that suggests cached pack names.
// When --registry is specified on the command line, suggestions are filtered
// to only that registry. Duplicate pack names across registries are removed.
var predictPackName = complete.PredictFunc(func(args complete.Args) []string {
	registryFilter := extractFlagValue(args.All, "registry")

	globalCache, err := caching.NewCache(&caching.CacheConfig{
		Path:   caching.DefaultCachePath(),
		Logger: nil,
	})
	if err != nil {
		return nil
	}

	if err = globalCache.Load(); err != nil {
		return nil
	}

	seen := map[string]struct{}{}
	var packNames []string

	for _, cachedRegistry := range globalCache.Registries() {
		if registryFilter != "" && cachedRegistry.Name != registryFilter {
			continue
		}
		for _, registryPack := range cachedRegistry.Packs {
			name := registryPack.Name()
			if _, ok := seen[name]; !ok {
				seen[name] = struct{}{}
				packNames = append(packNames, name)
			}
		}
	}

	return packNames
})

// extractFlagValue looks for --flag value or --flag=value in the given args
// and returns the value. Returns an empty string if not found.
func extractFlagValue(args []string, flag string) string {
	dashed := "--" + flag
	for i, arg := range args {
		if arg == dashed && i+1 < len(args) {
			return args[i+1]
		}
		if strings.HasPrefix(arg, dashed+"=") {
			return strings.TrimPrefix(arg, dashed+"=")
		}
	}
	return ""
}

func registryTable() *terminal.Table {
	return terminal.NewTable("REGISTRY NAME", "REF", "LOCAL_REF", "REGISTRY_URL")
}

func registryPackTable() *terminal.Table {
	return terminal.NewTable("PACK NAME", "REF", "LOCAL_REF", "METADATA VERSION", "REGISTRY NAME", "REGISTRY_URL")
}

func packTable() *terminal.Table {
	return terminal.NewTable("PACK NAME", "METADATA VERSION", "REGISTRY NAME")
}

func registryTableRow(cachedRegistry *caching.Registry) []string {
	return []string{
		cachedRegistry.Name,
		formatSHA1Reference(cachedRegistry.Ref),
		formatSHA1Reference(cachedRegistry.LocalRef),
		cachedRegistry.Source,
	}
}

func registryPackRow(cachedRegistry *caching.Registry, cachedPack *caching.Pack) []string {
	return []string{
		// The Name of the registryPack
		cachedPack.Name(),

		// The revision from where the registryPack was cloned
		formatSHA1Reference(cachedPack.Ref),

		// The canonical revision from where the registryPack was cloned
		formatSHA1Reference(cachedRegistry.LocalRef),

		// The metadata version
		cachedPack.Metadata.Pack.Version,

		// CachedRegistry name  user defined alias or registry URL slug
		cachedRegistry.Name,

		// The cachedRegistry URL from where the registryPack was cloned
		cachedRegistry.Source,

		// TODO: The app version
	}
}
func registryName(cr *caching.Registry) string {
	if cr.Ref == cr.LocalRef {
		return fmt.Sprintf("%s@%s", cr.Name, formatSHA1Reference(cr.LocalRef))
	}
	// While it is completely unexpected to encounter a case where the Ref is a
	// SHA1 hash and the LocalRef doesn't match, we can safely run the Ref through
	// the formatSHA1Reference func since it will return non-SHA values unmodified
	// and if somehow the mismatched SHA case does happen, the output will not
	// become ridiculous.
	return fmt.Sprintf("%s@%s (%s)", cr.Name, formatSHA1Reference(cr.Ref), formatSHA1Reference(cr.LocalRef))
}

func packRow(cachedRegistry *caching.Registry, cachedPack *caching.Pack) []string {
	return []string{
		// The Name of the registryPack
		cachedPack.Name(),

		// The metadata version
		cachedPack.Metadata.Pack.Version,

		// CachedRegistry name  user defined alias or registry URL slug
		registryName(cachedRegistry),

		// TODO: The app version
	}
}

// TODO: This needs to be on a domain specific pkg rather than a UI helpers file.
// This will be possible once we create a logger interface that can be passed
// between layers.
// Uses the pack manager to parse the templates, override template variables with var files
// and cli vars as applicable
func renderPack(
	manager *manager.PackManager,
	ui terminal.UI,
	renderAux bool,
	format bool,
	ignoreMissingVars bool,
	errCtx *errors.UIErrorContext,
) (*renderer.Rendered, error) {
	r, err := manager.ProcessTemplates(renderAux, format, ignoreMissingVars)
	if err != nil {
		packName := manager.PackName()
		errCtx.Add(errors.UIContextPrefixPackName, packName)
		for i := range err {
			err[i].Context.Append(errCtx)
			ui.ErrorWithContext(err[i].Err, "failed to process pack", err[i].Context.GetAll()...)
		}
		return nil, errors.New("failed to render")
	}
	return r, nil
}

// TODO: This needs to be on a domain specific pkg rather than a UI helpers file.
// This will be possible once we create a logger interface that can be passed
// between layers.
// Uses the pack manager to parse the templates, override template variables with var files
// and cli vars as applicable
func renderVariableOverrideFile(
	manager *manager.PackManager,
	ui terminal.UI,
	errCtx *errors.UIErrorContext,
) (*parser.ParsedVariables, error) {

	r, err := manager.ProcessVariableFiles()
	if err != nil {
		packName := manager.PackName()
		errCtx.Add(errors.UIContextPrefixPackName, packName)
		for i := range err {
			err[i].Context.Append(errCtx)
			ui.ErrorWithContext(err[i].Err, "failed to process pack", err[i].Context.GetAll()...)
		}
		return nil, errors.New("failed to render")
	}
	r.Metadata = manager.Metadata()
	return r, nil
}

// TODO: This needs to be on a domain specific pkg rather than a UI helpers file.
// This will be possible once we create a logger interface that can be passed
// between layers.
// Uses open api client to parse rendered hcl templates to
// open api jobs to send to nomad
func parseJob(cmd *baseCommand, hcl string, errCtx *errors.UIErrorContext) (*api.Job, error) {
	// instantiate client to parse hcl
	c, err := cmd.getAPIClient()
	if err != nil {
		cmd.ui.ErrorWithContext(err, "failed to initialize client", errCtx.GetAll()...)
		return nil, err
	}

	parsedJob, err := c.Jobs().ParseHCLOpts(&api.JobsParseRequest{
		JobHCL:       hcl,
		Canonicalize: true,
	})
	if err != nil {
		cmd.ui.ErrorWithContext(err, "failed to parse job specification", errCtx.GetAll()...)
		return nil, err
	}
	return parsedJob, nil
}

// Generates a deployment name if not specified. Default is pack@version.
func getDeploymentName(c *baseCommand, cfg *caching.PackConfig) string {
	if c.deploymentName == "" {
		return caching.AppendRef(cfg.Name, cfg.Ref)
	}
	return c.deploymentName
}

// TODO: Move to a domain specific package.

// getPackJobsByDeploy filters the provided jobs to only those matching the deployment name.
// If no jobs are found matching the deployment name but there are jobs matching the pack
// name with different deployment names, an error is returned to avoid accidentally operating
// on the wrong jobs.
func getPackJobsByDeploy(jobs []*api.Job, c *api.Client, cfg *caching.PackConfig, deploymentName string) ([]*api.Job, error) {
	packJobs := []*api.Job{}
	queryOpts := &api.QueryOptions{}
	jobsApi := c.Jobs()
	hasOtherDeploys := false
	for _, packJob := range jobs {
		if packJob.Namespace != nil && *packJob.Namespace != "" && *packJob.Namespace != api.DefaultNamespace {
			queryOpts.Namespace = *packJob.Namespace
		}
		nomadJob, _, err := jobsApi.Info(*packJob.ID, queryOpts)
		if err != nil {
			return nil, fmt.Errorf("error retrieving job %s for pack %s: %s", *packJob.ID, cfg.Name, err)
		}

		if nomadJob.Meta != nil {
			jobMeta := nomadJob.Meta
			jobDeploymentName, ok := jobMeta[job.PackDeploymentNameKey]

			if ok {
				if jobDeploymentName == deploymentName {
					packJobs = append(packJobs, nomadJob)
				} else {
					// Check if there are jobs that match the pack name but with different
					// deployment names in case packJobs is empty.
					// Since different registries can share pack names, we need to check
					// registry and pack names both match
					jobRegistry, registryOk := jobMeta[job.PackRegistryKey]
					jobPack, packOk := jobMeta[job.PackNameKey]
					if registryOk && packOk && jobRegistry == cfg.Registry && jobPack == cfg.Name {
						hasOtherDeploys = true
					}
				}
			}
		}

		if len(packJobs) == 0 && hasOtherDeploys {
			// TODO: the aesthetics here could be better. This error line is very long.
			return nil, fmt.Errorf(
				"pack %q running but not in deployment %q. Run \"nomad-pack status %s\" for more information",
				cfg.Name, deploymentName, cfg.Name)
		}
	}
	return packJobs, nil
}

// TODO: Needs code review. Will likely move if we decide to move client management
// out of CLI commands.
func generateRunner(client *api.Client, packType, cliCfg any, runnerCfg *runner.Config) (runner.Runner, error) {

	var (
		err          error
		deployerImpl runner.Runner
	)

	// Depending on the type of pack we are dealing with, generate the correct
	// implementation.
	switch packType {
	case "job":
		jobConfig, ok := cliCfg.(*job.CLIConfig)
		if !ok {
			return nil, fmt.Errorf("failed to assert correct config, unsuitable type %T", cliCfg)
		}
		deployerImpl = job.NewDeployer(client, jobConfig)
	default:
		err = fmt.Errorf("unsupported pack type %q", packType)
	}

	// Return the error if you got one.
	if err != nil {
		return nil, err
	}

	// Set the config; this means commands do not have to do this, and it's
	// done in a single place.
	deployerImpl.SetRunnerConfig(runnerCfg)
	return deployerImpl, nil
}

// TODO: Not all commands use vars or varFiles. These fields should be abstracted
// away from the baseCommand and then this function can get moved where appropriate.
func hasVarOverrides(c *baseCommand) bool {
	return len(c.varFiles) > 0 || len(c.vars) > 0
}

// TODO: Move to a domain specific package.
func getDeployedPacks(c *api.Client) (map[string]map[string]struct{}, error) {
	jobsApi := c.Jobs()
	jobs, _, err := jobsApi.List(&api.QueryOptions{})
	if err != nil {
		return nil, fmt.Errorf("error finding jobs: %s", err)
	}

	packRegistryMap := map[string]map[string]struct{}{}
	for _, jobStub := range jobs {
		nomadJob, _, err := jobsApi.Info(jobStub.ID, &api.QueryOptions{
			Namespace: jobStub.Namespace,
		})
		if err != nil {
			return nil, fmt.Errorf("error retrieving job %s: %w", jobStub.ID, err)
		}

		if nomadJob.Meta != nil {
			jobMeta := nomadJob.Meta
			// Check metadata for pack info
			packName, packNameOk := jobMeta[job.PackNameKey]
			packRegistry, registryNameOk := jobMeta[job.PackRegistryKey]
			if packNameOk && registryNameOk {
				// Build a map of packs and their registries
				registryMap, deployedPackOk := packRegistryMap[packName]

				if deployedPackOk {
					_, registryOk := registryMap[packRegistry]
					if !registryOk {
						registryMap[packRegistry] = struct{}{}
					}
				} else {
					packRegistryMap[packName] = map[string]struct{}{}
					packRegistryMap[packName][packRegistry] = struct{}{}
				}
			}
		}
	}
	return packRegistryMap, nil
}

// TODO: Move to a domain specific package.

// JobStatusInfo encapsulates status information about a running job.
type JobStatusInfo struct {
	packName       string
	registryName   string
	deploymentName string
	jobID          string
	status         string
}

// TODO: Move to a domain specific package.

// JobStatusError encapsulates error information related to trying to retrieve
// status information about a running job.
type JobStatusError struct {
	jobID    string
	jobError error
}

// TODO: Move to a domain specific package.
func getDeployedPackJobs(c *api.Client, cfg *caching.PackConfig, deploymentName string) ([]JobStatusInfo, []JobStatusError, error) {
	jobsApi := c.Jobs()
	jobs, _, err := jobsApi.List(&api.QueryOptions{})
	if err != nil {
		return nil, nil, fmt.Errorf("error finding jobs for pack %s: %s", cfg.Name, err)
	}

	var packJobs []JobStatusInfo
	var jobErrs []JobStatusError
	for _, jobStub := range jobs {
		nomadJob, _, err := jobsApi.Info(jobStub.ID, &api.QueryOptions{})
		if err != nil {
			jobErrs = append(jobErrs, JobStatusError{
				jobID:    jobStub.ID,
				jobError: err,
			})
			continue
		}

		if nomadJob.Meta != nil {
			jobMeta := nomadJob.Meta
			jobPackName, ok := jobMeta[job.PackNameKey]
			if ok && jobPackName == cfg.Name {
				// Filter by deployment name if specified
				if deploymentName != "" {
					jobDeployName, deployOk := jobMeta[job.PackDeploymentNameKey]
					if deployOk && jobDeployName != deploymentName {
						continue
					}
				}
				packJobs = append(packJobs, JobStatusInfo{
					packName:       cfg.Name,
					registryName:   jobMeta[job.PackRegistryKey],
					deploymentName: jobMeta[job.PackDeploymentNameKey],
					jobID:          *nomadJob.ID,
					status:         *nomadJob.Status,
				})
			}
		}
	}
	return packJobs, jobErrs, nil
}

// clientOptsFromCLI emits a slice of v1.ClientOptions based on the environment
// and flag set passed to the command.
func clientOptsFromCLI(c *baseCommand) *api.Config {
	// This implementation leverages the fact that flags always take precedence
	// over environment variables to naively append the flags to the env
	// settings.
	conf := api.DefaultConfig()

	clientOptsFromEnvironment(conf)
	clientOptsFromFlags(c, conf)
	return conf
}

// handlBasicAuth checks whether the NOMAD_ADDR string is in the user:pass@addr
// format and if it is, it returns user, password and address. It returns "", "",
// address otherwise.
func handleBasicAuth(s string) (string, string, string) {
	before, after, found := strings.Cut(s, "@")
	if found {
		user, pass, found := strings.Cut(before, ":")
		if found {
			return user, pass, after
		}
	}
	return "", "", before
}

// clientOptsFromEnvironment populates api client conf with environment
// variables present at the CLI's runtime.
func clientOptsFromEnvironment(conf *api.Config) {
	if v := os.Getenv("NOMAD_ADDR"); v != "" {
		// we support user:pass@addr here
		user, pass, addr := handleBasicAuth(v)
		conf.Address = addr
		if user != "" && pass != "" {
			conf.HttpAuth.Username = user
			conf.HttpAuth.Password = pass
		}
	}
	if v := os.Getenv("NOMAD_NAMESPACE"); v != "" {
		conf.Namespace = v
	}
	if v := os.Getenv("NOMAD_REGION"); v != "" {
		conf.Region = v
	}
	if v := os.Getenv("NOMAD_TOKEN"); v != "" {
		conf.SecretID = v
	}
	if cc, ck := os.Getenv("NOMAD_CLIENT_CERT"), os.Getenv("NOMAD_CLIENT_KEY"); cc != "" && ck != "" {
		conf.TLSConfig.ClientCert = cc
		conf.TLSConfig.ClientKey = ck
	}
	if v := os.Getenv("NOMAD_CACERT"); v != "" {
		conf.TLSConfig.CACert = v
	}
	if v := os.Getenv("NOMAD_TLS_SERVER_NAME"); v != "" {
		conf.TLSConfig.TLSServerName = v
	}
	if _, found := os.LookupEnv("NOMAD_SKIP_VERIFY"); found {
		conf.TLSConfig.Insecure = true
	}
}

// clientOptsFromFlags populates api client conf based on the flags passed to
// the CLI at runtime.
func clientOptsFromFlags(c *baseCommand, conf *api.Config) {
	cfg := c.nomadConfig
	if cfg.address != "" {
		// we support user:pass@addr here
		user, pass, addr := handleBasicAuth(cfg.address)
		conf.Address = addr
		if user != "" && pass != "" {
			conf.HttpAuth.Username = user
			conf.HttpAuth.Password = pass
		}
	}
	if cfg.namespace != "" {
		conf.Namespace = cfg.namespace
	}
	if cfg.region != "" {
		conf.Region = cfg.region
	}
	if cfg.token != "" {
		conf.SecretID = cfg.token
	}
	if cfg.clientCert != "" && cfg.clientKey != "" {
		conf.TLSConfig.ClientCert = cfg.clientCert
		conf.TLSConfig.ClientKey = cfg.clientKey
	}
	if cfg.caCert != "" {
		conf.TLSConfig.CACert = cfg.caCert
	}
	if cfg.tlsServerName != "" {
		conf.TLSConfig.TLSServerName = cfg.tlsServerName
	}
	if cfg.tlsSkipVerify {
		conf.TLSConfig.Insecure = true
	}
}

// Limits the length of the string.
func limit(s string, length int) string {
	if len(s) < length {
		return s
	}

	return s[:length]
}

// ConsulKVConfig holds configuration for connecting to Consul KV with TLS support.
// This struct is shared across run, plan, and render commands to avoid code duplication.
type ConsulKVConfig struct {
	Address       string // Consul server address (e.g., "https://consul.example.com:8501")
	Token         string // ACL token for authentication
	Namespace     string // Consul namespace
	CACert        string // Path to CA certificate file
	ClientCert    string // Path to client certificate file (for mTLS)
	ClientKey     string // Path to client key file (for mTLS)
	TLSSkipVerify bool   // Skip TLS certificate verification (for testing)
	TLSServerName string // Override TLS server name
}

// AddFlags adds all Consul KV configuration flags to the provided flag set.
// This method is called by run, plan, and render commands to register flags.
func (c *ConsulKVConfig) AddFlags(flags *flag.FlagSet) {
	flags.StringVar(&c.Address, "consul-address", "",
		"Address of the Consul instance to use for template variable lookups. "+
			"Can also be specified via the CONSUL_HTTP_ADDR environment variable.")

	flags.StringVar(&c.Token, "consul-token", "",
		"ACL token to use when connecting to Consul. "+
			"Can also be specified via the CONSUL_HTTP_TOKEN environment variable.")

	flags.StringVar(&c.Namespace, "consul-namespace", "",
		"Consul namespace to use for KV lookups. "+
			"Can also be specified via the CONSUL_NAMESPACE environment variable.")

	flags.StringVar(&c.CACert, "consul-ca-cert", "",
		"Path to a CA certificate file to use for TLS when communicating with Consul. "+
			"Can also be specified via the CONSUL_CACERT environment variable.")

	flags.StringVar(&c.ClientCert, "consul-client-cert", "",
		"Path to a client certificate file to use for TLS when communicating with Consul. "+
			"Can also be specified via the CONSUL_CLIENT_CERT environment variable.")

	flags.StringVar(&c.ClientKey, "consul-client-key", "",
		"Path to a client key file to use for TLS when communicating with Consul. "+
			"Can also be specified via the CONSUL_CLIENT_KEY environment variable.")

	flags.BoolVar(&c.TLSSkipVerify, "consul-tls-skip-verify", false,
		"Skip TLS certificate verification when communicating with Consul. "+
			"Can also be specified via the CONSUL_TLS_SKIP_VERIFY environment variable.")

	flags.StringVar(&c.TLSServerName, "consul-tls-server-name", "",
		"Server name to use for TLS verification when communicating with Consul. "+
			"Can also be specified via the CONSUL_TLS_SERVER_NAME environment variable.")
}

// Package-level variable to track if Consul flags have been registered
var consulFlagsRegistered = false

// AddFlagsToSet adds Consul KV configuration flags to a flag.Set (used by run, plan, render commands).
func (c *ConsulKVConfig) AddFlagsToSet(f *pkgflag.Set) {
	// Check if flags are already registered to avoid panic on redefinition
	if consulFlagsRegistered {
		return
	}
	consulFlagsRegistered = true
	f.StringVar(&pkgflag.StringVar{
		Name:    "consul-address",
		Target:  &c.Address,
		Default: "",
		Usage: `Consul server address (e.g., https://consul.example.com:8501). 
		        Can also be specified via the CONSUL_HTTP_ADDR environment variable.`,
	})

	f.StringVar(&pkgflag.StringVar{
		Name:    "consul-token",
		Target:  &c.Token,
		Default: "",
		Usage: `Consul ACL token for authentication. 
		        Can also be specified via the CONSUL_HTTP_TOKEN environment variable.`,
	})

	f.StringVar(&pkgflag.StringVar{
		Name:    "consul-namespace",
		Target:  &c.Namespace,
		Default: "",
		Usage: `Consul namespace (Consul Enterprise only). 
		        Can also be specified via the CONSUL_NAMESPACE environment variable.`,
	})

	f.StringVar(&pkgflag.StringVar{
		Name:    "consul-ca-cert",
		Target:  &c.CACert,
		Default: "",
		Usage: `Path to a CA certificate file to use for TLS when communicating with Consul. 
		        Can also be specified via the CONSUL_CACERT environment variable.`,
	})

	f.StringVar(&pkgflag.StringVar{
		Name:    "consul-client-cert",
		Target:  &c.ClientCert,
		Default: "",
		Usage: `Path to a client certificate file to use for TLS when communicating with Consul. 
		        Can also be specified via the CONSUL_CLIENT_CERT environment variable.`,
	})

	f.StringVar(&pkgflag.StringVar{
		Name:    "consul-client-key",
		Target:  &c.ClientKey,
		Default: "",
		Usage: `Path to a client key file to use for TLS when communicating with Consul. 
		        Can also be specified via the CONSUL_CLIENT_KEY environment variable.`,
	})

	f.BoolVar(&pkgflag.BoolVar{
		Name:    "consul-tls-skip-verify",
		Target:  &c.TLSSkipVerify,
		Default: false,
		Usage: `Skip TLS certificate verification when communicating with Consul. 
		        Can also be specified via the CONSUL_TLS_SKIP_VERIFY environment variable.`,
	})

	f.StringVar(&pkgflag.StringVar{
		Name:    "consul-tls-server-name",
		Target:  &c.TLSServerName,
		Default: "",
		Usage: `Server name to use for TLS verification when communicating with Consul. 
		        Can also be specified via the CONSUL_TLS_SERVER_NAME environment variable.`,
	})
}

// NewConsulClient creates a new Consul API client with the configured TLS settings.
// Returns an error if the client cannot be created.
func (c *ConsulKVConfig) NewConsulClient() (*consulapi.Client, error) {
	cfg := consulapi.DefaultConfig()

	// Set basic configuration
	if c.Address != "" {
		cfg.Address = c.Address
	}
	if c.Token != "" {
		cfg.Token = c.Token
	}
	if c.Namespace != "" {
		cfg.Namespace = c.Namespace
	}

	// Configure TLS if any TLS options are set
	if c.CACert != "" || c.ClientCert != "" || c.ClientKey != "" || c.TLSSkipVerify || c.TLSServerName != "" {
		tlsConfig := &consulapi.TLSConfig{
			CAFile:             c.CACert,
			CertFile:           c.ClientCert,
			KeyFile:            c.ClientKey,
			InsecureSkipVerify: c.TLSSkipVerify,
		}

		if c.TLSServerName != "" {
			tlsConfig.Address = c.TLSServerName
		}

		cfg.TLSConfig = *tlsConfig
	}

	return consulapi.NewClient(cfg)
}

// getConsulClient creates a Consul API client if Consul is configured.
// Returns nil client if no Consul address is configured (not an error).
// The presence of a Consul address (via CLI flag or CONSUL_HTTP_ADDR env var)
// indicates that Nomad Pack should attempt to create a Consul API client.
func getConsulClient(consulKV *ConsulKVConfig, errorContext *errors.UIErrorContext, ui terminal.UI) (*consulapi.Client, error) {
	// Check if Consul is configured via CLI flag
	if consulKV.Address != "" {
		return consulKV.NewConsulClient()
	}

	// Check if Consul is configured via environment variable
	consulAddr := os.Getenv("CONSUL_HTTP_ADDR")
	if consulAddr == "" {
		// No Consul configured - this is not an error, just means Consul integration is disabled
		return nil, nil
	}

	// Consul is configured via environment, create client
	return consulKV.NewConsulClient()
}

// addNoParentTemplatesContext adds error details for missing parent templates
// to an existing error context. It lists any .tpl files discovered and provides
// naming guidance.
func addNoParentTemplatesContext(errorContext *errors.UIErrorContext, packPath string) {
	errorContext.Add(errors.UIContextErrorDetail, "No parent templates (*.nomad.tpl files) were found in the pack")
	errorContext.Add(errors.UIContextErrorSuggestion, "Parent templates must end with .nomad.tpl (e.g., app.nomad.tpl). Helper templates should start with _ (e.g., _helpers.tpl)")

	// list found template files
	templatesPath := filepath.Join(packPath, "templates")
	// we silently ignore errors from ReadDir since listing template files is
	// supplementary information. If the directory doesn't exist or can't be read,
	// the main error message about missing parent templates is still clear.
	if entries, err := os.ReadDir(templatesPath); err == nil {
		var templateFiles []string
		for _, entry := range entries {
			if !entry.IsDir() && strings.HasSuffix(entry.Name(), ".tpl") {
				templateFiles = append(templateFiles, entry.Name())
			}
		}
		if len(templateFiles) > 0 {
			errorContext.Add("Found Templates: ", strings.Join(templateFiles, ", "))
		} else {
			errorContext.Add("Found Templates: ", "none")
		}
	}
}
