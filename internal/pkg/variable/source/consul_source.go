// Copyright IBM Corp. 2023, 2026
// SPDX-License-Identifier: MPL-2.0

package source

import (
	"context"
	"fmt"
	"strings"

	"github.com/hashicorp/consul/api"
	"github.com/hashicorp/nomad-pack/sdk/pack"
	"github.com/hashicorp/nomad-pack/sdk/pack/variables"
	"github.com/zclconf/go-cty/cty"
)

// ConsulSource fetches variables from Consul KV store. Each variable is read
// from <path>/<variable-name>, where <path> is the user-supplied KV path.
// Callers that want per-pack namespacing include it in the path
// themselves (for example, consul:///myapp/config).
type ConsulSource struct {
	name     string
	priority int
	client   *api.Client
	path     string // KV path under which variables are stored
}

// NewConsulSource creates a new Consul KV variable source.
// The config parameter can be nil to use default Consul configuration
// (which reads from CONSUL_HTTP_ADDR and CONSUL_HTTP_TOKEN env vars).
// The path parameter is the KV path where variables are stored; each variable
// is read from <path>/<variable-name>. It must not be empty.
func NewConsulSource(priority int, config *api.Config, path string) (*ConsulSource, error) {
	// Variables are read from <path>/<variable-name>, so an empty path would
	// list the entire KV store. Require an explicit path.
	trimmedPath := strings.Trim(path, "/")
	if trimmedPath == "" {
		return nil, fmt.Errorf("consul source requires a non-empty KV path")
	}

	if config == nil {
		config = api.DefaultConfig()
	}

	client, err := api.NewClient(config)
	if err != nil {
		return nil, fmt.Errorf("failed to create Consul client: %w", err)
	}

	// Make the name unique by including the address and path.
	// This allows multiple Consul sources with different configurations
	// and eliminates the possibility of duplicate names.
	name := fmt.Sprintf("consul(%s:%s)", config.Address, trimmedPath)

	return &ConsulSource{
		name:     name,
		priority: priority,
		client:   client,
		path:     trimmedPath,
	}, nil
}

// Name returns the unique identifier for this source.
func (c *ConsulSource) Name() string {
	return c.name
}

// Priority returns the precedence level (higher = higher priority).
func (c *ConsulSource) Priority() int {
	return c.priority
}

// Fetch retrieves variables for the given pack from Consul KV. Each variable is
// read from <path>/<variable-name> and decoded into the type the pack schema
// declares for it.
//
// Type Conversion Rules:
//   - If schema expects string: always return as string (even if valid JSON)
//   - If schema expects number: decode the value as a JSON number
//   - If schema expects bool: decode the value as a JSON boolean
//   - If schema expects object/list: decode the value as JSON into that type
//   - Variables not in schema are skipped (not returned)
//
// Edge Cases:
//   - Returns nil (not error) if no keys found at path
//   - Skips directory entries (keys ending with /)
//   - Skips variables not defined in the pack schema
//   - Returns an error for an empty value on a non-string variable; empty
//     values for string variables are kept as ""
//
// The parser wraps Fetch in a timeout context, so a slow or unreachable Consul
// fails the resolve instead of hanging.
func (c *ConsulSource) Fetch(ctx context.Context, _ pack.ID, schema map[variables.ID]*variables.Variable) ([]*variables.Variable, error) {
	// c.path was trimmed of slashes when the source was built; re-add a single
	// trailing slash to scope the KV list to keys under this path and to strip
	// each key down to its variable name. The pack ID is intentionally unused —
	// any per-pack grouping lives in the path itself.
	path := c.path + "/"

	// List all keys under this path
	opts := api.QueryOptions{RequireConsistent: true}
	pairs, _, err := c.client.KV().List(path, (&opts).WithContext(ctx))

	if err != nil {
		return nil, fmt.Errorf("failed to list Consul KV at %s: %w", path, err)
	}

	// If no keys found, return nil (not an error)
	if len(pairs) == 0 {
		return nil, nil
	}

	vars := make([]*variables.Variable, 0, len(pairs))
	for _, pair := range pairs {
		// Skip directory entries (keys ending in /) before stripping the prefix
		if strings.HasSuffix(pair.Key, "/") {
			continue
		}

		varName := strings.TrimPrefix(pair.Key, path)

		// Check if this variable exists in the schema
		schemaVar, inSchema := schema[variables.ID(varName)]
		if !inSchema {
			// Skip variables not defined in the pack schema
			continue
		}

		// A non-string variable has no meaningful empty form (there is no "empty"
		// number or bool), so an empty value almost always means the Consul key
		// was misconfigured. Empty values for string variables are valid and kept as "".
		if len(pair.Value) == 0 && schemaVar.Type != cty.String {
			return nil, fmt.Errorf("empty Consul value for %s at %s: a %s value is required", varName, pair.Key, schemaVar.Type.FriendlyName())
		}

		// Convert using the variable's constraint type. ConstraintType preserves
		// optional() attributes.
		expectedType := schemaVar.ConstraintType
		if expectedType == cty.NilType {
			expectedType = schemaVar.Type
		}

		// Convert value using schema-aware conversion
		value, err := decodeValue("Consul", pair.Value, expectedType)
		if err != nil {
			return nil, fmt.Errorf("failed to convert value for %s: %w", varName, err)
		}

		vars = append(vars, &variables.Variable{
			Name:  variables.ID(varName),
			Value: value,
			Type:  value.Type(),
		})
	}

	return vars, nil
}
