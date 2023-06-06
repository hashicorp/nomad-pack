// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package job

import (
	"testing"

	"github.com/hashicorp/nomad/api"
	"github.com/shoenig/test/must"

	"github.com/hashicorp/nomad-pack/internal/pkg/helper/pointer"
	"github.com/hashicorp/nomad-pack/internal/runner"
)

func TestDeployer_setJobMeta(t *testing.T) {
	testCases := []struct {
		inputRunner       *Runner
		inputJob          *api.Job
		expectedOutputJob *api.Job
		name              string
	}{
		{
			inputRunner: &Runner{
				runnerCfg: &runner.Config{
					PackName:       "foobar",
					PathPath:       "/opt/src/foobar",
					PackRef:        "123456",
					DeploymentName: "foobar@123456",
					RegistryName:   "default",
				},
			},
			inputJob: &api.Job{
				Name: pointer.Of("foobar"),
			},
			expectedOutputJob: &api.Job{
				Name: pointer.Of("foobar"),
				Meta: map[string]string{
					PackPathKey:           "/opt/src/foobar",
					PackNameKey:           "foobar",
					PackRegistryKey:       "default",
					PackDeploymentNameKey: "foobar@123456",
					PackJobKey:            "foobar",
					PackRefKey:            "123456",
				},
			},
			name: "nil input meta",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			tc.inputRunner.setJobMeta(tc.inputJob)
			must.Eq(t, tc.expectedOutputJob, tc.inputJob)
		})
	}
}
