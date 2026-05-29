// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package source

import (
	"context"
	"fmt"
	"sort"

	"github.com/hashicorp/nomad-pack/sdk/pack"
	"github.com/hashicorp/nomad-pack/sdk/pack/variables"
)

// Registry manages multiple variable sources and resolves them with
// priority-based precedence. Higher priority sources override lower
// priority sources for variables with the same name.
//
// Example usage:
//
//	registry := source.NewRegistry()
//	registry.Register(source.NewEnvSource(10, envVars))
//	registry.Register(source.NewFileSource(20, fileVars))
//	registry.Register(source.NewCLISource(30, cliVars))
//	vars, err := registry.Resolve(ctx, packID)
type Registry struct {
	sources []VariableSource
}

// NewRegistry creates a new source registry.
func NewRegistry() *Registry {
	return &Registry{
		sources: make([]VariableSource, 0),
	}
}

// Register adds a source to the registry. Returns an error if the source
// is nil or if a source with the same name is already registered.
func (r *Registry) Register(source VariableSource) error {
	if source == nil {
		return fmt.Errorf("cannot register nil source")
	}

	// Check for duplicate names
	for _, existing := range r.sources {
		if existing.Name() == source.Name() {
			return fmt.Errorf("source with name %q already registered", source.Name())
		}
	}

	r.sources = append(r.sources, source)
	return nil
}

// Resolve fetches and merges variables from all registered sources.
// Sources are processed in priority order (lowest to highest), with
// higher priority sources overwriting variables from lower priority sources.
// Returns an error if context is cancelled or if any source fails to fetch.
func (r *Registry) Resolve(ctx context.Context, packID pack.ID) ([]*variables.Variable, error) {
	// Check context before starting
	if err := ctx.Err(); err != nil {
		return nil, fmt.Errorf("context cancelled before resolution: %w", err)
	}

	// Sort by priority (lower first, so higher priority overwrites)
	sorted := make([]VariableSource, len(r.sources))
	copy(sorted, r.sources)
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].Priority() < sorted[j].Priority()
	})

	// Use a map to merge by variable name (higher priority overwrites)
	varMap := make(map[variables.ID]*variables.Variable)

	for _, source := range sorted {
		// Check context in loop for long-running operations
		if err := ctx.Err(); err != nil {
			return nil, fmt.Errorf("context cancelled during resolution: %w", err)
		}

		vars, err := source.Fetch(ctx, packID)
		if err != nil {
			return nil, fmt.Errorf("failed to fetch from %s: %w", source.Name(), err)
		}

		// Merge (higher priority overwrites)
		for _, v := range vars {
			varMap[v.Name] = v
		}
	}

	// Convert map back to slice
	result := make([]*variables.Variable, 0, len(varMap))
	for _, v := range varMap {
		result = append(result, v)
	}

	return result, nil
}

// Sources returns a copy of all registered sources for inspection.
// This is useful for debugging and testing.
func (r *Registry) Sources() []VariableSource {
	return append([]VariableSource(nil), r.sources...)
}

// Clear removes all registered sources from the registry.
func (r *Registry) Clear() {
	r.sources = r.sources[:0]
}
