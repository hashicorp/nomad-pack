// Copyright IBM Corp. 2023, 2026
// SPDX-License-Identifier: MPL-2.0

package source

import (
	"fmt"
	"io"
	"os/exec"
	"testing"
	"time"

	"github.com/hashicorp/nomad-pack/sdk/pack"
	"github.com/hashicorp/nomad-pack/sdk/pack/variables"
	"github.com/hashicorp/nomad/ci"
	vaultapi "github.com/hashicorp/vault/api"
	"github.com/shoenig/test/must"
	"github.com/zclconf/go-cty/cty"
)

const devRootToken = "root"

func startTestVault(t *testing.T) string {
	t.Helper()

	port := ci.PortAllocator.One()
	listen := fmt.Sprintf("127.0.0.1:%d", port)
	addr := "http://" + listen

	cmd := exec.Command("vault", "server", "-dev",
		"-dev-root-token-id="+devRootToken,
		"-dev-listen-address="+listen,
	)
	if !testing.Verbose() {
		cmd.Stdout = io.Discard
		cmd.Stderr = io.Discard
	}
	must.NoError(t, cmd.Start())
	t.Cleanup(func() {
		_ = cmd.Process.Kill()
		_, _ = cmd.Process.Wait()
	})

	waitForVault(t, addr)
	return addr
}

func waitForVault(t *testing.T, addr string) {
	t.Helper()

	cfg := vaultapi.DefaultConfig()
	cfg.Address = addr
	client, err := vaultapi.NewClient(cfg)
	must.NoError(t, err)

	deadline := time.Now().Add(20 * time.Second)
	for time.Now().Before(deadline) {
		health, err := client.Sys().Health()
		if err == nil && health.Initialized && !health.Sealed {
			return
		}
		time.Sleep(100 * time.Millisecond)
	}
	t.Fatal("vault dev server did not become ready within 20s")
}

func newVaultClient(t *testing.T, addr string) *vaultapi.Client {
	t.Helper()
	cfg := vaultapi.DefaultConfig()
	cfg.Address = addr
	client, err := vaultapi.NewClient(cfg)
	must.NoError(t, err)
	client.SetToken(devRootToken)
	return client
}

func writeSecret(t *testing.T, client *vaultapi.Client, path string, data map[string]any) {
	t.Helper()
	_, err := client.KVv2("secret").Put(t.Context(), path, data)
	must.NoError(t, err)
}

func newVaultSourceForServer(t *testing.T, addr, mount, path string) *VaultSource {
	t.Helper()
	cfg := vaultapi.DefaultConfig()
	cfg.Address = addr
	src, err := NewVaultSource(PriorityVault, cfg, mount, path)
	must.NoError(t, err)
	src.client.SetToken(devRootToken)
	return src
}

func vaultVarsByName(vars []*variables.Variable) map[string]*variables.Variable {
	out := make(map[string]*variables.Variable, len(vars))
	for _, v := range vars {
		out[string(v.Name)] = v
	}
	return out
}

func TestVaultSource_Fetch(t *testing.T) {
	ci.Parallel(t)

	addr := startTestVault(t)
	client := newVaultClient(t, addr)

	packID := pack.ID("webapp")
	schema := map[variables.ID]*variables.Variable{
		"replicas": {Name: "replicas", Type: cty.Number},
		"region":   {Name: "region", Type: cty.String},
		"name":     {Name: "name", Type: cty.String},
	}

	t.Run("fetches typed variables from secret", func(t *testing.T) {
		writeSecret(t, client, "deploy/webapp", map[string]any{
			"replicas": "3",
			"region":   "us-west-2",
		})

		src := newVaultSourceForServer(t, addr, "secret", "deploy/webapp")
		vars, err := src.Fetch(t.Context(), packID, schema)
		must.NoError(t, err)
		must.Len(t, 2, vars)

		got := vaultVarsByName(vars)
		replicas, _ := got["replicas"].Value.AsBigFloat().Int64()
		must.Eq(t, int64(3), replicas)
		must.Eq(t, "us-west-2", got["region"].Value.AsString())
	})

	t.Run("fields not in pack schema are ignored", func(t *testing.T) {
		writeSecret(t, client, "staging/webapp", map[string]any{
			"region":      "us-east-1",
			"not_in_pack": "ignored",
		})

		src := newVaultSourceForServer(t, addr, "secret", "staging/webapp")
		vars, err := src.Fetch(t.Context(), packID, schema)
		must.NoError(t, err)
		must.Len(t, 1, vars)
		must.Eq(t, "region", string(vars[0].Name))
	})

	t.Run("empty value for non-string variable is an error", func(t *testing.T) {
		writeSecret(t, client, "prod/webapp", map[string]any{
			"replicas": "",
			"region":   "us-west-1",
		})

		src := newVaultSourceForServer(t, addr, "secret", "prod/webapp")
		_, err := src.Fetch(t.Context(), packID, schema)
		must.ErrorContains(t, err, "empty Vault value")
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
		writeSecret(t, client, "services/webapp", map[string]any{
			"svc": `{"name":"api"}`,
		})

		src := newVaultSourceForServer(t, addr, "secret", "services/webapp")
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
		writeSecret(t, client, "config/webapp", map[string]any{
			"enabled": "true",
		})

		src := newVaultSourceForServer(t, addr, "secret", "config/webapp")
		vars, err := src.Fetch(t.Context(), packID, boolSchema)
		must.NoError(t, err)
		must.Len(t, 1, vars)
		must.True(t, vars[0].Value.True())
	})

	t.Run("malformed JSON for non-string variable is an error", func(t *testing.T) {
		writeSecret(t, client, "broken/webapp", map[string]any{
			"replicas": "not-a-number",
		})

		src := newVaultSourceForServer(t, addr, "secret", "broken/webapp")
		_, err := src.Fetch(t.Context(), packID, schema)
		must.ErrorContains(t, err, "decoding Vault value")
	})

	t.Run("empty string value is kept for string variable", func(t *testing.T) {
		writeSecret(t, client, "defaults/webapp", map[string]any{
			"name": "",
		})

		src := newVaultSourceForServer(t, addr, "secret", "defaults/webapp")
		vars, err := src.Fetch(t.Context(), packID, schema)
		must.NoError(t, err)
		must.Len(t, 1, vars)
		must.Eq(t, "", vars[0].Value.AsString())
	})

	t.Run("secret not found returns empty result", func(t *testing.T) {
		src := newVaultSourceForServer(t, addr, "secret", "empty/webapp")
		vars, err := src.Fetch(t.Context(), packID, schema)
		must.NoError(t, err)
		must.Len(t, 0, vars)
	})

	t.Run("non-string field value is an error", func(t *testing.T) {
		// replicas is stored as a JSON number rather than a string.
		writeSecret(t, client, "typed/webapp", map[string]any{
			"replicas": 3,
		})

		src := newVaultSourceForServer(t, addr, "secret", "typed/webapp")
		_, err := src.Fetch(t.Context(), packID, schema)
		must.ErrorContains(t, err, "is not a string")
	})

	t.Run("deleted secret returns empty result", func(t *testing.T) {
		writeSecret(t, client, "deleted/webapp", map[string]any{
			"region": "us-west-2",
		})
		must.NoError(t, client.KVv2("secret").Delete(t.Context(), "deleted/webapp"))

		src := newVaultSourceForServer(t, addr, "secret", "deleted/webapp")
		vars, err := src.Fetch(t.Context(), packID, schema)
		must.NoError(t, err)
		must.Len(t, 0, vars)
	})

	t.Run("vault unavailable returns read error", func(t *testing.T) {
		cfg := vaultapi.DefaultConfig()
		cfg.Address = fmt.Sprintf("http://127.0.0.1:%d", ci.PortAllocator.One())
		src, err := NewVaultSource(PriorityVault, cfg, "secret", "any/path")
		must.NoError(t, err)
		src.client.SetToken(devRootToken)

		_, err = src.Fetch(t.Context(), packID, schema)
		must.ErrorContains(t, err, "failed to read Vault secret")
	})

	t.Run("mount and path with surrounding slashes fetch correctly", func(t *testing.T) {
		writeSecret(t, client, "norm/webapp", map[string]any{
			"region": "ap-southeast-1",
		})

		src := newVaultSourceForServer(t, addr, "/secret/", "/norm/webapp/")
		vars, err := src.Fetch(t.Context(), packID, schema)
		must.NoError(t, err)
		must.Len(t, 1, vars)
		must.Eq(t, "ap-southeast-1", vaultVarsByName(vars)["region"].Value.AsString())
	})
}
