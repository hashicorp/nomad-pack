package cli

import (
	"fmt"
	"os"

	v1client "github.com/hashicorp/nomad-openapi/clients/go/v1"
	v1 "github.com/hashicorp/nomad-openapi/v1"
	"github.com/hashicorp/nomad-pack/internal/pkg/cache"
	"github.com/hashicorp/nomad-pack/internal/pkg/errors"
	"github.com/hashicorp/nomad-pack/internal/pkg/manager"
	"github.com/hashicorp/nomad-pack/internal/pkg/renderer"
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

	return
}

// generatePackManager is used to generate the pack manager for this Nomad Pack run.
func generatePackManager(c *baseCommand, client *v1.Client, packCfg *cache.PackConfig) *manager.PackManager {
	// TODO: Refactor to have manager use cache.
	cfg := manager.Config{
		Path:            packCfg.Path,
		VariableFiles:   c.varFiles,
		VariableCLIArgs: c.vars,
	}
	return manager.NewPackManager(&cfg, client)
}

func registryTable() *terminal.Table {
	return terminal.NewTable("PACK NAME", "REF", "METADATA VERSION", "REGISTRY", "REGISTRY_URL")
}

func emptyRegistryTableRow(cachedRegistry *cache.Registry) []terminal.TableEntry {
	return []terminal.TableEntry{
		// blank pack name
		{
			Value: "",
		},
		// blank revision
		{
			Value: "",
		},
		// blank metadata version
		{
			Value: "",
		},
		// CachedRegistry name - user defined alias or registry URL slug
		{
			Value: cachedRegistry.Name,
		},
		// The cachedRegistry URL from where the registryPack was cloned
		{
			Value: cachedRegistry.Source,
		},
		//// TODO: The app version
		//{
		//	Value: registryPack.Metadata.App.Version,
		//},
	}
}

func registryPackRow(cachedRegistry *cache.Registry, cachedPack *cache.Pack) []terminal.TableEntry {
	return []terminal.TableEntry{
		// The Name of the registryPack
		{
			Value: cachedPack.Name(),
		},
		// The revision from where the registryPack was cloned
		{
			Value: cachedPack.Ref,
		},
		// The metadata version
		{
			Value: cachedPack.Metadata.Pack.Version,
		},
		// CachedRegistry name - user defined alias or registry URL slug
		{
			Value: cachedRegistry.Name,
		},
		// The cachedRegistry URL from where the registryPack was cloned
		{
			Value: cachedRegistry.Source,
		},
		//// TODO: The app version
		//{
		//	Value: registryPack.Metadata.App.Version,
		//},
	}
}

// TODO: This needs to be on a domain specific pkg rather than a UI helpers file.
// This will be possible once we create a logger interface that can be passed
// between layers.
// Uses the pack manager to parse the templates, override template variables with var files
// and cli vars as applicable
func renderPack(manager *manager.PackManager, ui terminal.UI, errCtx *errors.UIErrorContext) (*renderer.Rendered, error) {
	r, err := manager.ProcessTemplates()
	if err != nil {
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
// Uses open api client to parse rendered hcl templates to
// open api jobs to send to nomad
func parseJob(ui terminal.UI, hcl string, hclV1 bool, errCtx *errors.UIErrorContext) (*v1client.Job, error) {
	// instantiate client to parse hcl
	c, err := v1.NewClient()
	if err != nil {
		ui.ErrorWithContext(err, "failed to initialize client", errCtx.GetAll()...)
		return nil, err
	}

	opts := newQueryOpts()
	parsedJob, err := c.Jobs().Parse(opts.Ctx(), hcl, true, hclV1)
	if err != nil {
		ui.ErrorWithContext(err, "failed to parse job specification", errCtx.GetAll()...)
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
func getPackJobsByDeploy(jobsApi *v1.Jobs, cfg *cache.PackConfig, deploymentName string) ([]*v1client.Job, error) {
	opts := newQueryOpts()
	jobs, _, err := jobsApi.GetJobs(opts.Ctx())
	if err != nil {
		return nil, fmt.Errorf("error finding jobs for pack %s: %s", cfg.Name, err)
	}
	if len(*jobs) == 0 {
		return nil, fmt.Errorf("no job(s) found")
	}

	var packJobs []*v1client.Job
	hasOtherDeploys := false
	for _, jobStub := range *jobs {
		nomadJob, _, err := jobsApi.GetJob(opts.Ctx(), *jobStub.ID)
		if err != nil {
			return nil, fmt.Errorf("error retrieving job %s for pack %s: %s", *nomadJob.ID, cfg.Name, err)
		}

		if nomadJob.Meta != nil {
			jobMeta := *nomadJob.Meta
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
func generateRunner(client *v1.Client, packType, cliCfg interface{}, runnerCfg *runner.Config) (runner.Runner, error) {

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
func getDeployedPacks(jobsApi *v1.Jobs) (map[string]map[string]struct{}, error) {
	opts := newQueryOpts()
	jobStubs, _, err := jobsApi.GetJobs(opts.Ctx())
	if err != nil {
		return nil, fmt.Errorf("error finding jobs: %s", err)
	}

	packRegistryMap := map[string]map[string]struct{}{}
	for _, jobStub := range *jobStubs {
		nomadJob, _, err := jobsApi.GetJob(opts.Ctx(), *jobStub.ID)
		if err != nil {
			return nil, fmt.Errorf("error retrieving job %s: %s", *nomadJob.ID, err)
		}

		if nomadJob.Meta != nil {
			jobMeta := *nomadJob.Meta
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
func getDeployedPackJobs(jobsApi *v1.Jobs, cfg *cache.PackConfig, deploymentName string) ([]JobStatusInfo, []JobStatusError, error) {
	opts := newQueryOpts()
	jobs, _, err := jobsApi.GetJobs(opts.Ctx())
	if err != nil {
		return nil, nil, fmt.Errorf("error finding jobs for pack %s: %s", cfg.Name, err)
	}

	var packJobs []JobStatusInfo
	var jobErrs []JobStatusError
	for _, jobStub := range *jobs {
		nomadJob, _, err := jobsApi.GetJob(opts.Ctx(), *jobStub.ID)
		if err != nil {
			jobErrs = append(jobErrs, JobStatusError{
				jobID:    *jobStub.ID,
				jobError: err,
			})
			continue
		}

		if nomadJob.Meta != nil {
			jobMeta := *nomadJob.Meta
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

func newQueryOpts() *v1.QueryOpts {
	opts := v1.QueryOpts{}
	return opts.WithAuthToken(os.Getenv("NOMAD_TOKEN"))
}

func newWriteOpts() *v1.WriteOpts {
	opts := v1.WriteOpts{}
	return opts.WithAuthToken(os.Getenv("NOMAD_TOKEN"))
}
