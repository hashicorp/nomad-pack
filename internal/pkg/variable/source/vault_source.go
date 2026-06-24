// Copyright IBM Corp. 2023, 2026
// SPDX-License-Identifier: MPL-2.0

package source

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/hashicorp/nomad-pack/sdk/pack"
	"github.com/hashicorp/nomad-pack/sdk/pack/variables"
	vaultapi "github.com/hashicorp/vault/api"
	"github.com/zclconf/go-cty/cty"
	ctyjson "github.com/zclconf/go-cty/cty/json"
)

// VaultSource fetches variables from a Vault KV v2 secret. All variables for a
// pack are stored as fields of a single secret at <mount>/<path>. Callers
// include any per-pack namespacing in the path themselves
// (for example, mount="secret", path="myapp/config").
type VaultSource struct {
	name     string
	priority int
	client   *vaultapi.Client
	mount    string // KV v2 mount point, e.g. "secret"
	path     string // secret path within the mount, e.g. "myapp/config"
}

// NewVaultSource creates a new Vault KV v2 variable source.
// config can be nil to use the default Vault configuration
// (reads VAULT_ADDR and VAULT_TOKEN from the environment).
// mount is the KV v2 engine mount point; path is the secret path within it.
func NewVaultSource(priority int, config *vaultapi.Config, mount, path string) (*VaultSource, error) {
	mount = strings.Trim(mount, "/")
	if mount == "" {
		return nil, fmt.Errorf("vault source requires a non-empty mount point")
	}

	path = strings.Trim(path, "/")
	if path == "" {
		return nil, fmt.Errorf("vault source requires a non-empty secret path")
	}

	if config == nil {
		config = vaultapi.DefaultConfig()
	}

	client, err := vaultapi.NewClient(config)
	if err != nil {
		return nil, fmt.Errorf("failed to create Vault client: %w", err)
	}

	name := fmt.Sprintf("vault(%s/%s/%s)", config.Address, mount, path)

	return &VaultSource{
		name:     name,
		priority: priority,
		client:   client,
		mount:    mount,
		path:     path,
	}, nil
}

// Name returns the unique identifier for this source.
func (v *VaultSource) Name() string {
	return v.name
}

// Priority returns the precedence level (higher = higher priority).
func (v *VaultSource) Priority() int {
	return v.priority
}

// Fetch reads the Vault KV v2 secret at <mount>/<path> and returns variables
// whose names appear in schema. Values are decoded using the schema type —
// string fields are returned as-is; all other types are JSON-decoded.
//
// Each field of the secret maps to one pack variable. Fields not present in
// the schema are silently skipped. An empty value for a non-string variable
// is an error; empty strings are valid for string variables.
//
// Returns nil (not an error) when the secret does not exist at the given path
// or when the latest version has been deleted.
func (v *VaultSource) Fetch(ctx context.Context, _ pack.ID, schema map[variables.ID]*variables.Variable) ([]*variables.Variable, error) {
	secret, err := v.client.KVv2(v.mount).Get(ctx, v.path)
	if err != nil {
		if errors.Is(err, vaultapi.ErrSecretNotFound) {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to read Vault secret at %s/%s: %w", v.mount, v.path, err)
	}

	// A deleted secret has nil Data.
	if secret.Data == nil {
		return nil, nil
	}

	vars := make([]*variables.Variable, 0, len(secret.Data))
	for rawKey, rawVal := range secret.Data {
		schemaVar, inSchema := schema[variables.ID(rawKey)]
		if !inSchema {
			continue
		}

		// Vault KV stores all values as strings.
		str, ok := rawVal.(string)
		if !ok {
			return nil, fmt.Errorf("field %s is not a string (got %T)", rawKey, rawVal)
		}

		// Empty values for string variables are valid and kept as "".
		if str == "" && schemaVar.Type != cty.String {
			return nil, fmt.Errorf("empty Vault value for %s: a %s value is required", rawKey, schemaVar.Type.FriendlyName())
		}

		// Convert using the variable's constraint type. ConstraintType preserves
		// optional() attributes.
		expectedType := schemaVar.ConstraintType
		if expectedType == cty.NilType {
			expectedType = schemaVar.Type
		}

		value, err := v.convertValue([]byte(str), expectedType)
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

// convertValue converts a raw Vault string value into a cty.Value of the
// expected type.
func (v *VaultSource) convertValue(data []byte, expectedType cty.Type) (cty.Value, error) {
	if expectedType == cty.String {
		return cty.StringVal(string(data)), nil
	}

	// For every other type, let cty decode the JSON directly into the expected type.
	val, err := ctyjson.Unmarshal(data, expectedType)
	if err != nil {
		return cty.NilVal, fmt.Errorf("decoding Vault value as %s: %w", expectedType.FriendlyName(), err)
	}

	return val, nil
}
