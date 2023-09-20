package varfile

import (
	"github.com/hashicorp/hcl/v2"
	"github.com/zclconf/go-cty/cty"
)

type Override struct {
	Name  VariableID
	Path  PackID
	Type  cty.Type
	Value cty.Value
	Range hcl.Range
}

type Overrides map[PackID][]*Override

func (o *Override) Equal(a *Override) bool {
	eq := o.Name == a.Name &&
		o.Path == a.Path &&
		o.Range == a.Range &&
		o.Type == a.Type &&
		o.Value.RawEquals(a.Value)
	return eq
}
