// Copyright IBM Corp. 2023, 2026
// SPDX-License-Identifier: MPL-2.0

package source

import (
	"fmt"
	"io"
	"os/exec"
	"testing"

	"github.com/hashicorp/nomad-pack/sdk/pack"
	"github.com/hashicorp/nomad-pack/sdk/pack/variables"
	"github.com/hashicorp/nomad/api"
	"github.com/hashicorp/nomad/ci"
	"github.com/hashicorp/nomad/testutil"
	"github.com/shoenig/test/must"
	"github.com/zclconf/go-cty/cty"
)

func startTestNomad(t *testing.T) *testutil.TestServer {
	t.Helper()

	if _, err := exec.LookPath("nomad"); err != nil {
		t.Fatalf("nomad binary not found on $PATH: %v", err)
	}

	srv := testutil.NewTestServer(t, func(c *testutil.TestServerConfig) {
		if !testing.Verbose() {
			c.Stdout = io.Discard
			c.Stderr = io.Discard
		}
	})
	t.Cleanup(srv.Stop)
	return srv
}

func newNomadClient(t *testing.T, srv *testutil.TestServer) *api.Client {
	t.Helper()
	cfg := api.DefaultConfig()
	cfg.Address = "http://" + srv.HTTPAddr
	client, err := api.NewClient(cfg)
	must.NoError(t, err)
	return client
}

func writeNomadVar(t *testing.T, client *api.Client, path string, items map[string]string) {
	t.Helper()
	_, _, err := client.Variables().Create(&api.Variable{
		Path:  path,
		Items: items,
	}, nil)
	must.NoError(t, err)
}

func newNomadSourceForServer(t *testing.T, srv *testutil.TestServer, path string) *NomadSource {
	t.Helper()
	cfg := api.DefaultConfig()
	cfg.Address = "http://" + srv.HTTPAddr
	src, err := NewNomadSource(PriorityExternalBase, cfg, path)
	must.NoError(t, err)
	return src
}

func nomadVarsByName(vars []*variables.Variable) map[string]*variables.Variable {
	out := make(map[string]*variables.Variable, len(vars))
	for _, v := range vars {
		out[string(v.Name)] = v
	}
	return out
}

func TestNomadSource_Fetch(t *testing.T) {
	ci.Parallel(t)

	srv := startTestNomad(t)
	client := newNomadClient(t, srv)

	packID := pack.ID("webapp")
	schema := map[variables.ID]*variables.Variable{
		"replicas": {Name: "replicas", Type: cty.Number},
		"region":   {Name: "region", Type: cty.String},
		"name":     {Name: "name", Type: cty.String},
	}

	t.Run("fetches typed variables from a variable", func(t *testing.T) {
		writeNomadVar(t, client, "deploy/webapp", map[string]string{
			"replicas": "3",
			"region":   "us-west-2",
		})

		src := newNomadSourceForServer(t, srv, "deploy/webapp")
		vars, err := src.Fetch(t.Context(), packID, schema)
		must.NoError(t, err)
		must.Len(t, 2, vars)

		got := nomadVarsByName(vars)
		replicas, _ := got["replicas"].Value.AsBigFloat().Int64()
		must.Eq(t, int64(3), replicas)
		must.Eq(t, "us-west-2", got["region"].Value.AsString())
	})

	t.Run("items not in pack schema are ignored", func(t *testing.T) {
		writeNomadVar(t, client, "staging/webapp", map[string]string{
			"region":      "us-east-1",
			"not_in_pack": "ignored",
		})

		src := newNomadSourceForServer(t, srv, "staging/webapp")
		vars, err := src.Fetch(t.Context(), packID, schema)
		must.NoError(t, err)
		must.Len(t, 1, vars)
		must.Eq(t, "region", string(vars[0].Name))
	})

	t.Run("empty value for non-string variable is an error", func(t *testing.T) {
		writeNomadVar(t, client, "prod/webapp", map[string]string{
			"replicas": "",
			"region":   "us-west-1",
		})

		src := newNomadSourceForServer(t, srv, "prod/webapp")
		_, err := src.Fetch(t.Context(), packID, schema)
		must.ErrorContains(t, err, "empty Nomad value")
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
		writeNomadVar(t, client, "services/webapp", map[string]string{
			"svc": `{"name":"api"}`,
		})

		src := newNomadSourceForServer(t, srv, "services/webapp")
		vars, err := src.Fetch(t.Context(), packID, objSchema)
		must.NoError(t, err)
		must.Len(t, 1, vars)
		must.Eq(t, "api", vars[0].Value.GetAttr("name").AsString())
		must.True(t, vars[0].Value.GetAttr("port").IsNull())
	})

	t.Run("list variable is decoded from JSON", func(t *testing.T) {
		listSchema := map[variables.ID]*variables.Variable{
			"zones": {Name: "zones", Type: cty.List(cty.String)},
		}
		writeNomadVar(t, client, "config/webapp", map[string]string{
			"zones": `["us-east-1a","us-east-1b"]`,
		})

		src := newNomadSourceForServer(t, srv, "config/webapp")
		vars, err := src.Fetch(t.Context(), packID, listSchema)
		must.NoError(t, err)
		must.Len(t, 1, vars)
		must.Eq(t, 2, vars[0].Value.LengthInt())
		must.Eq(t, "us-east-1a", vars[0].Value.Index(cty.NumberIntVal(0)).AsString())
	})

	t.Run("malformed JSON for non-string variable is an error", func(t *testing.T) {
		writeNomadVar(t, client, "broken/webapp", map[string]string{
			"replicas": "not-a-number",
		})

		src := newNomadSourceForServer(t, srv, "broken/webapp")
		_, err := src.Fetch(t.Context(), packID, schema)
		must.ErrorContains(t, err, "decoding Nomad value")
	})

	t.Run("empty string value is kept for string variable", func(t *testing.T) {
		writeNomadVar(t, client, "defaults/webapp", map[string]string{
			"name": "",
		})

		src := newNomadSourceForServer(t, srv, "defaults/webapp")
		vars, err := src.Fetch(t.Context(), packID, schema)
		must.NoError(t, err)
		must.Len(t, 1, vars)
		must.Eq(t, "", vars[0].Value.AsString())
	})

	t.Run("variable not found returns empty result", func(t *testing.T) {
		src := newNomadSourceForServer(t, srv, "missing/webapp")
		vars, err := src.Fetch(t.Context(), packID, schema)
		must.NoError(t, err)
		must.Len(t, 0, vars)
	})

	t.Run("variable with no matching items returns empty result", func(t *testing.T) {
		writeNomadVar(t, client, "nomatch/webapp", map[string]string{
			"not_in_pack": "ignored",
		})

		src := newNomadSourceForServer(t, srv, "nomatch/webapp")
		vars, err := src.Fetch(t.Context(), packID, schema)
		must.NoError(t, err)
		must.Len(t, 0, vars)
	})

	t.Run("nomad unavailable returns read error", func(t *testing.T) {
		cfg := api.DefaultConfig()
		cfg.Address = fmt.Sprintf("http://127.0.0.1:%d", ci.PortAllocator.One())
		src, err := NewNomadSource(PriorityExternalBase, cfg, "any/path")
		must.NoError(t, err)

		_, err = src.Fetch(t.Context(), packID, schema)
		must.ErrorContains(t, err, "failed to read Nomad variable")
	})
}
