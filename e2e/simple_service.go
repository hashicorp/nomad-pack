package main

import (
	"github.com/hashicorp/nomad-pack/cli"
	"github.com/hashicorp/nomad/e2e/framework"
)

func init() {
	framework.AddSuites(&framework.TestSuite{
		Component:   "simple_service",
		CanRunLocal: true,
		Cases: []framework.TestCase{
			new(SimpleServiceTestCase),
		},
	})
}

type SimpleServiceTestCase struct {
	framework.TC
}

func (tc *SimpleServiceTestCase) TestExample(f *framework.F) {
	defer func() {
		exitCode := cli.DestroyCmd().Run([]string{"simple_service"})
		f.Equal(0, exitCode)
	}()

	f.T().Log("Logging simple_service pack")

	cli.RunCmd().Run([]string{"simple_service"})

	JobExists(f, tc.Nomad(), "simple_service")
}
