// Copyright IBM Corp. 2023, 2026
// SPDX-License-Identifier: MPL-2.0

package source

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/hashicorp/nomad-pack/sdk/pack"
	"github.com/hashicorp/nomad-pack/sdk/pack/variables"
	"github.com/hashicorp/nomad/api"
	"github.com/zclconf/go-cty/cty"
)

// NomadSource fetches variables from Nomad Variables API.
// Variables are stored under a configurable path with the structure:
// {prefix}/{pack-id}/{variable-name}
type NomadSource struct {
	name      string
	priority  int
	client    *api.Client
	prefix    string
	namespace string
}

// NewNomadSource creates a new Nomad Variables source.
// The config parameter can be nil to use default Nomad configuration
// (which reads from NOMAD_ADDR and NOMAD_TOKEN env vars).
// The prefix parameter specifies the path prefix for pack variables.
// The namespace parameter specifies the Nomad namespace to use.
func NewNomadSource(priority int, config *api.Config, prefix, namespace string) (*NomadSource, error) {
	if config == nil {
		config = api.DefaultConfig()
	}

	client, err := api.NewClient(config)
	if err != nil {
		return nil, fmt.Errorf("failed to create Nomad client: %w", err)
	}

	// Default namespace to "default" if not specified
	if namespace == "" {
		namespace = "default"
	}

	return &NomadSource{
		name:      "nomad",
		priority:  priority,
		client:    client,
		prefix:    prefix,
		namespace: namespace,
	}, nil
}

// Name returns the unique identifier for this source.
func (n *NomadSource) Name() string {
	return n.name
}

// Priority returns the precedence level (higher = higher priority).
func (n *NomadSource) Priority() int {
	return n.priority
}

// Fetch retrieves variables for the given pack from Nomad Variables.
// Variables are expected to be stored as JSON-encoded values that can be
// converted to cty.Value types.
func (n *NomadSource) Fetch(ctx context.Context, packID pack.ID) ([]*variables.Variable, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	// Build the path prefix for this pack
	pathPrefix := n.buildPath(string(packID))

	// List all variables with this prefix using the List API
	opts := &api.QueryOptions{
		Namespace: n.namespace,
		Prefix:    pathPrefix,
	}
	opts = opts.WithContext(ctx)

	varList, _, err := n.client.Variables().List(opts)
	if err != nil {
		return nil, fmt.Errorf("failed to list Nomad variables with prefix %s: %w", pathPrefix, err)
	}

	// If no variables found, return nil (not an error)
	if len(varList) == 0 {
		return nil, nil
	}

	// Fetch each variable and convert to pack variables
	// Pre-allocate with estimated capacity (at least one variable per path)
	packVars := make([]*variables.Variable, 0, len(varList))
	for _, varMeta := range varList {
		// Read the full variable
		readOpts := &api.QueryOptions{
			Namespace: n.namespace,
		}
		readOpts = readOpts.WithContext(ctx)

		variable, _, err := n.client.Variables().Read(varMeta.Path, readOpts)
		if err != nil {
			return nil, fmt.Errorf("failed to read variable %s: %w", varMeta.Path, err)
		}

		// Convert each item in the variable to a pack variable
		for key, value := range variable.Items {
			ctyVal, err := n.convertValue(value)
			if err != nil {
				return nil, fmt.Errorf("failed to convert value for %s: %w", key, err)
			}

			packVars = append(packVars, &variables.Variable{
				Name:  variables.ID(key),
				Value: ctyVal,
				Type:  ctyVal.Type(),
			})
		}
	}

	return packVars, nil
}

// buildPath constructs the full path prefix for a pack's variables.
func (n *NomadSource) buildPath(packID string) string {
	if n.prefix == "" {
		return packID
	}
	return n.prefix + "/" + packID
}

// convertValue converts a value from Nomad Variables to a cty.Value.
// Nomad Variables stores values as strings, so we try to parse as JSON first.
func (n *NomadSource) convertValue(data string) (cty.Value, error) {
	// Try to parse as JSON
	var jsonValue interface{}
	if err := json.Unmarshal([]byte(data), &jsonValue); err == nil {
		// Successfully parsed as JSON
		return convertJSONToCty(jsonValue)
	}

	// Not JSON, treat as plain string
	return cty.StringVal(data), nil
}
