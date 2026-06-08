// Copyright IBM Corp. 2023, 2026
// SPDX-License-Identifier: MPL-2.0

package source

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/hashicorp/nomad-pack/sdk/pack"
	"github.com/hashicorp/nomad-pack/sdk/pack/variables"
	vault "github.com/hashicorp/vault/api"
	"github.com/zclconf/go-cty/cty"
)

// VaultSource fetches variables from Vault KV store.
// Supports both KV v1 and KV v2 secret engines.
// Variables are stored under a configurable path with the structure:
// {mount}/{prefix}/{pack-id}/{variable-name}
type VaultSource struct {
	name     string
	priority int
	client   *vault.Client
	mount    string
	prefix   string
}

// NewVaultSource creates a new Vault KV variable source.
// The config parameter can be nil to use default Vault configuration
// (which reads from VAULT_ADDR and VAULT_TOKEN env vars).
// The mount parameter specifies the KV mount point (e.g., "secret").
// The prefix parameter specifies the path prefix within the mount.
func NewVaultSource(priority int, config *vault.Config, mount, prefix string) (*VaultSource, error) {
	if config == nil {
		config = vault.DefaultConfig()
	}

	client, err := vault.NewClient(config)
	if err != nil {
		return nil, fmt.Errorf("failed to create Vault client: %w", err)
	}

	return &VaultSource{
		name:     "vault",
		priority: priority,
		client:   client,
		mount:    strings.Trim(mount, "/"),
		prefix:   strings.Trim(prefix, "/"),
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

// Fetch retrieves variables for the given pack from Vault KV.
// Automatically detects and handles both KV v1 and KV v2 engines.
// Variables are expected to be stored as JSON values that can be
// converted to cty.Value types. If a value is not valid JSON,
// it will be treated as a string.
func (v *VaultSource) Fetch(ctx context.Context, packID pack.ID) ([]*variables.Variable, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	// Build the base path
	basePath := v.buildPath(string(packID))

	// Try to list secrets at this path
	secrets, err := v.listSecrets(ctx, basePath)
	if err != nil {
		return nil, fmt.Errorf("failed to list Vault secrets at %s: %w", basePath, err)
	}

	// If no secrets found, return nil (not an error)
	if len(secrets) == 0 {
		return nil, nil
	}

	// Fetch each secret and convert to variables
	vars := make([]*variables.Variable, 0, len(secrets))
	for _, secretName := range secrets {
		secretPath := basePath + "/" + secretName

		// Read the secret
		data, err := v.readSecret(ctx, secretPath)
		if err != nil {
			return nil, fmt.Errorf("failed to read secret %s: %w", secretPath, err)
		}

		// Convert to cty.Value
		value, err := v.convertValue(data)
		if err != nil {
			return nil, fmt.Errorf("failed to convert value for %s: %w", secretName, err)
		}

		vars = append(vars, &variables.Variable{
			Name:  variables.ID(secretName),
			Value: value,
			Type:  value.Type(),
		})
	}

	return vars, nil
}

// buildPath constructs the full path for a pack's variables.
func (v *VaultSource) buildPath(packID string) string {
	if v.prefix == "" {
		return packID
	}
	return v.prefix + "/" + packID
}

// listSecrets lists all secret names at the given path.
// Handles both KV v1 and KV v2 automatically.
func (v *VaultSource) listSecrets(ctx context.Context, path string) ([]string, error) {
	// Try KV v2 first (most common)
	listPath := v.mount + "/metadata/" + path
	secret, err := v.client.Logical().ListWithContext(ctx, listPath)

	if err == nil && secret != nil && secret.Data != nil {
		// KV v2 successful
		return v.extractKeys(secret.Data)
	}

	// Try KV v1
	listPath = v.mount + "/" + path
	secret, err = v.client.Logical().ListWithContext(ctx, listPath)

	if err != nil {
		return nil, err
	}

	if secret == nil || secret.Data == nil {
		return nil, nil
	}

	return v.extractKeys(secret.Data)
}

// extractKeys extracts the list of keys from Vault's list response.
func (v *VaultSource) extractKeys(data map[string]interface{}) ([]string, error) {
	keysRaw, ok := data["keys"]
	if !ok {
		return nil, nil
	}

	keysSlice, ok := keysRaw.([]interface{})
	if !ok {
		return nil, fmt.Errorf("unexpected keys format: %T", keysRaw)
	}

	keys := make([]string, 0, len(keysSlice))
	for _, k := range keysSlice {
		keyStr, ok := k.(string)
		if !ok {
			continue
		}
		// Skip directories (ending with /)
		if !strings.HasSuffix(keyStr, "/") {
			keys = append(keys, keyStr)
		}
	}

	return keys, nil
}

// readSecret reads a secret from Vault.
// Handles both KV v1 and KV v2 automatically.
func (v *VaultSource) readSecret(ctx context.Context, path string) (interface{}, error) {
	// Try KV v2 first
	readPath := v.mount + "/data/" + path
	secret, err := v.client.Logical().ReadWithContext(ctx, readPath)

	if err == nil && secret != nil && secret.Data != nil {
		// KV v2 - data is nested under "data" key
		if data, ok := secret.Data["data"]; ok {
			if dataMap, ok := data.(map[string]interface{}); ok {
				// If there's a "value" key, use that; otherwise use the whole map
				if value, ok := dataMap["value"]; ok {
					return value, nil
				}
				return dataMap, nil
			}
		}
	}

	// Try KV v1
	readPath = v.mount + "/" + path
	secret, err = v.client.Logical().ReadWithContext(ctx, readPath)

	if err != nil {
		return nil, err
	}

	if secret == nil || secret.Data == nil {
		return nil, fmt.Errorf("secret not found")
	}

	// KV v1 - check for "value" key first, then return whole data
	if value, ok := secret.Data["value"]; ok {
		return value, nil
	}

	return secret.Data, nil
}

// convertValue converts a value from Vault to a cty.Value.
// Handles strings, numbers, booleans, maps, and lists.
func (v *VaultSource) convertValue(data interface{}) (cty.Value, error) {
	// If it's a string, try to parse as JSON first
	if str, ok := data.(string); ok {
		var jsonValue interface{}
		if err := json.Unmarshal([]byte(str), &jsonValue); err == nil {
			// Successfully parsed as JSON
			return convertJSONToCty(jsonValue)
		}
		// Not JSON, treat as plain string
		return cty.StringVal(str), nil
	}

	// For other types, convert directly
	return convertJSONToCty(data)
}
