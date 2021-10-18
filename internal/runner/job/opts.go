package job

import (
	"os"

	v1client "github.com/hashicorp/nomad-openapi/clients/go/v1"
	v1 "github.com/hashicorp/nomad-openapi/v1"
)

func newQueryOptsFromJob(job *v1client.Job) *v1.QueryOpts {
	opts := newQueryOpts()
	if job.Region != nil {
		opts.Region = *job.Region
	}
	if job.Namespace != nil {
		opts.Namespace = *job.Namespace
	}
	return opts
}

func newWriteOptsFromJob(job *v1client.Job) *v1.WriteOpts {
	opts := newWriteOpts()
	if job.Region != nil {
		opts.Region = *job.Region
	}
	if job.Namespace != nil {
		opts.Namespace = *job.Namespace
	}
	return opts
}

func newQueryOpts() *v1.QueryOpts {
	opts := v1.QueryOpts{}
	return opts.WithAuthToken(os.Getenv("NOMAD_TOKEN"))
}

func newWriteOpts() *v1.WriteOpts {
	opts := v1.WriteOpts{}
	return opts.WithAuthToken(os.Getenv("NOMAD_TOKEN"))
}
