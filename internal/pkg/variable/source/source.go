// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

// Package source provides a pluggable architecture for variable sources in nomad-pack.
// The source package enables nomad-pack to fetch variables from multiple sources
// (environment, files, CLI flags, and future external sources like Consul KV, Vault,
// or Nomad Variables) with priority-based resolution.
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
