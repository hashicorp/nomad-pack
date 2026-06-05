// Copyright IBM Corp. 2023, 2026
// SPDX-License-Identifier: MPL-2.0

package source

import (
	"context"
	"testing"

	"github.com/hashicorp/nomad-pack/sdk/pack"
	"github.com/hashicorp/nomad-pack/sdk/pack/variables"
	"github.com/hashicorp/nomad/api"
	"github.com/hashicorp/nomad/command/agent"
	"github.com/hashicorp/nomad/testutil"
	"github.com/shoenig/test/must"
	"github.com/zclconf/go-cty/cty"
)

// skipIfNomadUnavailable checks if a Nomad test server can be started
func skipIfNomadUnavailable(t *testing.T) *api.Client {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	// Start a test Nomad server
	srv := agent.NewTestAgent(t, t.Name(), nil)
	t.Cleanup(func() { srv.Shutdown() })

	// Wait for leader election
	testutil.WaitForLeader(t, srv.RPC)
	testutil.WaitForKeyring(t, srv.RPC, srv.Config.Region)

	// Get client from test server
	client := srv.APIClient()

	return client
}

// TestNomadSource_Fetch_Success tests fetching variables from Nomad
func TestNomadSource_Fetch_Success(t *testing.T) {
	client := skipIfNomadUnavailable(t)

	// Create source (pass nil to use client's config)
	source, err := NewNomadSource(20, nil, "nomad-pack", "default")
	must.NoError(t, err)

	// Override the client with our test client
	source.client = client

	packID := pack.ID("test-pack")

	// Setup test data in Nomad Variables
	testVars := []*api.Variable{
		{
			Namespace: "default",
			Path:      "nomad-pack/test-pack/config",
			Items: map[string]string{
				"string_var": `"hello world"`,
				"number_var": `42`,
				"bool_var":   `true`,
			},
		},
		{
			Namespace: "default",
			Path:      "nomad-pack/test-pack/secrets",
			Items: map[string]string{
				"list_var": `["a", "b", "c"]`,
				"map_var":  `{"key1": "value1", "key2": "value2"}`,
			},
		},
	}

	for _, v := range testVars {
		_, _, err := client.Variables().Create(v, nil)
		must.NoError(t, err)
	}

	// Cleanup
	defer func() {
		for _, v := range testVars {
			_, _ = client.Variables().Delete(v.Path, nil)
		}
	}()

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

// TestNomadSource_Fetch_ContextCancellation tests context cancellation
func TestNomadSource_Fetch_ContextCancellation(t *testing.T) {
	client := skipIfNomadUnavailable(t)

	source, err := NewNomadSource(20, nil, "nomad-pack", "default")
	must.NoError(t, err)
	source.client = client

	// Create a cancelled context
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	// Fetch should return context error
	_, err = source.Fetch(ctx, pack.ID("test-pack"))
	must.Error(t, err)
}

// TestNomadSource_Fetch_StringValue tests plain string values
func TestNomadSource_Fetch_StringValue(t *testing.T) {
	client := skipIfNomadUnavailable(t)

	source, err := NewNomadSource(20, nil, "nomad-pack", "default")
	must.NoError(t, err)
	source.client = client

	packID := pack.ID("test-pack-string")

	// Create a variable with plain string value (not JSON)
	testVar := &api.Variable{
		Namespace: "default",
		Path:      "nomad-pack/test-pack-string/config",
		Items: map[string]string{
			"plain_string": "just a plain string",
		},
	}

	_, _, err = client.Variables().Create(testVar, nil)
	must.NoError(t, err)

	defer func() {
		_, _ = client.Variables().Delete(testVar.Path, nil)
	}()

	// Fetch variables
	vars, err := source.Fetch(context.Background(), packID)
	must.NoError(t, err)
	must.Len(t, 1, vars)

	// Should be treated as string
	must.Eq(t, "plain_string", string(vars[0].Name))
	must.True(t, vars[0].Value.Equals(cty.StringVal("just a plain string")).True())
}

// TestNomadSource_WithRegistry tests integration with Registry
func TestNomadSource_WithRegistry(t *testing.T) {
	client := skipIfNomadUnavailable(t)

	// Create registry and add Nomad source
	registry := NewRegistry()

	nomadSource, err := NewNomadSource(20, nil, "nomad-pack", "default")
	must.NoError(t, err)
	nomadSource.client = client

	err = registry.Register(nomadSource)
	must.NoError(t, err)

	// Verify source is registered
	must.Eq(t, "nomad", nomadSource.Name())
	must.Eq(t, 20, nomadSource.Priority())

	// Verify we can resolve from the registered source
	ctx := context.Background()
	vars, err := registry.Resolve(ctx, pack.ID("nonexistent"))
	must.NoError(t, err)
	must.Len(t, 0, vars) // No variables for nonexistent pack
}
