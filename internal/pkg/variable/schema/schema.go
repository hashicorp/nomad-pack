// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package schema

import "github.com/hashicorp/hcl/v2"

const (
	VariableAttributeType        = "type"
	VariableAttributeDefault     = "default"
	VariableAttributeDescription = "description"
)

// VariableFileSchema defines the hcl.BlockHeaderSchema for each root variable
// block. It allows us to capture the label for use as the variable name.
var VariableFileSchema = &hcl.BodySchema{
	Blocks: []hcl.BlockHeaderSchema{
		{
			Type:       "variable",
			LabelNames: []string{"name"},
		},
	},
}

// VariableBlockSchema defines the hcl.BodySchema for a root variable block. It
// allows us to decode blocks effectively.
var VariableBlockSchema = &hcl.BodySchema{
	Attributes: []hcl.AttributeSchema{
		{Name: VariableAttributeDescription},
		{Name: VariableAttributeDefault},
		{Name: VariableAttributeType},
	},
}
