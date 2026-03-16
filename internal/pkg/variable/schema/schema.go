// Copyright IBM Corp. 2021, 2025
// SPDX-License-Identifier: MPL-2.0

package schema

import "github.com/hashicorp/hcl/v2"

const (
	VariableAttributeType        = "type"
	VariableAttributeDefault     = "default"
	VariableAttributeDescription = "description"

	VariableBlockValidation         = "validation"
	ValidationAttributeCondition    = "condition"
	ValidationAttributeErrorMessage = "error_message"
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
	Blocks: []hcl.BlockHeaderSchema{
		{Type: VariableBlockValidation},
	},
}

// ValidationBlockSchema defines the hcl.BodySchema for a validation block
// inside a variable definition.
var ValidationBlockSchema = &hcl.BodySchema{
	Attributes: []hcl.AttributeSchema{
		{Name: ValidationAttributeCondition, Required: true},
		{Name: ValidationAttributeErrorMessage, Required: true},
	},
}
