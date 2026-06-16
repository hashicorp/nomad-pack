// Copyright IBM Corp. 2023, 2026
// SPDX-License-Identifier: MPL-2.0

package source

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/hashicorp/consul/api"
	"github.com/hashicorp/nomad-pack/sdk/pack"
	"github.com/hashicorp/nomad-pack/sdk/pack/variables"
	"github.com/zclconf/go-cty/cty"
	"github.com/zclconf/go-cty/cty/convert"
)

// ConsulSource fetches variables from Consul KV store.
// Variables are stored under the configured prefix. When includePackID is set,
// the pack ID is inserted between the prefix and the variable name at fetch
// time (<prefix>/<pack-id>/<var-name>); otherwise the prefix is used
type ConsulSource struct {
	name          string
	priority      int
	client        *api.Client
	prefix        string // KV prefix where variables are stored
	includePackID bool   // whether to append the pack ID to the prefix
}

// NewConsulSource creates a new Consul KV variable source.
// The config parameter can be nil to use default Consul configuration
// (which reads from CONSUL_HTTP_ADDR and CONSUL_HTTP_TOKEN env vars).
// The prefix parameter is the KV prefix where variables are stored. When
// includePackID is true, the pack ID is appended to the prefix at fetch time.
func NewConsulSource(priority int, config *api.Config, prefix string, includePackID bool) (*ConsulSource, error) {
	if config == nil {
		config = api.DefaultConfig()
	}

	client, err := api.NewClient(config)
	if err != nil {
		return nil, fmt.Errorf("failed to create Consul client: %w", err)
	}

	// Make the name unique by including the address and prefix.
	// This allows multiple Consul sources with different configurations
	// and eliminates the possibility of duplicate names.
	trimmedPrefix := strings.Trim(prefix, "/")
	name := fmt.Sprintf("consul(%s:%s)", config.Address, trimmedPrefix)

	return &ConsulSource{
		name:          name,
		priority:      priority,
		client:        client,
		prefix:        trimmedPrefix,
		includePackID: includePackID,
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

// Fetch retrieves variables for the given pack from Consul KV.
// Uses the schema to determine the expected type for each variable,
// performing schema-aware type conversion instead of guessing.
//
// Type Conversion Rules:
//   - If schema expects string: always return as string (even if valid JSON)
//   - If schema expects number: parse as JSON number
//   - If schema expects bool: parse as JSON boolean
//   - If schema expects object/list: parse as JSON and convert
//   - Variables not in schema are skipped (not returned)
//
// Edge Cases:
//   - Returns nil (not error) if no keys found at path
//   - Skips directory entries (keys ending with /)
//   - Skips variables not defined in the pack schema
//
// The parser automatically provides a 30-second timeout context.
func (c *ConsulSource) Fetch(ctx context.Context, packID pack.ID, schema map[variables.ID]*variables.Variable) ([]*variables.Variable, error) {
	// Build the lookup path. When configured, the pack ID is inserted between
	// the prefix and the variable name so that multiple packs can share a
	// single prefix without colliding.
	path := c.prefix
	if c.includePackID {
		path = path + "/" + string(packID)
	}
	path = path + "/"

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
		// Extract variable name from key (remove prefix)
		varName := strings.TrimPrefix(pair.Key, path)

		// Skip if this is a directory (ends with /)
		if strings.HasSuffix(varName, "/") {
			continue
		}

		// Check if this variable exists in the schema
		schemaVar, inSchema := schema[variables.ID(varName)]
		if !inSchema {
			// Skip variables not defined in the pack schema
			continue
		}

		// Convert value using schema-aware conversion
		value, err := c.convertValueWithSchema(pair.Value, schemaVar.Type)
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

// convertValueWithSchema converts a byte slice to a cty.Value using the expected type from the schema.
// This prevents guessing and ensures the value matches what the pack expects.
func (c *ConsulSource) convertValueWithSchema(data []byte, expectedType cty.Type) (cty.Value, error) {
	// If the schema expects a string, always return as string (even if it's valid JSON)
	if expectedType == cty.String {
		return cty.StringVal(string(data)), nil
	}

	// For non-string types, parse as JSON
	var v any
	if err := json.Unmarshal(data, &v); err != nil {
		return cty.NilVal, fmt.Errorf("expected %s but value is not valid JSON: %w", expectedType.FriendlyName(), err)
	}

	// Convert JSON to cty.Value
	ctyVal, err := convertJSONToCty(v)
	if err != nil {
		return cty.NilVal, err
	}

	// Using convert.Convert lets compatible shapes succeed: empty collections, JSON objects into
	// map(...), and homogeneous arrays into list(...)/set(...).
	converted, err := convert.Convert(ctyVal, expectedType)
	if err != nil {
		return cty.NilVal, fmt.Errorf("type mismatch: expected %s: %w", expectedType.FriendlyName(), err)
	}

	return converted, nil
}
