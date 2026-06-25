// Copyright IBM Corp. 2023, 2026
// SPDX-License-Identifier: MPL-2.0

package source

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/hashicorp/nomad/api"

	"github.com/hashicorp/nomad-pack/sdk/pack"
	"github.com/hashicorp/nomad-pack/sdk/pack/variables"
	"github.com/zclconf/go-cty/cty"
	ctyjson "github.com/zclconf/go-cty/cty/json"
)

// NomadSource fetches variables from a Nomad Variable. All variables for a pack
// are stored as items of a single Nomad Variable at <path>.
type NomadSource struct {
	name     string
	priority int
	client   *api.Client
	path     string // Nomad Variable path, e.g. "nomad-pack/myapp"
}

// NewNomadSource creates a new Nomad Variables source. config can be nil to use
// the default Nomad configuration (reads NOMAD_ADDR, NOMAD_TOKEN, and
// NOMAD_NAMESPACE from the environment). path is the Nomad Variable path whose
// items become pack variables.
func NewNomadSource(priority int, config *api.Config, path string) (*NomadSource, error) {
	path = strings.Trim(path, "/")
	if path == "" {
		return nil, fmt.Errorf("nomad source requires a non-empty variable path")
	}

	if config == nil {
		config = api.DefaultConfig()
	}

	client, err := api.NewClient(config)
	if err != nil {
		return nil, fmt.Errorf("failed to create Nomad client: %w", err)
	}

	name := fmt.Sprintf("nomad(%s/%s)", config.Address, path)

	return &NomadSource{
		name:     name,
		priority: priority,
		client:   client,
		path:     path,
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

// Fetch reads the Nomad Variable at <path> and returns variables whose names
// appear in schema. Values are decoded using the schema type — string items are
// returned as-is; all other types are JSON-decoded.
//
// Each item of the Nomad Variable maps to one pack variable.
// Returns nil (not an error) when no Nomad Variable exists at the given path.
func (n *NomadSource) Fetch(ctx context.Context, _ pack.ID, schema map[variables.ID]*variables.Variable) ([]*variables.Variable, error) {
	variable, _, err := n.client.Variables().Read(n.path, (&api.QueryOptions{}).WithContext(ctx))
	if err != nil {
		if errors.Is(err, api.ErrVariablePathNotFound) {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to read Nomad variable at %s: %w", n.path, err)
	}

	vars := make([]*variables.Variable, 0, len(variable.Items))
	for rawKey, str := range variable.Items {
		schemaVar, inSchema := schema[variables.ID(rawKey)]
		if !inSchema {
			continue
		}

		// Empty values for string variables are valid and kept as "".
		if str == "" && schemaVar.Type != cty.String {
			return nil, fmt.Errorf("empty Nomad value for %s: a %s value is required", rawKey, schemaVar.Type.FriendlyName())
		}

		// Convert using the variable's constraint type. ConstraintType preserves
		// optional() attributes.
		expectedType := schemaVar.ConstraintType
		if expectedType == cty.NilType {
			expectedType = schemaVar.Type
		}

		value, err := n.convertValue([]byte(str), expectedType)
		if err != nil {
			return nil, fmt.Errorf("failed to convert value for %s: %w", rawKey, err)
		}

		vars = append(vars, &variables.Variable{
			Name:  variables.ID(rawKey),
			Value: value,
			Type:  value.Type(),
		})
	}

	return vars, nil
}

// convertValue converts a raw Nomad Variable item value into a cty.Value of the
// expected type.
func (n *NomadSource) convertValue(data []byte, expectedType cty.Type) (cty.Value, error) {
	if expectedType == cty.String {
		return cty.StringVal(string(data)), nil
	}

	val, err := ctyjson.Unmarshal(data, expectedType)
	if err != nil {
		return cty.NilVal, fmt.Errorf("decoding Nomad value as %s: %w", expectedType.FriendlyName(), err)
	}

	return val, nil
}
