// Copyright IBM Corp. 2023, 2026
// SPDX-License-Identifier: MPL-2.0

package source

import (
	"context"

	"github.com/hashicorp/nomad-pack/sdk/pack"
	"github.com/hashicorp/nomad-pack/sdk/pack/variables"
)

// BaseSource provides common functionality for simple variable sources that
// wrap existing PackIDKeyedVarMap data structures.
type BaseSource struct {
	name     string
	priority int
	vars     variables.PackIDKeyedVarMap
}

// NewBaseSource creates a new base source with the given name, priority, and variables.
func NewBaseSource(name string, priority int, vars variables.PackIDKeyedVarMap) *BaseSource {
	return &BaseSource{
		name:     name,
		priority: priority,
		vars:     vars,
	}
}

// Name returns the unique identifier for this source.
func (b *BaseSource) Name() string {
	return b.name
}

// Priority returns the precedence level (higher = higher priority).
func (b *BaseSource) Priority() int {
	return b.priority
}

// Fetch retrieves variables for the given pack from the wrapped map.
// Returns an empty slice if the pack is not found or vars is nil.
//
// Unlike external sources, BaseSource does not filter by schema. External sources (e.g. Consul) do their own schema filtering.
func (b *BaseSource) Fetch(ctx context.Context, packID pack.ID, schema map[variables.ID]*variables.Variable) ([]*variables.Variable, error) {
	if b.vars == nil {
		return make([]*variables.Variable, 0), nil
	}

	packVars, exists := b.vars[packID]
	if !exists {
		return make([]*variables.Variable, 0), nil
	}

	return packVars, nil
}

// NewEnvSource creates a new environment variable source.
// This is a convenience constructor for BaseSource with name "env".
func NewEnvSource(priority int, vars variables.PackIDKeyedVarMap) VariableSource {
	return NewBaseSource("env", priority, vars)
}

// NewFileSource creates a new file variable source.
// This is a convenience constructor for BaseSource with name "file".
func NewFileSource(priority int, vars variables.PackIDKeyedVarMap) VariableSource {
	return NewBaseSource("file", priority, vars)
}

// NewCLISource creates a new CLI variable source.
// This is a convenience constructor for BaseSource with name "cli".
func NewCLISource(priority int, vars variables.PackIDKeyedVarMap) VariableSource {
	return NewBaseSource("cli", priority, vars)
}
