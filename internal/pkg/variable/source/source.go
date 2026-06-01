// Copyright IBM Corp. 2023, 2026
// SPDX-License-Identifier: MPL-2.0

package source

import (
	"context"

	"github.com/hashicorp/nomad-pack/sdk/pack"
	"github.com/hashicorp/nomad-pack/sdk/pack/variables"
)

// VariableSource represents a source of variables (CLI, file, env, external)
type VariableSource interface {
	// Name returns the unique identifier for this source
	Name() string

	// Priority returns the precedence level (higher = higher priority)
	Priority() int

	// Fetch retrieves variables for the given pack
	Fetch(ctx context.Context, packID pack.ID) ([]*variables.Variable, error)
}
