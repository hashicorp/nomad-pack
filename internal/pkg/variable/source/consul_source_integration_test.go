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

func startTestConsul(t *testing.T) *consultest.TestServer {
	t.Helper()

	srv, err := consultest.NewTestServerConfigT(t, func(c *consultest.TestServerConfig) {
		c.Peering = nil
		if !testing.Verbose() {
			c.Stdout = io.Discard
			c.Stderr = io.Discard
		}
	})
	if err != nil {
		t.Fatalf("failed to start Consul test server: %v", err)
	}
	t.Cleanup(func() { _ = srv.Stop() })

	srv.WaitForLeader(t)
	return srv
}

func newSourceForServer(t *testing.T, srv *consultest.TestServer, path string) *ConsulSource {
	t.Helper()
	cfg := api.DefaultConfig()
	cfg.Address = srv.HTTPAddr
	src, err := NewConsulSource(PriorityConsul, cfg, path)
	must.NoError(t, err)
	return src
}

func varsByName(vars []*variables.Variable) map[string]*variables.Variable {
	out := make(map[string]*variables.Variable, len(vars))
	for _, v := range vars {
		out[string(v.Name)] = v
	}
	return out
}

func TestConsulSource_Fetch_Integration(t *testing.T) {
	ci.Parallel(t)

	srv := startTestConsul(t)

	packID := pack.ID("webapp")
	schema := map[variables.ID]*variables.Variable{
		"replicas": {Name: "replicas", Type: cty.Number},
		"region":   {Name: "region", Type: cty.String},
		"name":     {Name: "name", Type: cty.String},
	}

	t.Run("fetches typed variables from KV path", func(t *testing.T) {
		srv.SetKVString(t, "deploy/webapp/replicas", "3")
		srv.SetKVString(t, "deploy/webapp/region", "us-west-2")

		src := newSourceForServer(t, srv, "deploy/webapp")
		vars, err := src.Fetch(t.Context(), packID, schema)
		must.NoError(t, err)
		must.Len(t, 2, vars)

		got := varsByName(vars)
		replicas, _ := got["replicas"].Value.AsBigFloat().Int64()
		must.Eq(t, int64(3), replicas)
		must.Eq(t, "us-west-2", got["region"].Value.AsString())
	})

	t.Run("path is not modified", func(t *testing.T) {
		srv.SetKVString(t, "ops/webapp/region", "eu-central-1")

		src := newSourceForServer(t, srv, "ops/webapp")
		vars, err := src.Fetch(t.Context(), packID, schema)
		must.NoError(t, err)
		must.Len(t, 1, vars)
		must.Eq(t, "eu-central-1", varsByName(vars)["region"].Value.AsString())
	})

	t.Run("keys not in pack schema are ignored", func(t *testing.T) {
		srv.SetKVString(t, "staging/webapp/region", "us-east-1")
		srv.SetKVString(t, "staging/webapp/not_in_pack", "ignored")

		src := newSourceForServer(t, srv, "staging/webapp")
		vars, err := src.Fetch(t.Context(), packID, schema)
		must.NoError(t, err)
		must.Len(t, 1, vars)
		must.Eq(t, "region", string(vars[0].Name))
	})

	t.Run("empty value for non-string variable is an error", func(t *testing.T) {
		srv.SetKVString(t, "prod/webapp/replicas", "")
		srv.SetKVString(t, "prod/webapp/region", "us-west-1")

		src := newSourceForServer(t, srv, "prod/webapp")
		_, err := src.Fetch(t.Context(), packID, schema)
		must.ErrorContains(t, err, "empty Consul value")
	})

	t.Run("object with optional field missing is valid", func(t *testing.T) {
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
		srv.SetKVString(t, "services/webapp/svc", `{"name":"api"}`)

		src := newSourceForServer(t, srv, "services/webapp")
		vars, err := src.Fetch(t.Context(), packID, objSchema)
		must.NoError(t, err)
		must.Len(t, 1, vars)
		must.Eq(t, "api", vars[0].Value.GetAttr("name").AsString())
		must.True(t, vars[0].Value.GetAttr("port").IsNull())
	})

	t.Run("bool variable is decoded from JSON", func(t *testing.T) {
		boolSchema := map[variables.ID]*variables.Variable{
			"enabled": {Name: "enabled", Type: cty.Bool},
		}
		srv.SetKVString(t, "config/webapp/enabled", "true")

		src := newSourceForServer(t, srv, "config/webapp")
		vars, err := src.Fetch(t.Context(), packID, boolSchema)
		must.NoError(t, err)
		must.Len(t, 1, vars)
		must.True(t, vars[0].Value.True())
	})

	t.Run("malformed JSON for non-string variable is an error", func(t *testing.T) {
		srv.SetKVString(t, "broken/webapp/replicas", "not-a-number")

		src := newSourceForServer(t, srv, "broken/webapp")
		_, err := src.Fetch(t.Context(), packID, schema)
		must.ErrorContains(t, err, "decoding Consul value")
	})

	t.Run("empty string value is kept for string variable", func(t *testing.T) {
		srv.SetKVString(t, "defaults/webapp/name", "")

		src := newSourceForServer(t, srv, "defaults/webapp")
		vars, err := src.Fetch(t.Context(), packID, schema)
		must.NoError(t, err)
		must.Len(t, 1, vars)
		must.Eq(t, "", vars[0].Value.AsString())
	})

	t.Run("path with no keys returns empty result", func(t *testing.T) {
		src := newSourceForServer(t, srv, "empty/webapp")
		vars, err := src.Fetch(t.Context(), packID, schema)
		must.NoError(t, err)
		must.Len(t, 0, vars)
	})
}
