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
// Priority Order (lowest to highest):
//   - Environment variables (10)
//   - Variable files (20)
//   - Nomad Variables (23)
//   - Vault KV (24)
//   - Consul KV (25)
//   - CLI flags (30)
const (
	PriorityEnv    = 10
	PriorityFile   = 20
	PriorityNomad  = 23
	PriorityVault  = 24
	PriorityConsul = 25
	PriorityCLI    = 30
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
