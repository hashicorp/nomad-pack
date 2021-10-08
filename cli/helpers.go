package cli

import (
	stdErrors "errors"
	"fmt"
	"os"
	"path"
	"strings"

	gg "github.com/hashicorp/go-getter"
	v1client "github.com/hashicorp/nomad-openapi/clients/go/v1"
	v1 "github.com/hashicorp/nomad-openapi/v1"
	"github.com/hashicorp/nomad-pack/internal/pkg/errors"
	"github.com/hashicorp/nomad-pack/internal/pkg/manager"
	"github.com/hashicorp/nomad-pack/internal/pkg/registry"
	"github.com/hashicorp/nomad-pack/internal/pkg/renderer"
	"github.com/hashicorp/nomad-pack/internal/runner"
	"github.com/hashicorp/nomad-pack/internal/runner/job"
	"github.com/hashicorp/nomad-pack/terminal"
)

const (
	NomadCache            = ".nomad/packs"
	DefaultRegistryName   = "default"
	DefaultRegistrySource = "git@github.com:hashicorp/nomad-pack-registry.git"
)

// log is returns an injectable log function that allows lower level packages to
// report publish log information without creating a dependency on the terminal.UI
// in lower level packages.
func log(ui terminal.UI) func(string) {
	l := func(message string) {
		ui.Info(message)
	}

	return l
}

// get the global cache directory
func globalCacheDir() (string, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}

	return path.Join(homeDir, NomadCache), nil
}

// get the default registry directory
func defaultRegistryDir() (string, error) {
	globalCache, err := globalCacheDir()
	if err != nil {
		return "", err
	}
	return path.Join(globalCache, DefaultRegistryName), nil
}

func addRegistry(cacheDir, from, alias, target string, ui terminal.UI) error {
	// Add the registry or registry target to the global cache
	newRegistry, err := registry.AddFromGitURL(cacheDir, from, alias, target, log(ui))
	if err != nil {
		ui.ErrorWithContext(err, "error adding registry")
		return err
	}

	// If subprocess fails to add any packs, report this to the user.
	if len(newRegistry.Packs) == 0 {
		// TODO: Should this be an error?
		ui.Info("no packs added - see output for reason")
		return nil
	}

	// Initialize output table
	table := registryTable()
	var successfulPack *registry.CachedPack
	// If only targeting a single pack, only output a single row
	if target != "" {
		// It is safe to target pack 0 here because registry.AddFromGitURL will
		// ensure only the target pack is returned.
		tableRow := registryPackRow(newRegistry, newRegistry.Packs[0])
		table.Rows = append(table.Rows, tableRow)
	} else {
		for _, registryPack := range newRegistry.Packs {
			tableRow := registryPackRow(newRegistry, registryPack)
			table.Rows = append(table.Rows, tableRow)
			// Grab a successful pack to show extra help text.
			if successfulPack == nil &&
				!strings.Contains(strings.ToLower(registryPack.CacheVersion), "invalid") {
				successfulPack = registryPack
			}
		}
	}

	ui.Info("CachedRegistry successfully added")
	ui.Table(table)

	if successfulPack != nil {
		ui.Info(fmt.Sprintf("Try running one the packs you just added liked this:  nomad-pack run %s:%s@%s", newRegistry.Name, successfulPack.Name(), successfulPack.CacheVersion))
	}

	return nil
}

func deleteRegistry(cacheDir, name, target string, ui terminal.UI) error {
	err := registry.DeleteFromCache(cacheDir, name, target, log(ui))
	if err != nil {
		ui.ErrorWithContext(err, "error deleting registry")
	}

	return nil
}
func listPacks(ui terminal.UI) error {
	// Get the global cache dir - may be configurable in the future, so using this
	// helper function rather than a direct reference to the CONST.
	registryPath, err := defaultRegistryDir()
	if err != nil {
		return err
	}

	// Initialize a table for a nice glint UI rendering
	table := registryTable()

	// Load the registry from the path
	registry, err := registry.LoadFromCache(registryPath)
	if err != nil {
		return err
	}

	// Iterate over packs in a registry and build a table row for
	// each cachedRegistry/pack entry at each version
	// If no packs, just show registry.
	if registry.Packs == nil || len(registry.Packs) == 0 {
		tableRow := emptyRegistryTableRow(registry)
		// append table row
		table.Rows = append(table.Rows, tableRow)
	} else {
		// Show registry/pack combo for each pack.
		for _, registryPack := range registry.Packs {
			tableRow := registryPackRow(registry, registryPack)
			// append table row
			table.Rows = append(table.Rows, tableRow)
		}
	}

	// Display output table
	ui.Table(table)

	return nil
}

// lists the currently configured global cache registries and their packs
func listRegistries(ui terminal.UI) error {
	// Get the global cache dir - may be configurable in the future, so using this
	// helper function rather than a direct reference to the CONST.
	globalCache, err := globalCacheDir()
	if err != nil {
		ui.ErrorWithContext(err, "error resolving global cache directory")
		return err
	}

	// Initialize a table for a nice glint UI rendering
	table := registryTable()

	// Load the list of registries.
	registries, err := registry.LoadAllFromCache(globalCache)
	if err != nil {
		ui.ErrorWithContext(err, "error listing registries")
		return err
	}

	// Iterate over the registries and build a table row for each cachedRegistry/pack
	// entry at each version. Hierarchically, this should equate to the default
	// cachedRegistry and all its peers.
	for _, cachedRegistry := range registries {
		// If no packs, just show registry.
		if cachedRegistry.Packs == nil || len(cachedRegistry.Packs) == 0 {
			tableRow := emptyRegistryTableRow(cachedRegistry)
			// append table row
			table.Rows = append(table.Rows, tableRow)
		} else {
			// Show registry/pack combo for each pack.
			for _, registryPack := range cachedRegistry.Packs {
				tableRow := registryPackRow(cachedRegistry, registryPack)
				// append table row
				table.Rows = append(table.Rows, tableRow)
			}
		}
	}

	// Display output table
	ui.Table(table)

	return nil
}

func registryTable() *terminal.Table {
	return terminal.NewTable("PACK NAME", "CACHE_VERSION", "METADATA VERSION", "REGISTRY", "REGISTRY_URL")
}

func emptyRegistryTableRow(cachedRegistry *registry.CachedRegistry) []terminal.TableEntry {
	return []terminal.TableEntry{
		// blank pack name
		{
			Value: "",
		},
		// blank pack version
		{
			Value: "",
		},
		// blank cache version
		{
			Value: "",
		},
		// CachedRegistry name - user defined alias or registry URL slug
		{
			Value: cachedRegistry.Name,
		},
		// The cachedRegistry URL from where the registryPack was cloned
		{
			Value: cachedRegistry.URL,
		},
		//// TODO: The app version
		//{
		//	Value: registryPack.Metadata.App.Version,
		//},
	}
}

func registryPackRow(cachedRegistry *registry.CachedRegistry, cachedPack *registry.CachedPack) []terminal.TableEntry {
	return []terminal.TableEntry{
		// The Name of the registryPack
		{
			Value: cachedPack.CacheName,
		},
		// The cachedRegistry Version from where the registryPack was cloned
		{
			Value: cachedPack.CacheVersion,
		},
		// The registryPack version
		{
			Value: cachedPack.Metadata.Pack.Version,
		},
		// CachedRegistry name - user defined alias or registry URL slug
		{
			Value: cachedRegistry.Name,
		},
		// The cachedRegistry URL from where the registryPack was cloned
		{
			Value: cachedRegistry.URL,
		},
		//// TODO: The app version
		//{
		//	Value: registryPack.Metadata.App.Version,
		//},
	}
}

func installRegistry(source string, destination string,
	ui terminal.UI, errCtx *errors.UIErrorContext) error {
	ui.Info("Initializing registry...")
	ui.Info(fmt.Sprintf("Downloading source from %s", source))
	ui.Info(fmt.Sprintf("Installing into: %s", destination))
	err := gg.Get(destination, source)
	if err != nil {
		ui.ErrorWithContext(err, fmt.Sprintf("could not install %s registry: %s", destination, source), errCtx.GetAll()...)
	}
	return err
}

func createDir(dir string, dirName string,
	ui terminal.UI, errCtx *errors.UIErrorContext) error {
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		err = os.MkdirAll(dir, 0755)
		if err != nil {
			ui.ErrorWithContext(err, fmt.Sprintf("cannot create %s: %s", dirName, dir), errCtx.GetAll()...)
		}
		ui.Info(fmt.Sprintf("created %s directory: %s", dirName, dir))
	} else if err != nil {
		// some other error
		ui.ErrorWithContext(err, fmt.Sprintf("cannot create %s: %s", dirName, dir), errCtx.GetAll()...)
	} else {
		ui.Info(fmt.Sprintf("%s directory already exists: %s", dirName, dir))
	}
	return nil
}

func createGlobalCache(ui terminal.UI, errCtx *errors.UIErrorContext) error {
	homedir, err := os.UserHomeDir()
	if err != nil {
		ui.ErrorWithContext(err, "error accessing home directory", errCtx.GetAll()...)
		return err
	}
	globalCacheDir := path.Join(homedir, NomadCache)
	return createDir(globalCacheDir, "global cache", ui, errCtx)
}

func installDefaultRegistry(ui terminal.UI, errCtx *errors.UIErrorContext) error {
	// Create default registry, if not exist
	homedir, err := os.UserHomeDir()
	if err != nil {
		ui.ErrorWithContext(err, "error accessing home directory", errCtx.GetAll()...)
		return err
	}
	defaultRegistryDir := path.Join(homedir, NomadCache, DefaultRegistryName)
	return installRegistry(DefaultRegistrySource, defaultRegistryDir, ui, errCtx)
}

func installUserRegistry(source string, name string, ui terminal.UI, errCtx *errors.UIErrorContext) error {
	homedir, err := os.UserHomeDir()
	if err != nil {
		ui.ErrorWithContext(err, "error accessing home directory", errCtx.GetAll()...)
		return err
	}
	userRegistryDir := path.Join(homedir, NomadCache, name)
	return installRegistry(source, userRegistryDir, ui, errCtx)
}

func parseRegistryAndPackName(packName string) (string, string, error) {
	if len(packName) == 0 {
		return "", "", stdErrors.New("invalid pack name: pack name cannot be empty")
	}

	s := strings.Split(packName, ":")

	if len(s) == 1 {
		return DefaultRegistryName, packName, nil
	}

	if len(s) > 2 {
		return "", "", fmt.Errorf("invalid pack name %s, pack name must be formatted 'registry:pack'", packName)
	}

	return s[0], s[1], nil
}

func getRegistryPath(repoName string, ui terminal.UI, errCtx *errors.UIErrorContext) (string, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		ui.ErrorWithContext(err, fmt.Sprintf("cannot determine user home directory"), errCtx.GetAll()...)
		return "", err
	}

	cacheDir := path.Join(homeDir, NomadCache)
	registryPath := path.Join(cacheDir, repoName)

	errCtx.Add(errors.UIContextPrefixRegistryPath, registryPath)

	// Attempt to stat the path. If we get an error, ensure we return this but
	// output specific errors to help debugging.
	_, err = os.Stat(registryPath)
	if err != nil {
		switch os.IsNotExist(err) {
		case true:
			ui.ErrorWithContext(err, "registry does not exist", errCtx.GetAll()...)
		default:
			ui.ErrorWithContext(err, "failed to read registry", errCtx.GetAll()...)
		}
		return "", err
	}

	return registryPath, nil
}

func getPackPath(repoName string, packName string) (string, error) {
	homedir, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return path.Join(homedir, NomadCache, repoName, packName), nil
}

// Returns an error if the pack doesn't exist in the specified repo
func verifyPackExist(ui terminal.UI, packName, registryPath string, errCtx *errors.UIErrorContext) error {
	packPath := path.Join(registryPath, packName)
	if _, err := os.Stat(packPath); os.IsNotExist(err) {
		ui.ErrorWithContext(err, "failed to find pack", errCtx.GetAll()...)
		return err
	}

	return nil
}

// generatePackManager is used to generate the pack manager for this Nomad Pack
// run.
func generatePackManager(c *baseCommand, client *v1.Client, registryName, pack string) *manager.PackManager {
	cfg := manager.Config{
		Path:            path.Join(registryName, pack),
		VariableFiles:   c.varFiles,
		VariableCLIArgs: c.vars,
	}
	return manager.NewPackManager(&cfg, client)
}

// Uses the pack manager to parse the templates, override template variables with var files
// and cli vars as applicable
func renderPack(manager *manager.PackManager, ui terminal.UI, errCtx *errors.UIErrorContext) (*renderer.Rendered, error) {
	r, err := manager.ProcessTemplates()
	if err != nil {
		ui.ErrorWithContext(err, "failed to process pack ", errCtx.GetAll()...)
		return nil, err
	}
	return r, nil
}

// Uses open api client to parse rendered hcl templates to
// open api jobs to send to nomad
func parseJob(ui terminal.UI, hcl string, hclV1 bool, errCtx *errors.UIErrorContext) (*v1client.Job, error) {
	// instantiate client to parse hcl
	c, err := v1.NewClient()
	if err != nil {
		ui.ErrorWithContext(err, "failed to initialize client", errCtx.GetAll()...)
		return nil, err
	}

	opts := &v1.QueryOpts{}
	job, err := c.Jobs().Parse(opts.Ctx(), hcl, true, hclV1)
	if err != nil {
		ui.ErrorWithContext(err, "failed to parse job specification", errCtx.GetAll()...)
		return nil, err
	}
	return job, nil
}

// Generates a deployment name if not specified. Default is pack@version.
func getDeploymentName(c *baseCommand, packName string, packVersion string) string {
	if c.deploymentName == "" {
		return fmt.Sprintf("%s@%s", packName, packVersion)
	}
	return c.deploymentName
}

// using jobsApi to get a job by job name
func getJob(jobsApi *v1.Jobs, jobName string, queryOpts *v1.QueryOpts) (*v1client.Job, *v1.QueryMeta, error) {
	result, meta, err := jobsApi.GetJob(queryOpts.Ctx(), jobName)
	if err != nil {
		return nil, nil, err
	}
	return result, meta, nil
}

func getPackJobsByDeploy(jobsApi *v1.Jobs, packName, deploymentName, registryName string) ([]*v1client.Job, error) {
	opts := &v1.QueryOpts{}
	jobs, _, err := jobsApi.GetJobs(opts.Ctx())
	if err != nil {
		return nil, fmt.Errorf("error finding jobs for pack %s: %s", packName, err)
	}
	if len(jobs) == 0 {
		return nil, fmt.Errorf("no job(s) found")
	}

	var packJobs []*v1client.Job
	hasOtherDeploys := false
	for _, jobStub := range jobs {
		nomadJob, _, err := jobsApi.GetJob(opts.Ctx(), *jobStub.ID)
		if err != nil {
			return nil, fmt.Errorf("error retrieving job %s for pack %s: %s", *nomadJob.ID, packName, err)
		}

		if nomadJob.Meta != nil {
			jobMeta := *nomadJob.Meta
			jobDeploymentName, ok := jobMeta[packDeploymentNameKey]

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
					if registryOk && packOk && jobRegistry == registryName && jobPack == packName {
						hasOtherDeploys = true
					}
				}
			}
		}

		if len(packJobs) == 0 && hasOtherDeploys {
			// TODO: the aesthetics here could be better. This error line is very long.
			return nil, fmt.Errorf(
				"pack %q running but not in deployment %q. Run \"nomad-pack status %s\" for more information",
				packName, deploymentName, packName)
		}
	}
	return packJobs, nil
}

// generate write options for openapi based on the nomad job.
// This just sets namespace and region, but we might want to
// extend it to include tokens, etc later
func newWriteOptsFromJob(job *v1client.Job) *v1.WriteOpts {
	opts := &v1.WriteOpts{}
	if job.Region != nil {
		opts.Region = *job.Region
	}
	if job.Namespace != nil {
		opts.Namespace = *job.Namespace
	}
	return opts
}

func newQueryOptsFromJob(job *v1client.Job) *v1.QueryOpts {
	opts := &v1.QueryOpts{}
	if job.Region != nil {
		opts.Region = *job.Region
	}
	if job.Namespace != nil {
		opts.Namespace = *job.Namespace
	}
	return opts
}

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
			return nil, fmt.Errorf("failed to assert correct config, unsiutable type %T", cliCfg)
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

func hasVarOverrides(c *baseCommand) bool {
	return len(c.varFiles) > 0 || len(c.vars) > 0
}

func getDeployedPacks(jobsApi *v1.Jobs) (map[string]map[string]struct{}, error) {
	opts := &v1.QueryOpts{}
	jobStubs, _, err := jobsApi.GetJobs(opts.Ctx())
	if err != nil {
		return nil, fmt.Errorf("error finding jobs: %s", err)
	}

	packRegistryMap := map[string]map[string]struct{}{}
	for _, jobStub := range jobStubs {
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

type JobStatusInfo struct {
	packName       string
	registryName   string
	deploymentName string
	jobID          string
	status         string
}

type JobStatusError struct {
	jobID    string
	jobError error
}

func getDeployedPackJobs(jobsApi *v1.Jobs, packName, registryName, deploymentName string) ([]JobStatusInfo, []JobStatusError, error) {
	opts := &v1.QueryOpts{}
	jobs, _, err := jobsApi.GetJobs(opts.Ctx())
	if err != nil {
		return nil, nil, fmt.Errorf("error finding jobs for pack %s: %s", packName, err)
	}

	var packJobs []JobStatusInfo
	var jobErrs []JobStatusError
	for _, jobStub := range jobs {
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
			if ok && jobPackName == packName {
				// Filter by deployment name if specified
				if deploymentName != "" {
					jobDeployName, deployOk := jobMeta[packDeploymentNameKey]
					if deployOk && jobDeployName != deploymentName {
						continue
					}
				}
				packJobs = append(packJobs, JobStatusInfo{
					packName:       packName,
					registryName:   registryName,
					deploymentName: jobMeta[packDeploymentNameKey],
					jobID:          *nomadJob.ID,
					status:         *nomadJob.Status,
				})
			}
		}
	}
	return packJobs, jobErrs, nil
}
