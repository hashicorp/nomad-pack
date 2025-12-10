// Copyright IBM Corp. 2021, 2025
// SPDX-License-Identifier: MPL-2.0

package cli

import (
	"fmt"
	"os"
	"strings"

	"github.com/hashicorp/nomad/api"

	"github.com/hashicorp/nomad-pack/internal/pkg/cache"
	"github.com/hashicorp/nomad-pack/internal/pkg/errors"
	"github.com/hashicorp/nomad-pack/internal/pkg/manager"
	"github.com/hashicorp/nomad-pack/internal/pkg/renderer"
	"github.com/hashicorp/nomad-pack/internal/pkg/variable/parser"
	"github.com/hashicorp/nomad-pack/internal/runner"
	"github.com/hashicorp/nomad-pack/internal/runner/job"
	"github.com/hashicorp/nomad-pack/terminal"
)

// get an initialized error context for a command that accepts pack args.
func initPackCommand(cfg *cache.PackConfig) (errorContext *errors.UIErrorContext) {
	cfg.Init()

	// Generate our UI error context.
	errorContext = errors.NewUIErrorContext()
	errorContext.Add(errors.UIContextPrefixRegistryName, cfg.Registry)
	errorContext.Add(errors.UIContextPrefixPackName, cfg.Name)
	errorContext.Add(errors.UIContextPrefixPackRef, cfg.Ref)
	if cfg.Registry == cache.DevRegistryName && cfg.Ref == cache.DevRef {
		errorContext.Add(errors.UIContextPrefixPackPath, cfg.Path)
	}
	return
}

// generatePackManager is used to generate the pack manager for this Nomad Pack run.
func generatePackManager(c *baseCommand, client *api.Client, packCfg *cache.PackConfig) *manager.PackManager {
	// TODO: Refactor to have manager use cache.
	cfg := manager.Config{
		Path:            packCfg.Path,
		VariableFiles:   c.varFiles,
		VariableCLIArgs: c.vars,
		VariableEnvVars: c.envVars,
		AllowUnsetVars:  c.allowUnsetVars,
		UseParserV1:     c.useParserV1,
	}
	return manager.NewPackManager(&cfg, client)
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

func registryTableRow(cachedRegistry *cache.Registry) []string {
	return []string{
		cachedRegistry.Name,
		formatSHA1Reference(cachedRegistry.Ref),
		formatSHA1Reference(cachedRegistry.LocalRef),
		cachedRegistry.Source,
	}
}

func registryPackRow(cachedRegistry *cache.Registry, cachedPack *cache.Pack) []string {
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
func registryName(cr *cache.Registry) string {
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

func packRow(cachedRegistry *cache.Registry, cachedPack *cache.Pack) []string {
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
func getDeploymentName(c *baseCommand, cfg *cache.PackConfig) string {
	if c.deploymentName == "" {
		return cache.AppendRef(cfg.Name, cfg.Ref)
	}
	return c.deploymentName
}

// TODO: Move to a domain specific package.
func getPackJobsByDeploy(c *api.Client, cfg *cache.PackConfig, deploymentName string) ([]*api.Job, error) {
	jobsApi := c.Jobs()
	jobs, _, err := jobsApi.List(&api.QueryOptions{})
	if err != nil {
		return nil, fmt.Errorf("error finding jobs for pack %s: %s", cfg.Name, err)
	}
	if len(jobs) == 0 {
		return nil, errors.New("no job(s) found")
	}

	var packJobs []*api.Job
	hasOtherDeploys := false
	for _, jobStub := range jobs {
		nomadJob, _, err := jobsApi.Info(jobStub.ID, &api.QueryOptions{})
		if err != nil {
			return nil, fmt.Errorf("error retrieving job %s for pack %s: %s", *nomadJob.ID, cfg.Name, err)
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
		nomadJob, _, err := jobsApi.Info(jobStub.ID, &api.QueryOptions{})
		if err != nil {
			return nil, fmt.Errorf("error retrieving job %s: %s", *nomadJob.ID, err)
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
func getDeployedPackJobs(c *api.Client, cfg *cache.PackConfig, deploymentName string) ([]JobStatusInfo, []JobStatusError, error) {
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
