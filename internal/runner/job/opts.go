package job

import (
	v1client "github.com/hashicorp/nomad-openapi/clients/go/v1"
	v1 "github.com/hashicorp/nomad-openapi/v1"
)

// newQueryOptsFromJob merges job settings for region with the client's
// provided default QueryOpts. This MUST be run before canonicalizing the
// job or default values set as part of canonicalization will always
// opaque the client's (potentially) overridden defaults
func (r *Runner) newQueryOptsFromJob(job ParsedTemplate) *v1.QueryOpts {
	opts := r.newQueryOpts()
	if job.HasRegion() {
		opts.Region = job.Job().GetRegion()
	}
	if job.HasNamespace() {
		opts.Namespace = job.Job().GetNamespace()
	}
	return opts
}

func (r *Runner) newQueryOptsFromClientJob(job *v1client.Job) *v1.QueryOpts {
	opts := r.newQueryOpts()
	if job.HasRegion() {
		opts.Region = job.GetRegion()
	}
	if job.HasNamespace() {
		opts.Namespace = job.GetNamespace()
	}
	return opts
}

// newWriteOptsFromJob merges job settings for region with the client's
// provided default WriteOpts. This MUST be run before canonicalizing the
// job or default values set as part of canonicalization will always
// opaque the client's (potentially) overridden defaults
func (r *Runner) newWriteOptsFromJob(job ParsedTemplate) *v1.WriteOpts {
	opts := r.newWriteOpts()
	if job.HasRegion() {
		opts.Region = job.Job().GetRegion()
	}
	if job.HasNamespace() {
		opts.Namespace = job.Job().GetNamespace()
	}
	return opts
}

func (r *Runner) newWriteOptsFromClientJob(job *v1client.Job) *v1.WriteOpts {
	opts := r.newWriteOpts()
	if job.HasRegion() {
		opts.Region = job.GetRegion()
	}
	if job.HasNamespace() {
		opts.Namespace = job.GetNamespace()
	}
	return opts
}

func (r *Runner) newQueryOpts() *v1.QueryOpts {
	return r.client.QueryOpts()
}

func (r *Runner) newWriteOpts() *v1.WriteOpts {
	return r.client.WriteOpts()
}
