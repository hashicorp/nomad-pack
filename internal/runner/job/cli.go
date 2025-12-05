// Copyright IBM Corp. 2021, 2025
// SPDX-License-Identifier: MPL-2.0

package job

// CLIConfig contains all possible configurations required by the Nomad Pack
// CLI in order to render, plan, run, and destroy job templates.
type CLIConfig struct {
	RunConfig  *RunCLIConfig
	PlanConfig *PlanCLIConfig
}

// RunCLIConfig specifies the configuration that is used by the Nomad Pack run
// command.
type RunCLIConfig struct {
	CheckIndex        uint64
	ConsulToken       string
	ConsulNamespace   string
	VaultToken        string
	VaultNamespace    string
	EnableRollback    bool
	PreserveCounts    bool
	PreserveResources bool
	PolicyOverride    bool
}

// PlanCLIConfig specifies the configuration that is used by the Nomad Pack
// plan command.
type PlanCLIConfig struct {
	PolicyOverride bool
	Verbose        bool
	Diff           bool
}
