// Copyright IBM Corp. 2023, 2026
// SPDX-License-Identifier: MPL-2.0

package schema

import (
	"testing"

	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/nomad/ci"
	"github.com/shoenig/test/must"
)

func TestNomadVariableBlockSchema(t *testing.T) {
	ci.Parallel(t)

	t.Run("has required path attribute", func(t *testing.T) {
		schema := NomadVariableBlockSchema
		must.NotNil(t, schema)
		must.NotNil(t, schema.Attributes)

		// Find path attribute in slice
		var pathAttr *hcl.AttributeSchema
		for i := range schema.Attributes {
			if schema.Attributes[i].Name == "path" {
				pathAttr = &schema.Attributes[i]
				break
			}
		}
		must.NotNil(t, pathAttr)
		must.Eq(t, "path", pathAttr.Name)
	})

	t.Run("has optional namespace attribute", func(t *testing.T) {
		schema := NomadVariableBlockSchema

		// Find namespace attribute in slice
		var namespaceAttr *hcl.AttributeSchema
		for i := range schema.Attributes {
			if schema.Attributes[i].Name == "namespace" {
				namespaceAttr = &schema.Attributes[i]
				break
			}
		}
		must.NotNil(t, namespaceAttr)
		must.Eq(t, "namespace", namespaceAttr.Name)
	})

	t.Run("has required items attribute", func(t *testing.T) {
		schema := NomadVariableBlockSchema

		// Find items attribute in slice
		var itemsAttr *hcl.AttributeSchema
		for i := range schema.Attributes {
			if schema.Attributes[i].Name == "items" {
				itemsAttr = &schema.Attributes[i]
				break
			}
		}
		must.NotNil(t, itemsAttr)
		must.Eq(t, "items", itemsAttr.Name)
	})
}

func TestVariableFileSchema(t *testing.T) {
	ci.Parallel(t)

	t.Run("recognizes nomad_variable blocks", func(t *testing.T) {
		schema := VariableFileSchema
		must.NotNil(t, schema)
		must.NotNil(t, schema.Blocks)

		// Find nomad_variable block
		var nvBlock *hcl.BlockHeaderSchema
		for i := range schema.Blocks {
			if schema.Blocks[i].Type == "nomad_variable" {
				nvBlock = &schema.Blocks[i]
				break
			}
		}
		must.NotNil(t, nvBlock)
		must.Eq(t, "nomad_variable", nvBlock.Type)
		must.SliceContains(t, nvBlock.LabelNames, "name")
	})

	t.Run("still recognizes variable blocks", func(t *testing.T) {
		schema := VariableFileSchema

		// Find variable block
		var varBlock *hcl.BlockHeaderSchema
		for i := range schema.Blocks {
			if schema.Blocks[i].Type == "variable" {
				varBlock = &schema.Blocks[i]
				break
			}
		}
		must.NotNil(t, varBlock)
		must.Eq(t, "variable", varBlock.Type)
		must.SliceContains(t, varBlock.LabelNames, "name")
	})
}
