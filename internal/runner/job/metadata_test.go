// Copyright IBM Corp. 2021, 2025
// SPDX-License-Identifier: MPL-2.0

package job

import (
	"testing"

	"github.com/shoenig/test/must"

	"github.com/hashicorp/nomad-pack/internal/runner"
)

func TestDeployer_setHCLMeta(t *testing.T) {
	defaultConfig := &runner.Config{
		PackName:       "foobar",
		PathPath:       "/opt/src/foobar",
		PackRef:        "123456",
		DeploymentName: "foobar@123456",
		RegistryName:   "default",
	}
	testCases := []struct {
		desc        string
		inputRunner *Runner
		inputJob    string
		expectedJob string
	}{
		{
			desc: "empty job returns empty job",
			inputRunner: &Runner{
				runnerCfg: defaultConfig,
			},
			inputJob:    "",
			expectedJob: "",
		},
		{
			desc: "job without meta defined gets meta attribute added",
			inputRunner: &Runner{
				runnerCfg: defaultConfig,
			},
			inputJob: "job \"basic\" {}",
			expectedJob: `job "basic" { meta = {
  "pack.deployment_name" = "foobar@123456"
  "pack.job"             = "basic"
  "pack.name"            = "foobar"
  "pack.path"            = "/opt/src/foobar"
  "pack.registry"        = "default"
  "pack.version"         = "123456"
  }
}`,
		},
		{
			desc: "existing meta attribute is merged with nomad-pack metadata",
			inputRunner: &Runner{
				runnerCfg: defaultConfig,
			},
			inputJob: "job \"foobar\" { meta = { \"other.stuff\" = \"foobar\" \n \"thing\" = \"baz\" } }",
			expectedJob: `job "foobar" { meta = {
  "other.stuff"          = "foobar"
  "pack.deployment_name" = "foobar@123456"
  "pack.job"             = "foobar"
  "pack.name"            = "foobar"
  "pack.path"            = "/opt/src/foobar"
  "pack.registry"        = "default"
  "pack.version"         = "123456"
  thing                  = "baz"
} }`,
		},
		{
			desc: "existing meta block is merged with nomad-pack metadata",
			inputRunner: &Runner{
				runnerCfg: defaultConfig,
			},
			inputJob: "job \"foobar\" {\n meta {\n other = \"foobar\" \n thing = \"baz\" \n}\n }",
			expectedJob: `job "foobar" {
  meta = {
    other                  = "foobar"
    "pack.deployment_name" = "foobar@123456"
    "pack.job"             = "foobar"
    "pack.name"            = "foobar"
    "pack.path"            = "/opt/src/foobar"
    "pack.registry"        = "default"
    "pack.version"         = "123456"
    thing                  = "baz"
  }
}`,
		},
		{
			desc: "nested meta block is untouched",
			inputRunner: &Runner{
				runnerCfg: defaultConfig,
			},
			inputJob: "job \"foobar\" {\n task \"server\" {\n meta {\n other = \"foobar\" \n thing = \"baz\" \n}\n }\n }",
			expectedJob: `job "foobar" {
  task "server" {
    meta {
      other = "foobar"
      thing = "baz"
    }
  }
  meta = {
    "pack.deployment_name" = "foobar@123456"
    "pack.job"             = "foobar"
    "pack.name"            = "foobar"
    "pack.path"            = "/opt/src/foobar"
    "pack.registry"        = "default"
    "pack.version"         = "123456"
  }
}`,
		},
		{
			desc: "multiple meta blocks returns one merged meta attribute and one block ignored, as invalid HCL",
			inputRunner: &Runner{
				runnerCfg: defaultConfig,
			},
			inputJob: "job \"foobar\" {\n meta {\n other = \"foobar\" \n thing = \"baz\" \n}\n meta {\n other2 = \"foobar2\" \n thing2 = \"baz2\" \n}\n }",
			expectedJob: `job "foobar" {
  meta {
    other2 = "foobar2"
    thing2 = "baz2"
  }
  meta = {
    other                  = "foobar"
    "pack.deployment_name" = "foobar@123456"
    "pack.job"             = "foobar"
    "pack.name"            = "foobar"
    "pack.path"            = "/opt/src/foobar"
    "pack.registry"        = "default"
    "pack.version"         = "123456"
    thing                  = "baz"
  }
}`,
		},
		{
			desc: "multiple meta attributes returns unchanged job, as invalid HCL",
			inputRunner: &Runner{
				runnerCfg: defaultConfig,
			},
			inputJob:    "job \"foobar\" {\n meta = {\n other = \"foobar\" \n thing = \"baz\" \n}\n meta = {\n other2 = \"foobar2\" \n thing2 = \"baz2\" \n}\n }",
			expectedJob: "job \"foobar\" {\n meta = {\n other = \"foobar\" \n thing = \"baz\" \n}\n meta = {\n other2 = \"foobar2\" \n thing2 = \"baz2\" \n}\n }",
		},
		{
			desc: "defined meta attribute & block returns merged meta attribute, and the block ignored, as invalid HCL",
			inputRunner: &Runner{
				runnerCfg: defaultConfig,
			},
			inputJob: "job \"foobar\" {\n meta {\n other = \"foobar\" \n thing = \"baz\" \n}\n meta = {\n \"other.2\" = \"foobar2\" \n \"thing.2\" = \"baz2\" \n}\n }",
			expectedJob: `job "foobar" {
  meta {
    other = "foobar"
    thing = "baz"
  }
  meta = {
    "other.2"              = "foobar2"
    "pack.deployment_name" = "foobar@123456"
    "pack.job"             = "foobar"
    "pack.name"            = "foobar"
    "pack.path"            = "/opt/src/foobar"
    "pack.registry"        = "default"
    "pack.version"         = "123456"
    "thing.2"              = "baz2"
  }
}`,
		},
	}
	for _, tc := range testCases {
		t.Run(tc.desc, func(t *testing.T) {
			result := tc.inputRunner.setHCLMeta(tc.inputJob)
			must.Eq(t, tc.expectedJob, result)
		})
	}

}
