// Copyright IBM Corp. 2021, 2025
// SPDX-License-Identifier: MPL-2.0

package flag

import (
	"testing"

	"github.com/shoenig/test/must"
)

func TestSets(t *testing.T) {
	var valA, valB int
	sets := NewSets()
	{
		set := sets.NewSet("setA")
		set.IntVarP(&IntVarP{
			IntVar: &IntVar{
				Name:   "alpha",
				Target: &valA,
			},
			Shorthand: "a",
		})
	}

	{
		set := sets.NewSet("setB")
		set.IntVarP(&IntVarP{
			IntVar: &IntVar{
				Name:   "beta",
				Target: &valB,
			},
			Shorthand: "b",
		})
	}

	testCases := []struct {
		Name        string
		Args        []string
		expectError bool
	}{
		{
			Name: "all small flags",
			Args: []string{"-b", "42", "-a", "21"},
		},
		{
			Name: "positional argument in the middle",
			Args: []string{"-b", "42", "something", "-a", "21"},
		},
		{
			Name: "mixed flag types after positionals",
			Args: []string{"-b", "42", "something", "--alpha", "21"},
		},
		{
			Name: "posix-style long flags",
			Args: []string{"--beta", "42", "something", "--alpha", "21"},
		},
		{
			Name:        "missing value",
			Args:        []string{"-d", "42", "-a", "21"},
			expectError: true,
		},
	}
	for _, tc := range testCases {
		t.Run(tc.Name, func(t *testing.T) {
			err := sets.Parse(tc.Args)
			if tc.expectError {
				must.Error(t, err)
			} else {
				must.NoError(t, err)
				must.Eq(t, int(21), valA)
				must.Eq(t, int(42), valB)
			}
		})
	}
}
