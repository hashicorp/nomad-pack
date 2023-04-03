// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package job

import (
	"testing"

	v1client "github.com/hashicorp/nomad-openapi/clients/go/v1"
	"github.com/hashicorp/nomad-pack/internal/runner"
	"github.com/stretchr/testify/assert"
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
				Name: stringToPtr("foobar"),
			},
			expectedOutputJob: &v1client.Job{
				Name: stringToPtr("foobar"),
				Meta: mapToPtr(map[string]string{
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
			assert.Equal(t, tc.expectedOutputJob, tc.inputJob, tc.name)
		})
	}
}

func mapToPtr(m map[string]string) *map[string]string { return &m }
func stringToPtr(s string) *string                    { return &s }
