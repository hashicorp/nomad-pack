// Copyright IBM Corp. 2021, 2025
// SPDX-License-Identifier: MPL-2.0

package variables

import (
	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/nomad-pack/sdk/pack"
	"github.com/zclconf/go-cty/cty"
)

type Override struct {
	Name  ID
	Path  pack.ID
	Type  cty.Type
	Value cty.Value
	Range hcl.Range
}

type Overrides map[pack.ID][]*Override

func (o *Override) Equal(a *Override) bool {
	eq := o.Name == a.Name &&
		o.Path == a.Path &&
		o.Range == a.Range &&
		o.Type == a.Type &&
		o.Value.RawEquals(a.Value)
	return eq
}
