// Copyright IBM Corp. 2023, 2026
// SPDX-License-Identifier: MPL-2.0

package source

import (
	"context"
	"testing"

	"github.com/hashicorp/nomad-pack/sdk/pack"
	"github.com/hashicorp/nomad-pack/sdk/pack/variables"
	vault "github.com/hashicorp/vault/api"
	"github.com/shoenig/test/must"
	"github.com/zclconf/go-cty/cty"
)

// skipIfVaultUnavailable checks if Vault is available and skips the test if not.
func skipIfVaultUnavailable(t *testing.T) *vault.Client {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	config := vault.DefaultConfig()
	client, err := vault.NewClient(config)
	if err != nil {
		t.Fatalf("Vault client creation failed: %v", err)
	}

	// Verify Vault is reachable and unsealed
	health, err := client.Sys().Health()
	if err != nil {
		t.Fatalf("Vault not reachable: %v", err)
	}

	if health.Sealed {
		t.Skip("Vault is sealed")
	}

	return client
}

func TestVaultSource_Fetch_Success_KVv2(t *testing.T) {
	client := skipIfVaultUnavailable(t)

	// Create source for KV v2
	source, err := NewVaultSource(30, nil, "secret", "nomad-pack-test/vars")
	must.NoError(t, err)

	packID := pack.ID("test-pack")

	// Setup test data in Vault KV v2
	// Store values as JSON strings so they can be properly parsed
	testData := map[string]string{
		"string_var": `"hello world"`,
		"number_var": `42`,
		"bool_var":   `true`,
		"list_var":   `["a", "b", "c"]`,
		"map_var":    `{"key1": "value1", "key2": "value2"}`,
	}

	for key, jsonValue := range testData {
		path := "secret/data/nomad-pack-test/vars/test-pack/" + key
		_, err := client.Logical().Write(path, map[string]interface{}{
			"data": map[string]interface{}{
				"value": jsonValue,
			},
		})
		must.NoError(t, err)
	}

	// Cleanup
	t.Cleanup(func() {
		for key := range testData {
			path := "secret/metadata/nomad-pack-test/vars/test-pack/" + key
			_, _ = client.Logical().Delete(path)
		}
	})

	// Fetch variables
	vars, err := source.Fetch(context.Background(), packID)
	must.NoError(t, err)
	must.Len(t, 5, vars)

	// Verify each variable
	varMap := make(map[string]*variables.Variable)
	for _, v := range vars {
		varMap[string(v.Name)] = v
	}

	// Check string_var
	must.True(t, varMap["string_var"].Value.Equals(cty.StringVal("hello world")).True())

	// Check number_var
	must.True(t, varMap["number_var"].Value.Equals(cty.NumberIntVal(42)).True())

	// Check bool_var
	must.True(t, varMap["bool_var"].Value.Equals(cty.BoolVal(true)).True())

	// Check list_var
	expectedList := cty.ListVal([]cty.Value{
		cty.StringVal("a"),
		cty.StringVal("b"),
		cty.StringVal("c"),
	})
	must.True(t, varMap["list_var"].Value.Equals(expectedList).True())

	// Check map_var
	expectedMap := cty.ObjectVal(map[string]cty.Value{
		"key1": cty.StringVal("value1"),
		"key2": cty.StringVal("value2"),
	})
	must.True(t, varMap["map_var"].Value.Equals(expectedMap).True())
}

func TestVaultSource_Fetch_ContextCancellation(t *testing.T) {
	skipIfVaultUnavailable(t)

	source, err := NewVaultSource(30, nil, "secret", "nomad-pack-test/vars")
	must.NoError(t, err)

	// Create cancelled context
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	// Fetch should fail with context error
	_, err = source.Fetch(ctx, pack.ID("test-pack"))
	must.Error(t, err)
}

func TestVaultSource_Fetch_StringValue(t *testing.T) {
	client := skipIfVaultUnavailable(t)

	source, err := NewVaultSource(30, nil, "secret", "nomad-pack-test/vars")
	must.NoError(t, err)

	packID := pack.ID("test-pack-string")

	// Put string value in Vault KV v2
	path := "secret/data/nomad-pack-test/vars/test-pack-string/plain_text"
	_, err = client.Logical().Write(path, map[string]interface{}{
		"data": map[string]interface{}{
			"value": "this is plain text",
		},
	})
	must.NoError(t, err)

	// Cleanup
	t.Cleanup(func() {
		path := "secret/metadata/nomad-pack-test/vars/test-pack-string/plain_text"
		_, _ = client.Logical().Delete(path)
	})

	// Fetch should succeed and treat as string
	vars, err := source.Fetch(context.Background(), packID)
	must.NoError(t, err)
	must.Len(t, 1, vars)
	must.True(t, vars[0].Value.Equals(cty.StringVal("this is plain text")).True())
}

func TestVaultSource_WithRegistry(t *testing.T) {
	client := skipIfVaultUnavailable(t)

	packID := pack.ID("test-pack-registry")

	// Setup test data in Vault
	path := "secret/data/nomad-pack-test/vars/test-pack-registry/vault_var"
	_, err := client.Logical().Write(path, map[string]interface{}{
		"data": map[string]interface{}{
			"value": "from-vault",
		},
	})
	must.NoError(t, err)

	// Cleanup
	t.Cleanup(func() {
		path := "secret/metadata/nomad-pack-test/vars/test-pack-registry/vault_var"
		_, _ = client.Logical().Delete(path)
	})

	// Create registry with Vault source
	registry := NewRegistry()

	vaultSource, err := NewVaultSource(30, nil, "secret", "nomad-pack-test/vars")
	must.NoError(t, err)

	err = registry.Register(vaultSource)
	must.NoError(t, err)

	// Resolve variables
	vars, err := registry.Resolve(context.Background(), packID)
	must.NoError(t, err)
	must.Len(t, 1, vars)
	must.Eq(t, "vault_var", string(vars[0].Name))
	must.True(t, vars[0].Value.Equals(cty.StringVal("from-vault")).True())
}
