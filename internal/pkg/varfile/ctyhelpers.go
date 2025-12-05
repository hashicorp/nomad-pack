// Copyright IBM Corp. 2021, 2025
// SPDX-License-Identifier: MPL-2.0

package varfile

import (
	"fmt"

	"github.com/hashicorp/hcl/v2"
)

func traversalToName(t hcl.Traversal) []string {
	acc := make([]string, len(t)) // make an accumulator
	travToNameR(t, 0, &acc)
	return acc
}

func travToNameR(t hcl.Traversal, cur int, acc *[]string) {
	if len(t) == 0 { // base case for the recursion
		return
	}
	(*acc)[cur] = getStepName(t[0])
	travToNameR(t[1:], cur+1, acc)
}

func getStepName(t any) string {
	if _, ok := t.(hcl.Traverser); !ok {
		panic(fmt.Sprintf("can't getStepName for non hcl.Traverser type %T", t))
	}
	switch tt := t.(type) {
	case hcl.TraverseRoot:
		return tt.Name
	case hcl.TraverseAttr:
		return tt.Name
	case hcl.TraverseIndex:
		return tt.Key.AsString()
	default:
		panic(fmt.Sprintf("can't getStepName for hcl.Traverser type %T", tt))
	}
}
