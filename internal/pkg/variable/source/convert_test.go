// Copyright IBM Corp. 2023, 2026
// SPDX-License-Identifier: MPL-2.0

package source

import (
	"testing"

	"github.com/hashicorp/nomad/ci"
	"github.com/shoenig/test/must"
	"github.com/zclconf/go-cty/cty"
)

func TestConvertJSONToCty(t *testing.T) {
	ci.Parallel(t)

	t.Run("strings", func(t *testing.T) {
		ci.Parallel(t)
		val, err := convertJSONToCty("webapp")
		must.NoError(t, err)
		must.Eq(t, cty.String, val.Type())
		must.Eq(t, "webapp", val.AsString())
	})

	t.Run("numbers", func(t *testing.T) {
		ci.Parallel(t)
		val, err := convertJSONToCty(float64(42))
		must.NoError(t, err)
		must.Eq(t, cty.Number, val.Type())
		f, _ := val.AsBigFloat().Float64()
		must.Eq(t, 42.0, f)
	})

	t.Run("booleans", func(t *testing.T) {
		ci.Parallel(t)

		trueVal, err := convertJSONToCty(true)
		must.NoError(t, err)
		must.True(t, trueVal.True())

		falseVal, err := convertJSONToCty(false)
		must.NoError(t, err)
		must.True(t, falseVal.False())
	})

	t.Run("null becomes dynamic", func(t *testing.T) {
		ci.Parallel(t)
		val, err := convertJSONToCty(nil)
		must.NoError(t, err)
		must.Eq(t, cty.DynamicPseudoType, val.Type())
	})

	t.Run("port list from consul", func(t *testing.T) {
		ci.Parallel(t)
		// Simulates JSON array from Consul: [8080, 8443, 9090].
		ports := []any{float64(8080), float64(8443), float64(9090)}
		val, err := convertJSONToCty(ports)
		must.NoError(t, err)
		must.Eq(t, cty.Tuple([]cty.Type{cty.Number, cty.Number, cty.Number}), val.Type())
		must.Eq(t, 3, val.LengthInt())
	})

	t.Run("datacenter list", func(t *testing.T) {
		ci.Parallel(t)
		dcs := []any{"dc1", "dc2", "dc3"}
		val, err := convertJSONToCty(dcs)
		must.NoError(t, err)
		must.Eq(t, cty.Tuple([]cty.Type{cty.String, cty.String, cty.String}), val.Type())
		must.Eq(t, 3, val.LengthInt())
	})

	t.Run("heterogeneous array does not panic", func(t *testing.T) {
		ci.Parallel(t)
		// A mixed-type JSON array would panic with cty.ListVal; tuples handle it.
		mixed := []any{float64(1), "two", true}
		val, err := convertJSONToCty(mixed)
		must.NoError(t, err)
		must.Eq(t, cty.Tuple([]cty.Type{cty.Number, cty.String, cty.Bool}), val.Type())
		must.Eq(t, 3, val.LengthInt())
	})

	t.Run("empty list defaults to dynamic", func(t *testing.T) {
		ci.Parallel(t)
		val, err := convertJSONToCty([]any{})
		must.NoError(t, err)
		must.Eq(t, cty.EmptyTuple, val.Type())
	})

	t.Run("service config object", func(t *testing.T) {
		ci.Parallel(t)
		// Typical service configuration from Consul
		config := map[string]any{
			"service_name": "api",
			"port":         float64(8080),
			"health_check": true,
		}
		val, err := convertJSONToCty(config)
		must.NoError(t, err)

		m := val.AsValueMap()
		must.Eq(t, "api", m["service_name"].AsString())

		port, _ := m["port"].AsBigFloat().Float64()
		must.Eq(t, 8080.0, port)

		must.True(t, m["health_check"].True())
	})

	t.Run("nested resource limits", func(t *testing.T) {
		ci.Parallel(t)
		// Resource configuration with nested structure
		resources := map[string]any{
			"cpu": float64(500),
			"memory": map[string]any{
				"limit":   float64(256),
				"reserve": float64(128),
			},
		}
		val, err := convertJSONToCty(resources)
		must.NoError(t, err)

		m := val.AsValueMap()
		cpu, _ := m["cpu"].AsBigFloat().Float64()
		must.Eq(t, 500.0, cpu)

		memMap := m["memory"].AsValueMap()
		limit, _ := memMap["limit"].AsBigFloat().Float64()
		must.Eq(t, 256.0, limit)
	})

	t.Run("empty object", func(t *testing.T) {
		ci.Parallel(t)
		val, err := convertJSONToCty(map[string]any{})
		must.NoError(t, err)
		must.Eq(t, cty.EmptyObject, val.Type())
	})
}
