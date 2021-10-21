package renderer

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func Test_toStringList(t *testing.T) {
	testCases := []struct {
		input          []interface{}
		expectedOutput string
	}{
		{
			input:          []interface{}{"dc1", "dc2", "dc3", "dc4"},
			expectedOutput: `["dc1", "dc2", "dc3", "dc4"]`,
		},
		{
			input:          []interface{}{"dc1"},
			expectedOutput: `["dc1"]`,
		},
		{
			input:          []interface{}{},
			expectedOutput: `[]`,
		},
	}

	for _, tc := range testCases {
		actualOutput, _ := toStringList(tc.input)
		assert.Equal(t, tc.expectedOutput, actualOutput)
	}
}
