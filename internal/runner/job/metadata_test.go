// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package job

import (
	"testing"

	v1client "github.com/hashicorp/nomad-openapi/clients/go/v1"
	"github.com/shoenig/test/must"

	"github.com/hashicorp/nomad-pack/internal/runner"
	"github.com/hashicorp/nomad-pack/sdk/helper"
)

func TestDeployer_setJobMeta(t *testing.T) {
	testCases := []struct {
		inputRunner       *Runner
		inputJob          *v1client.Job
		expectedOutputJob *v1client.Job
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
			inputJob: &v1client.Job{
				Name: helper.StringToPtr("foobar"),
			},
			expectedOutputJob: &v1client.Job{
				Name: helper.StringToPtr("foobar"),
				Meta: helper.MapToPtr(map[string]string{
					PackPathKey:           "/opt/src/foobar",
					PackNameKey:           "foobar",
					PackRegistryKey:       "default",
					PackDeploymentNameKey: "foobar@123456",
					PackJobKey:            "foobar",
					PackRefKey:            "123456",
				}),
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
