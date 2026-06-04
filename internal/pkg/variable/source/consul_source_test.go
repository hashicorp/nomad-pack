// Copyright IBM Corp. 2023, 2026
// SPDX-License-Identifier: MPL-2.0

package source

import (
	"context"
	"testing"

	"github.com/hashicorp/consul/api"
	"github.com/hashicorp/nomad-pack/sdk/pack"
	"github.com/hashicorp/nomad-pack/sdk/pack/variables"
	"github.com/shoenig/test/must"
	"github.com/zclconf/go-cty/cty"
)

// skipIfConsulUnavailable checks if Consul is available and skips the test if not.
func skipIfConsulUnavailable(t *testing.T) *api.Client {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	config := api.DefaultConfig()
	client, err := api.NewClient(config)
	if err != nil {
		t.Skipf("Consul client creation failed: %v", err)
	}

	// Verify Consul is reachable
	_, err = client.Status().Leader()
	if err != nil {
		t.Skipf("Consul not reachable: %v", err)
	}

	return client
}

func TestConsulSource_Fetch_Success(t *testing.T) {
	client := skipIfConsulUnavailable(t)

	// Create source
	source, err := NewConsulSource(40, nil, "nomad-pack-test/vars")
	must.NoError(t, err)

	packID := pack.ID("test-pack")
	kv := client.KV()

	// Setup test data
	testData := map[string]string{
		"nomad-pack-test/vars/test-pack/string_var": `"hello world"`,
		"nomad-pack-test/vars/test-pack/number_var": `42`,
		"nomad-pack-test/vars/test-pack/bool_var":   `true`,
		"nomad-pack-test/vars/test-pack/list_var":   `["a", "b", "c"]`,
		"nomad-pack-test/vars/test-pack/map_var":    `{"key1": "value1", "key2": "value2"}`,
	}

	for key, value := range testData {
		_, err := kv.Put(&api.KVPair{
			Key:   key,
			Value: []byte(value),
		}, nil)
		must.NoError(t, err)
	}

	// Cleanup
	defer func() {
		_, err := kv.DeleteTree("nomad-pack-test/vars/test-pack/", nil)
		must.NoError(t, err)
	}()

	// Fetch variables
	vars, err := source.Fetch(t.Context(), packID)
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

func TestConsulSource_Fetch_ContextCancellation(t *testing.T) {
	skipIfConsulUnavailable(t)

	source, err := NewConsulSource(40, nil, "nomad-pack-test/vars")
	must.NoError(t, err)

	// Create cancelled context
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	// Fetch should fail with context error
	_, err = source.Fetch(ctx, pack.ID("test-pack"))
	must.Error(t, err)
}

func TestConsulSource_Fetch_NonJSONValue(t *testing.T) {
	client := skipIfConsulUnavailable(t)

	source, err := NewConsulSource(40, nil, "nomad-pack-test/vars")
	must.NoError(t, err)

	packID := pack.ID("test-pack-plain")
	kv := client.KV()

	// Put non-JSON value (plain string)
	_, err = kv.Put(&api.KVPair{
		Key:   "nomad-pack-test/vars/test-pack-plain/plain_text",
		Value: []byte("this is not json"),
	}, nil)
	must.NoError(t, err)

	// Cleanup
	defer func() {
		_, err := kv.DeleteTree("nomad-pack-test/vars/test-pack-plain/", nil)
		must.NoError(t, err)
	}()

	// Fetch should succeed and treat as string
	vars, err := source.Fetch(t.Context(), packID)
	must.NoError(t, err)
	must.Len(t, 1, vars)
	must.True(t, vars[0].Value.Equals(cty.StringVal("this is not json")).True())
}

func TestConsulSource_WithRegistry(t *testing.T) {
	client := skipIfConsulUnavailable(t)

	packID := pack.ID("test-pack-registry")
	kv := client.KV()

	// Setup test data in Consul
	_, err := kv.Put(&api.KVPair{
		Key:   "nomad-pack-test/vars/test-pack-registry/consul_var",
		Value: []byte(`"from-consul"`),
	}, nil)
	must.NoError(t, err)

	// Cleanup
	defer func() {
		_, err := kv.DeleteTree("nomad-pack-test/vars/test-pack-registry/", nil)
		must.NoError(t, err)
	}()

	// Create registry with Consul source
	registry := NewRegistry()

	consulSource, err := NewConsulSource(40, nil, "nomad-pack-test/vars")
	must.NoError(t, err)

	err = registry.Register(consulSource)
	must.NoError(t, err)

	// Resolve variables
	vars, err := registry.Resolve(t.Context(), packID)
	must.NoError(t, err)
	must.Len(t, 1, vars)
	must.Eq(t, "consul_var", string(vars[0].Name))
	must.True(t, vars[0].Value.Equals(cty.StringVal("from-consul")).True())
}
