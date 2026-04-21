// Copyright IBM Corp. 2023, 2026
// SPDX-License-Identifier: MPL-2.0

package variables

import (
	"testing"

	"github.com/hashicorp/nomad/ci"
	"github.com/shoenig/test/must"
	"github.com/zclconf/go-cty/cty"
)

func TestNomadVariable(t *testing.T) {
	ci.Parallel(t)

	t.Run("creates nomad variable with all fields", func(t *testing.T) {
		items := map[string]cty.Value{
			"key1": cty.StringVal("value1"),
			"key2": cty.StringVal("value2"),
		}

		nv := &NomadVariable{
			Name:      "test",
			Path:      "nomad/jobs/test",
			Namespace: "default",
			Items:     items,
		}

		must.Eq(t, "test", nv.Name)
		must.Eq(t, "nomad/jobs/test", nv.Path)
		must.Eq(t, "default", nv.Namespace)
		must.Eq(t, 2, len(nv.Items))
	})

	t.Run("creates nomad variable without namespace", func(t *testing.T) {
		items := map[string]cty.Value{
			"key1": cty.StringVal("value1"),
		}

		nv := &NomadVariable{
			Name:  "test",
			Path:  "nomad/jobs/test",
			Items: items,
		}

		must.Eq(t, "", nv.Namespace)
	})

	t.Run("creates nomad variable with empty items", func(t *testing.T) {
		nv := &NomadVariable{
			Name:  "test",
			Path:  "nomad/jobs/test",
			Items: make(map[string]cty.Value),
		}

		must.Eq(t, 0, len(nv.Items))
	})
}
