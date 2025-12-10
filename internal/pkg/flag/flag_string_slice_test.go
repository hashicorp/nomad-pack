// Copyright IBM Corp. 2021, 2025
// SPDX-License-Identifier: MPL-2.0

package flag

import (
	"testing"

	"github.com/shoenig/test/must"
)

func TestStringSlice(t *testing.T) {
	var valA, valB []string
	sets := NewSets()
	{
		set := sets.NewSet("A")
		set.StringSliceVar(&StringSliceVar{
			Name:   "a",
			Target: &valA,
		})
	}

	{
		set := sets.NewSet("B")
		set.StringSliceVar(&StringSliceVar{
			Name:   "b",
			Target: &valB,
		})
	}

	err := sets.Parse([]string{
		"--b", "somevalueB",
		"--a", "somevalueA,somevalueB",
	})
	must.NoError(t, err)

	must.Eq(t, []string{"somevalueB"}, valB)
	must.Eq(t, []string{"somevalueA", "somevalueB"}, valA)
}
