// Copyright (c) HashiCorp, Inc.
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
	CheckIndex      uint64
	ConsulToken     string
	ConsulNamespace string
	VaultToken      string
	VaultNamespace  string
	EnableRollback  bool
	HCL1            bool
	PreserveCounts  bool
	PolicyOverride  bool
}

// PlanCLIConfig specifies the configuration that is used by the Nomad Pack
// plan command.
type PlanCLIConfig struct {
	HCL1           bool
	PolicyOverride bool
	Verbose        bool
	Diff           bool
}
