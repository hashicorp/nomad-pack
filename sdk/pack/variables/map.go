package variables

import "github.com/hashicorp/nomad-pack/sdk/pack"

type PackIDKeyedVarMap map[pack.ID][]*Variable

func (m PackIDKeyedVarMap) Variables(k pack.ID) []*Variable {
	return m[k]
}

func (m PackIDKeyedVarMap) AsMapOfStringToVariable() map[string][]*Variable {
	o := make(map[string][]*Variable)
	for k, v := range m {
		o[string(k)] = v
	}
	return o
}
