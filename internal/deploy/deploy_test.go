package deploy

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func Test_HigherPlanCode(t *testing.T) {
	testCases := []struct {
		inputOld       int
		inputNew       int
		expectedOutput int
		name           string
	}{
		{inputOld: 0, inputNew: 0, expectedOutput: 0, name: "all zeroes"},
		{inputOld: 1, inputNew: 1, expectedOutput: 1, name: "all ones"},
		{inputOld: 255, inputNew: 255, expectedOutput: 255, name: "all two-five-five"},
		{inputOld: 1, inputNew: 0, expectedOutput: 1, name: "old higher"},
		{inputOld: 1, inputNew: 255, expectedOutput: 255, name: "old lower"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			actualOutput := HigherPlanCode(tc.inputOld, tc.inputNew)
			assert.Equal(t, tc.expectedOutput, actualOutput, tc.name)
		})
	}
}
