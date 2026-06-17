// Copyright IBM Corp. 2023, 2026
// SPDX-License-Identifier: MPL-2.0

package source

import (
	"io"
	"testing"

	"github.com/hashicorp/consul/api"
	consultest "github.com/hashicorp/consul/sdk/testutil"
	"github.com/hashicorp/nomad-pack/sdk/pack"
	"github.com/hashicorp/nomad-pack/sdk/pack/variables"
	"github.com/hashicorp/nomad/ci"
	"github.com/shoenig/test/must"
	"github.com/zclconf/go-cty/cty"
)

// startTestConsul starts an in-process Consul dev agent for integration tests.
//
// It skips the test (rather than failing) when the consul binary is not on
// PATH, so the suite stays green on developer machines without Consul installed
// while still exercising real Consul in CI, where the binary is installed. This
// mirrors the pattern Nomad core uses for its Consul compatibility tests.
func startTestConsul(t *testing.T) *consultest.TestServer {
	t.Helper()

	if testing.Short() {
		t.Skip("-short set; skipping Consul integration test")
	}

	srv, err := consultest.NewTestServerConfigT(t, func(c *consultest.TestServerConfig) {
		c.Peering = nil // older Consul versions don't support peering
		if !testing.Verbose() {
			// Squelch Consul's logs unless the test run is verbose.
			c.Stdout = io.Discard
			c.Stderr = io.Discard
		}
	})
	if err != nil {
		t.Skipf("consul not available, skipping integration test: %v", err)
	}
	t.Cleanup(func() { _ = srv.Stop() })

	srv.WaitForLeader(t)
	return srv
}

// newSourceForServer builds a ConsulSource pointed at the test server's address.
func newSourceForServer(t *testing.T, srv *consultest.TestServer, prefix string, includePackID bool) *ConsulSource {
	t.Helper()
	cfg := api.DefaultConfig()
	cfg.Address = srv.HTTPAddr
	src, err := NewConsulSource(PriorityConsul, cfg, prefix, includePackID)
	must.NoError(t, err)
	return src
}

// varsByName indexes a slice of variables by name for convenient assertions.
func varsByName(vars []*variables.Variable) map[string]*variables.Variable {
	out := make(map[string]*variables.Variable, len(vars))
	for _, v := range vars {
		out[string(v.Name)] = v
	}
	return out
}

// TestConsulSource_Fetch_Integration exercises ConsulSource.Fetch against a real
// Consul KV store. A single Consul agent is shared across subtests, and each
// subtest uses a unique KV prefix to stay isolated.
func TestConsulSource_Fetch_Integration(t *testing.T) {
	ci.Parallel(t)

	srv := startTestConsul(t)

	packID := pack.ID("webapp")
	schema := map[variables.ID]*variables.Variable{
		"replicas": {Name: "replicas", Type: cty.Number},
		"region":   {Name: "region", Type: cty.String},
		"name":     {Name: "name", Type: cty.String},
	}

	t.Run("reads vars under prefix/pack-id", func(t *testing.T) {
		srv.SetKVString(t, "p1/webapp/replicas", "3")
		srv.SetKVString(t, "p1/webapp/region", "us-west-2")

		src := newSourceForServer(t, srv, "p1", true)
		vars, err := src.Fetch(t.Context(), packID, schema)
		must.NoError(t, err)
		must.Len(t, 2, vars)

		got := varsByName(vars)
		replicas, _ := got["replicas"].Value.AsBigFloat().Int64()
		must.Eq(t, int64(3), replicas)
		must.Eq(t, "us-west-2", got["region"].Value.AsString())
	})

	t.Run("full-path mode does not append pack id", func(t *testing.T) {
		srv.SetKVString(t, "p2/path/region", "eu-central-1")

		src := newSourceForServer(t, srv, "p2/path", false)
		vars, err := src.Fetch(t.Context(), packID, schema)
		must.NoError(t, err)
		must.Len(t, 1, vars)
		must.Eq(t, "eu-central-1", varsByName(vars)["region"].Value.AsString())
	})

	t.Run("variables not in schema are skipped", func(t *testing.T) {
		srv.SetKVString(t, "p3/webapp/region", "us-east-1")
		srv.SetKVString(t, "p3/webapp/not_in_pack", "ignored")

		src := newSourceForServer(t, srv, "p3", true)
		vars, err := src.Fetch(t.Context(), packID, schema)
		must.NoError(t, err)
		must.Len(t, 1, vars)
		must.Eq(t, "region", string(vars[0].Name))
	})

	t.Run("empty value for non-string var is skipped so default applies", func(t *testing.T) {
		srv.SetKVString(t, "p4/webapp/replicas", "") // empty: can't decode into a number
		srv.SetKVString(t, "p4/webapp/region", "us-west-1")

		src := newSourceForServer(t, srv, "p4", true)
		vars, err := src.Fetch(t.Context(), packID, schema)
		must.NoError(t, err)
		// replicas (number) with an empty value is skipped so the pack default
		// applies; region (string) is still returned. The whole fetch must not
		// fail just because one key is empty.
		must.Len(t, 1, vars)
		must.Eq(t, "region", string(vars[0].Name))
	})

	t.Run("object var with optional attribute resolves via constraint type", func(t *testing.T) {
		// object({name=string, port=optional(number)}) stored as a partial JSON
		// document. Type has optional() stripped; ConstraintType preserves it.
		objSchema := map[variables.ID]*variables.Variable{
			"svc": {
				Name: "svc",
				Type: cty.Object(map[string]cty.Type{
					"name": cty.String,
					"port": cty.Number,
				}),
				ConstraintType: cty.ObjectWithOptionalAttrs(
					map[string]cty.Type{"name": cty.String, "port": cty.Number},
					[]string{"port"},
				),
			},
		}
		srv.SetKVString(t, "p5/webapp/svc", `{"name":"api"}`)

		src := newSourceForServer(t, srv, "p5", true)
		vars, err := src.Fetch(t.Context(), packID, objSchema)
		must.NoError(t, err)
		must.Len(t, 1, vars)
		must.Eq(t, "api", vars[0].Value.GetAttr("name").AsString())
		must.True(t, vars[0].Value.GetAttr("port").IsNull())
	})

	t.Run("no keys under prefix returns no variables", func(t *testing.T) {
		src := newSourceForServer(t, srv, "p6-empty", true)
		vars, err := src.Fetch(t.Context(), pack.ID("absent"), schema)
		must.NoError(t, err)
		must.Len(t, 0, vars)
	})
}
