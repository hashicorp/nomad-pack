// Copyright IBM Corp. 2023, 2026
// SPDX-License-Identifier: MPL-2.0

package source

import (
	"context"
	"testing"

	"github.com/hashicorp/nomad-pack/sdk/pack"
	"github.com/hashicorp/nomad-pack/sdk/pack/variables"
	"github.com/shoenig/test/must"
	"github.com/zclconf/go-cty/cty"
)

func TestRegistry_Resolve(t *testing.T) {
	packID := pack.ID("webapp")

	schema := map[variables.ID]*variables.Variable{
		"app_name": {
			Name: "app_name",
			Type: cty.String,
		},
		"replicas": {
			Name: "replicas",
			Type: cty.Number,
		},
	}

	t.Run("cli overrides consul", func(t *testing.T) {
		registry := NewRegistry()

		// Consul has lower priority
		consulVars := map[pack.ID][]*variables.Variable{
			packID: {
				{Name: "app_name", Value: cty.StringVal("consul-app")},
				{Name: "replicas", Value: cty.NumberIntVal(3)},
			},
		}
		registry.Register(NewBaseSource("consul", PriorityConsul, consulVars))

		// CLI has higher priority
		cliVars := map[pack.ID][]*variables.Variable{
			packID: {
				{Name: "app_name", Value: cty.StringVal("cli-app")},
			},
		}
		registry.Register(NewBaseSource("cli", PriorityCLI, cliVars))

		result, err := registry.Resolve(t.Context(), packID, schema)
		must.NoError(t, err)
		must.Len(t, 2, result)

		// CLI value should win for app_name
		var appName, replicas *variables.Variable
		for _, v := range result {
			switch v.Name {
			case "app_name":
				appName = v
			case "replicas":
				replicas = v
			}
		}

		must.Eq(t, "cli-app", appName.Value.AsString())
		replicasInt, _ := replicas.Value.AsBigFloat().Int64()
		must.Eq(t, int64(3), replicasInt)
	})

	t.Run("multiple sources merge correctly", func(t *testing.T) {
		registry := NewRegistry()

		// File source provides base config
		fileVars := map[pack.ID][]*variables.Variable{
			packID: {{Name: "replicas", Value: cty.NumberIntVal(1)}},
		}
		registry.Register(NewBaseSource("file", PriorityFile, fileVars))

		// Consul provides app name
		consulVars := map[pack.ID][]*variables.Variable{
			packID: {{Name: "app_name", Value: cty.StringVal("prod-app")}},
		}
		registry.Register(NewBaseSource("consul", PriorityConsul, consulVars))

		result, err := registry.Resolve(t.Context(), packID, schema)
		must.NoError(t, err)
		must.Len(t, 2, result)
	})

	t.Run("no variables for pack", func(t *testing.T) {
		registry := NewRegistry()
		registry.Register(NewBaseSource("empty", PriorityFile, nil))

		result, err := registry.Resolve(t.Context(), pack.ID("nonexistent"), schema)
		must.NoError(t, err)
		must.Len(t, 0, result)
	})

	t.Run("cancelled context fails fast", func(t *testing.T) {
		registry := NewRegistry()
		vars := map[pack.ID][]*variables.Variable{
			packID: {{Name: "app_name", Value: cty.StringVal("test")}},
		}
		registry.Register(NewBaseSource("test", PriorityFile, vars))

		ctx, cancel := context.WithCancel(t.Context())
		t.Cleanup(cancel)
		cancel() // Cancel immediately

		_, err := registry.Resolve(ctx, packID, schema)
		must.ErrorContains(t, err, "context canceled")
	})
}
