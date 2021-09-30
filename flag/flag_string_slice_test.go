package flag

import (
	"testing"

	"github.com/stretchr/testify/require"
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
	require.NoError(t, err)

	require.Equal(t, []string{"somevalueB"}, valB)
	require.Equal(t, []string{"somevalueA", "somevalueB"}, valA)
}
