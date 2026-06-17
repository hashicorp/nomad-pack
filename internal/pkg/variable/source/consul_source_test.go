// Copyright IBM Corp. 2023, 2026
// SPDX-License-Identifier: MPL-2.0

package source

import (
	"testing"

	"github.com/hashicorp/consul/api"
	"github.com/hashicorp/nomad/ci"
	"github.com/shoenig/test/must"
	"github.com/zclconf/go-cty/cty"
)

// TestConsulSource_convertValueWithSchema verifies that raw Consul KV bytes are
// converted using the variable's declared type, rather than by guessing the
// type from the value. This is the behavior reviewers asked for: a value is
// only ever coerced into the type the pack actually declares.
func TestConsulSource_convertValueWithSchema(t *testing.T) {
	ci.Parallel(t)

	// convertValueWithSchema uses no receiver state, so a zero-value source is
	// enough to exercise it.
	c := &ConsulSource{}

	t.Run("string type keeps raw bytes even when valid JSON", func(t *testing.T) {
		ci.Parallel(t)
		// A string variable must preserve the exact bytes, so a JSON document
		// stored in Consul stays a string instead of being decoded. This is the
		// case that broke when the source guessed the type.
		val, err := c.convertValueWithSchema([]byte(`{"hello":"world"}`), cty.String)
		must.NoError(t, err)
		must.Eq(t, cty.String, val.Type())
		must.Eq(t, `{"hello":"world"}`, val.AsString())
	})

	t.Run("number type parses JSON number", func(t *testing.T) {
		ci.Parallel(t)
		val, err := c.convertValueWithSchema([]byte("3"), cty.Number)
		must.NoError(t, err)
		must.Eq(t, cty.Number, val.Type())
		got, _ := val.AsBigFloat().Int64()
		must.Eq(t, int64(3), got)
	})

	t.Run("bool type parses JSON boolean", func(t *testing.T) {
		ci.Parallel(t)
		val, err := c.convertValueWithSchema([]byte("true"), cty.Bool)
		must.NoError(t, err)
		must.True(t, val.True())
	})

	t.Run("list type parses JSON array", func(t *testing.T) {
		ci.Parallel(t)
		val, err := c.convertValueWithSchema([]byte(`["dc1","dc2"]`), cty.List(cty.String))
		must.NoError(t, err)
		must.Eq(t, cty.List(cty.String), val.Type())
		must.Eq(t, 2, val.LengthInt())
	})

	t.Run("object with optional attribute omitted", func(t *testing.T) {
		ci.Parallel(t)
		// The pack declares object({name=string, port=optional(number)}). The
		// ConstraintType preserves optional(), so a Consul value missing "port"
		// must still convert, with port set to null. Converting against the
		// plain Type (which strips optional) would fail with "attribute port is
		// required".
		constraint := cty.ObjectWithOptionalAttrs(
			map[string]cty.Type{"name": cty.String, "port": cty.Number},
			[]string{"port"},
		)
		val, err := c.convertValueWithSchema([]byte(`{"name":"api"}`), constraint)
		must.NoError(t, err)
		must.Eq(t, "api", val.GetAttr("name").AsString())
		must.True(t, val.GetAttr("port").IsNull())
	})

	t.Run("type mismatch returns error", func(t *testing.T) {
		ci.Parallel(t)
		// A JSON string cannot be coerced into a number.
		_, err := c.convertValueWithSchema([]byte(`"not-a-number"`), cty.Number)
		must.ErrorContains(t, err, "type mismatch")
	})

	t.Run("invalid JSON for non-string type returns error", func(t *testing.T) {
		ci.Parallel(t)
		_, err := c.convertValueWithSchema([]byte("not json"), cty.Number)
		must.ErrorContains(t, err, "not valid JSON")
	})
}

// TestNewConsulSource verifies that the source is constructed with a unique,
// descriptive name and a normalized prefix, without making any network calls.
func TestNewConsulSource(t *testing.T) {
	ci.Parallel(t)

	t.Run("name encodes address and trimmed prefix", func(t *testing.T) {
		ci.Parallel(t)
		cfg := api.DefaultConfig()
		cfg.Address = "consul.example:8500"

		src, err := NewConsulSource(PriorityConsul, cfg, "/my/prefix/", true)
		must.NoError(t, err)
		must.Eq(t, PriorityConsul, src.Priority())
		// Leading/trailing slashes are trimmed, and the name embeds the address
		// and prefix so multiple Consul sources never collide.
		must.Eq(t, "consul(consul.example:8500:my/prefix)", src.Name())
	})

	t.Run("nil config falls back to Consul defaults", func(t *testing.T) {
		ci.Parallel(t)
		src, err := NewConsulSource(PriorityConsul, nil, "prefix", false)
		must.NoError(t, err)
		must.NotNil(t, src)
	})
}
