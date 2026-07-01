// Copyright IBM Corp. 2023, 2026
// SPDX-License-Identifier: MPL-2.0

package source

import (
	"cmp"
	"context"
	"fmt"
	"maps"
	"slices"

	"github.com/hashicorp/nomad-pack/sdk/pack"
	"github.com/hashicorp/nomad-pack/sdk/pack/variables"
)

// Registry manages multiple variable sources and resolves them with
// priority-based precedence. Higher priority sources override lower
// priority sources for variables with the same name.
//
// Thread Safety: Registry is NOT thread-safe. It is designed for
// single-threaded CLI usage where sources are registered once during
// initialization and then resolved. Do not call Register() and Resolve()
// concurrently without external synchronization.
//
// Example usage:
//
//	registry := source.NewRegistry()
//	registry.Register(source.NewEnvSource(10, envVars))
//	registry.Register(source.NewFileSource(20, fileVars))
//	registry.Register(source.NewCLISource(30, cliVars))
//	vars, err := registry.Resolve(ctx, packID, schema)
type Registry struct {
	sources []VariableSource
}

// NewRegistry creates a new source registry.
func NewRegistry() *Registry {
	return &Registry{
		sources: make([]VariableSource, 0),
	}
}

// Register adds a source to the registry.
// Sources are automatically sorted by priority after registration.
// Source names are expected to be unique (enforced by source constructors).
func (r *Registry) Register(source VariableSource) {
	r.sources = append(r.sources, source)

	// Sort by priority immediately after adding (lower first, so higher priority overwrites)
	slices.SortFunc(r.sources, func(a, b VariableSource) int {
		return cmp.Compare(a.Priority(), b.Priority())
	})
}

// Resolve fetches and merges variables from all registered sources.
// Sources are processed in priority order (lowest to highest), with
// higher priority sources overwriting variables from lower priority sources.
// The schema parameter provides the expected type for each variable, allowing
// sources to perform schema-aware type conversion.
// Returns an error if context is cancelled or if any source fails to fetch.
func (r *Registry) Resolve(ctx context.Context, packID pack.ID, schema map[variables.ID]*variables.Variable) ([]*variables.Variable, error) {
	// Check context before starting
	if err := ctx.Err(); err != nil {
		return nil, fmt.Errorf("context cancelled before resolution: %w", err)
	}

	// Note: Sources are already sorted by priority in Register()

	// Use a map to merge by variable name (higher priority overwrites)
	varMap := make(map[variables.ID]*variables.Variable)

	for _, source := range r.sources {
		// Check context in loop for long-running operations
		if err := ctx.Err(); err != nil {
			return nil, fmt.Errorf("context cancelled during resolution: %w", err)
		}

		vars, err := source.Fetch(ctx, packID, schema)
		if err != nil {
			return nil, fmt.Errorf("failed to fetch from %s: %w", source.Name(), err)
		}

		// Merge (higher priority overwrites)
		for _, v := range vars {
			varMap[v.Name] = v
		}
	}

	result := slices.Collect(maps.Values(varMap))

	return result, nil
}
