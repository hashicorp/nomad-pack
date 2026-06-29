// Copyright IBM Corp. 2023, 2026
// SPDX-License-Identifier: MPL-2.0

package source

import (
	"context"

	"github.com/hashicorp/nomad-pack/sdk/pack"
	"github.com/hashicorp/nomad-pack/sdk/pack/variables"
)

// Priority constants define the precedence order for variable sources.
// Higher values take precedence over lower values when variables conflict.
//
// Precedence order (lowest to highest):
//   - Pack defaults
//   - External sources (--var-source), in command-line order
//   - Environment variables
//   - Variable files (-f/--var-file)
//   - CLI flags (--var)
//
// External sources rank below all local input: anything passed with --var,
// --var-file, or the environment overrides a value read from Consul, Vault, or
// Nomad. Each external source is assigned PriorityExternalBase plus its position
// on the command line, so when two sources supply the same variable the one
// given later wins.
const (
	PriorityExternalBase = 10
	PriorityEnv          = 1000
	PriorityFile         = 2000
	PriorityCLI          = 3000
)

// VariableSource represents a source of variables (CLI, file, env, external)
type VariableSource interface {
	// Name returns the unique identifier for this source
	Name() string

	// Priority returns the precedence level (higher = higher priority)
	Priority() int

	// Fetch retrieves variables for the given pack.
	// If a variable is not in the schema, it will be skipped (not returned).
	Fetch(ctx context.Context, packID pack.ID, schema map[variables.ID]*variables.Variable) ([]*variables.Variable, error)
}
