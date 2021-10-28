package main

import (
	"github.com/hashicorp/nomad/api"
	"github.com/hashicorp/nomad/e2e/framework"
)

func JobExists(f *framework.F, client *api.Client, jobID string) bool {
	jobs, _, err := client.Jobs().List(nil)

	f.NoError(err)
	f.NotEmpty(jobs)

	for _, job := range jobs {
		if job.ID == jobID {
			return true
		}
	}

	return false
}
