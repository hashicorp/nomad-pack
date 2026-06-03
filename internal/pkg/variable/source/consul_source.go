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
	"github.com/zclconf/go-cty/cty/gocty"
)

// ConsulSource fetches variables from Consul KV store.
// Variables are stored under a configurable prefix with the structure:
// {prefix}/{pack-id}/{variable-name}
type ConsulSource struct {
	name     string
	priority int
	client   *api.Client
	prefix   string
}

// NewConsulSource creates a new Consul KV variable source.
// The config parameter can be nil to use default Consul configuration
// (which reads from CONSUL_HTTP_ADDR and CONSUL_HTTP_TOKEN env vars).
func NewConsulSource(priority int, config *api.Config, prefix string) (*ConsulSource, error) {
	if config == nil {
		config = api.DefaultConfig()
	}

	client, err := api.NewClient(config)
	if err != nil {
		return nil, fmt.Errorf("failed to create Consul client: %w", err)
	}

	if prefix == "" {
		prefix = "nomad-pack/vars"
	}

	return &ConsulSource{
		name:     "consul",
		priority: priority,
		client:   client,
		prefix:   strings.TrimSuffix(prefix, "/"),
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
// Variables are expected to be stored as JSON values that can be
// converted to cty.Value types. If a value is not valid JSON,
// it will be treated as a string.
func (c *ConsulSource) Fetch(ctx context.Context, packID pack.ID) ([]*variables.Variable, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	// Build KV path: prefix/packID/
	path := fmt.Sprintf("%s/%s/", c.prefix, packID)

	// List all keys under this path
	opts := api.QueryOptions{RequireConsistent: true}
	pairs, _, err := c.client.KV().List(path, (&opts).WithContext(ctx))

	if err != nil {
		return nil, fmt.Errorf("failed to list Consul KV at %s: %w", path, err)
	}

	// If no keys found, return empty slice (not an error)
	if len(pairs) == 0 {
		return make([]*variables.Variable, 0), nil
	}

	vars := make([]*variables.Variable, 0, len(pairs))
	for _, pair := range pairs {
		// Extract variable name from key (remove prefix)
		varName := strings.TrimPrefix(pair.Key, path)

		// Skip if this is a directory (ends with /)
		if strings.HasSuffix(varName, "/") {
			continue
		}

		// Convert value to cty.Value
		value, err := c.convertValue(pair.Value)
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

// convertValue converts a byte slice to a cty.Value.
// It first attempts to parse as JSON. If that fails, it treats the value as a string.
func (c *ConsulSource) convertValue(data []byte) (cty.Value, error) {
	// Try to parse as JSON first
	var v interface{}
	if err := json.Unmarshal(data, &v); err != nil {
		// If not valid JSON, treat as string
		return cty.StringVal(string(data)), nil
	}

	// Convert JSON to cty.Value
	return convertJSONToCty(v)
}

// convertJSONToCty converts a Go interface{} (from JSON) to a cty.Value.
func convertJSONToCty(v interface{}) (cty.Value, error) {
	switch val := v.(type) {
	case nil:
		return cty.NullVal(cty.DynamicPseudoType), nil

	case string:
		return cty.StringVal(val), nil

	case float64:
		return cty.NumberFloatVal(val), nil

	case bool:
		return cty.BoolVal(val), nil

	case []interface{}:
		if len(val) == 0 {
			return cty.ListValEmpty(cty.DynamicPseudoType), nil
		}

		// Convert each element
		elements := make([]cty.Value, len(val))
		for i, elem := range val {
			elemVal, err := convertJSONToCty(elem)
			if err != nil {
				return cty.NilVal, fmt.Errorf("failed to convert list element %d: %w", i, err)
			}
			elements[i] = elemVal
		}

		// Try to create a list with a unified type
		return cty.ListVal(elements), nil

	case map[string]interface{}:
		if len(val) == 0 {
			return cty.EmptyObjectVal, nil
		}

		// Convert each value
		attrs := make(map[string]cty.Value)
		for k, v := range val {
			attrVal, err := convertJSONToCty(v)
			if err != nil {
				return cty.NilVal, fmt.Errorf("failed to convert object attribute %s: %w", k, err)
			}
			attrs[k] = attrVal
		}

		return cty.ObjectVal(attrs), nil

	default:
		// Fallback: try to use gocty to convert
		ty, err := gocty.ImpliedType(v)
		if err != nil {
			return cty.NilVal, fmt.Errorf("unsupported type %T: %w", v, err)
		}

		ctyVal, err := gocty.ToCtyValue(v, ty)
		if err != nil {
			return cty.NilVal, fmt.Errorf("failed to convert %T to cty.Value: %w", v, err)
		}

		return ctyVal, nil
	}
}
