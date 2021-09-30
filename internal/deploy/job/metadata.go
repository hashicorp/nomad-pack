package job

import v1client "github.com/hashicorp/nomad-openapi/clients/go/v1"

const (
	packKey               = "pack"
	packDeploymentNameKey = "pack-deployment-name"
	packJobKey            = "pack-job"
	packVersionKey        = "pack-version"
)

// add metadata to the job for in cluster querying and management
func (d *Deployer) setJobMeta(job *v1client.Job) {
	jobMeta := make(map[string]string)

	// If current job meta isn't nil, use that instead
	if job.Meta != nil {
		jobMeta = *job.Meta
	}

	// Add the Nomad Pack custom metadata.
	jobMeta[packKey] = d.deployerCfg.PathPath
	jobMeta[packDeploymentNameKey] = d.deployerCfg.DeploymentName
	jobMeta[packJobKey] = *job.Name
	jobMeta[packVersionKey] = d.deployerCfg.PackVersion

	// Replace the job metadata with our modified version.
	job.Meta = &jobMeta
}
