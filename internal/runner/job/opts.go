// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package job

import (
	"github.com/hashicorp/nomad/api"
)

func (r *Runner) newQueryOptsFromJob(job ParsedTemplate) *api.QueryOptions {
	opts := &api.QueryOptions{}
	if job.HasRegion() {
		opts.Region = *job.Job().Region
	}
	if job.HasNamespace() {
		opts.Namespace = *job.Job().Namespace
	}
	return opts
}

func (r *Runner) newQueryOptsFromClientJob(job *api.Job) *api.QueryOptions {
	opts := &api.QueryOptions{}
	if job.Region != nil {
		opts.Region = *job.Region
	}
	if job.Namespace != nil {
		opts.Namespace = *job.Namespace
	}
	return opts
}

func (r *Runner) newWriteOptsFromJob(job ParsedTemplate) *api.WriteOptions {
	opts := &api.WriteOptions{}
	if job.HasRegion() {
		opts.Region = *job.Job().Region
	}
	if job.HasNamespace() {
		opts.Namespace = *job.Job().Namespace
	}
	return opts
}

func (r *Runner) newWriteOptsFromClientJob(job *api.Job) *api.WriteOptions {
	opts := &api.WriteOptions{}
	if job.Region != nil {
		opts.Region = *job.Region
	}
	if job.Namespace != nil {
		opts.Namespace = *job.Namespace
	}
	return opts
}
