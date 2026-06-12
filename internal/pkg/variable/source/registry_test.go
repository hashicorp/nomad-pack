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
	packID := pack.ID("test")

	testCases := []struct {
		name     string
		sources  []VariableSource
		expected cty.Value
	}{
		{
			name: "priority resolution",
			sources: []VariableSource{
				NewBaseSource("low", 1, map[pack.ID][]*variables.Variable{
					packID: {{Name: "var", Value: cty.StringVal("low")}},
				}),
				NewBaseSource("high", 10, map[pack.ID][]*variables.Variable{
					packID: {{Name: "var", Value: cty.StringVal("high")}},
				}),
			},
			expected: cty.StringVal("high"),
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			registry := NewRegistry()
			for _, s := range tc.sources {
				must.NoError(t, registry.Register(s))
			}

			result, err := registry.Resolve(t.Context(), packID)
			must.NoError(t, err)
			must.Len(t, 1, result)
			must.True(t, tc.expected.Equals(result[0].Value).True())
		})
	}

	t.Run("empty registry", func(t *testing.T) {
		registry := NewRegistry()
		result, err := registry.Resolve(t.Context(), pack.ID("test"))
		must.NoError(t, err)
		must.Len(t, 0, result)
	})

	t.Run("context cancellation", func(t *testing.T) {
		registry := NewRegistry()
		s := NewBaseSource("test", 1, map[pack.ID][]*variables.Variable{
			pack.ID("test"): {{Name: "var", Value: cty.StringVal("val")}},
		})
		must.NoError(t, registry.Register(s))

		ctx, cancel := context.WithCancel(t.Context())
		t.Cleanup(cancel)
		cancel()

		_, err := registry.Resolve(ctx, pack.ID("test"))
		must.Error(t, err)
	})
}
