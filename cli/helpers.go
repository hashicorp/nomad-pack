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
	"github.com/hashicorp/nomad-pack/internal/pkg/renderer"
	"github.com/hashicorp/nomad-pack/terminal"
)

const (
	NomadCache            = ".nomad/packs"
	DefaultRegistryName   = "default"
	DefaultRegistrySource = "git@github.com:hashicorp/nomad-pack-registry.git"
)

// Run, plan, stop/destroy, and status all need access to pack, registry,
// and deployment names. This allows those commands to share helper methods
// that rely on those fields.
type PackInfo interface {
	PackName() string
	RegistryName() string
	DeploymentName() string
}

// Add metadata to the job for in cluster querying and management
func setJobMeta(packInfo interface{}, job *v1client.Job, packVersion string) error {
	info, ok := packInfo.(PackInfo)
	if !ok {
		return fmt.Errorf("failed to assert pack info, unsuitable type %T", packInfo)
	}
	packName := info.PackName()
	registryName := info.RegistryName()
	deploymentName := info.DeploymentName()

	jobMeta := make(map[string]string)

	// If current job meta isn't nil, use that instead
	if job.Meta != nil {
		jobMeta = *job.Meta
	}
	// Add the Nomad Pack custom metadata.
	jobMeta[packPathKey], _ = getPackPath(registryName, packName)
	jobMeta[packNameKey] = packName
	jobMeta[packRegistryKey] = registryName
	jobMeta[packDeploymentNameKey] = deploymentName
	jobMeta[packJobKey] = *job.Name
	jobMeta[packVersionKey] = packVersion

	// Replace the job metadata with our modified version.
	job.Meta = &jobMeta
	return nil
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
func parseRepoFromPackName(packName string) (string, string, error) {
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
	repo := s[0]
	pack := s[1]
	return repo, pack, nil
}

func getRepoPath(repoName string, ui terminal.UI, errCtx *errors.UIErrorContext) (string, error) {
	homedir, err := os.UserHomeDir()
	if err != nil {
		ui.ErrorWithContext(err, fmt.Sprintf("cannot determine user home directory"), errCtx.GetAll()...)
		return "", err
	}
	globalCacheDir := path.Join(homedir, NomadCache)
	repoPath := path.Join(globalCacheDir, repoName)

	if _, err := os.Stat(repoPath); os.IsNotExist(err) {
		ui.ErrorWithContext(err, fmt.Sprintf("registry %s does not exist at path: %s", repoName, repoPath), errCtx.GetAll()...)
	} else if err != nil {
		// some other error
		ui.ErrorWithContext(err, fmt.Sprintf("cannot read registry %s at path: %s", repoName, repoPath), errCtx.GetAll()...)
	}
	return repoPath, nil
}

func getPackPath(repoName string, packName string) (string, error) {
	homedir, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return path.Join(homedir, NomadCache, repoName, packName), nil
}

// Returns an error if the pack doesn't exist in the specified repo
func verifyPackExist(ui terminal.UI, packName, repoPath string, errCtx *errors.UIErrorContext) error {
	packPath := path.Join(repoPath, packName)
	if _, err := os.Stat(packPath); os.IsNotExist(err) {
		ui.ErrorWithContext(err, "failed to find pack", errCtx.GetAll()...)
		return err
	}

	return nil
}

// generatePackManager is used to generate the pack manager for this Nomad Pack
// run.
func generatePackManager(c *baseCommand, client *v1.Client, repoPath, pack string) *manager.PackManager {
	cfg := manager.Config{
		Path:            path.Join(repoPath, pack),
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

func getDeployedPackJobs(jobsApi *v1.Jobs, packName string, deploymentName string) ([]*v1client.Job, error) {
	opts := &v1.QueryOpts{}
	jobs, _, err := jobsApi.GetJobs(opts.Ctx())
	if err != nil {
		return nil, fmt.Errorf("error finding jobs for pack %s: %s", packName, err)
	}

	if len(jobs) == 0 {
		return nil, fmt.Errorf("no job(s) found")
	}

	var packJobs []*v1client.Job
	for _, jobStub := range jobs {
		job, _, err := jobsApi.GetJob(opts.Ctx(), *jobStub.ID)
		if err != nil {
			return nil, fmt.Errorf("error retrieving job %s for pack %s: %s", *job.ID, packName, err)
		}

		if job.Meta != nil {
			jobMeta := *job.Meta
			jobPackName, ok := jobMeta[packDeploymentNameKey]
			if ok && jobPackName == deploymentName {
				packJobs = append(packJobs, job)
			}
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
