// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package variable

import "github.com/hashicorp/hcl/v2"

const (
	variableAttributeType        = "type"
	variableAttributeDefault     = "default"
	variableAttributeDescription = "description"
)

// variableFileSchema defines the hcl.BlockHeaderSchema for each root variable
// block. It allows us to capture the label for use as the variable name.
var variableFileSchema = &hcl.BodySchema{
	Blocks: []hcl.BlockHeaderSchema{
		{
			Type:       "variable",
			LabelNames: []string{"name"},
		},
	},
}

// variableBlockSchema defines the hcl.BodySchema for a root variable block. It
// allows us to decode blocks effectively.
var variableBlockSchema = &hcl.BodySchema{
	Attributes: []hcl.AttributeSchema{
		{Name: variableAttributeDescription},
		{Name: variableAttributeDefault},
		{Name: variableAttributeType},
	},
}
