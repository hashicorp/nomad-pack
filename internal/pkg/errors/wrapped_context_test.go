package errors

import (
	"testing"

	"github.com/hashicorp/hcl/v2"

	"github.com/stretchr/testify/assert"
)

func TestWrappedUIContext_Error(t *testing.T) {
	testCases := []struct {
		inputWrappedUIContext *WrappedUIContext
		expectedOutput        string
		name                  string
	}{
		{
			inputWrappedUIContext: &WrappedUIContext{
				Err:     newError("tis but a scratch"),
				Subject: "the cause of camalot",
				Context: &UIErrorContext{contexts: []string{"King: Arthur"}},
			},
			expectedOutput: "the cause of camalot: tis but a scratch: \nKing: Arthur",
			name:           "basic test 1",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.expectedOutput, tc.inputWrappedUIContext.Error(), tc.name)
		})
	}
}

func TestWrappedUIContext_HCLDiagsToWrappedUIContext(t *testing.T) {
	testCases := []struct {
		inputDiags     hcl.Diagnostics
		expectedOutput []*WrappedUIContext
		name           string
	}{
		{
			inputDiags: hcl.Diagnostics{
				{
					Summary: "some poor diag detail",
					Detail:  "this is the longer detail and is the real error",
					Subject: &hcl.Range{Filename: "test.hcl"},
				},
			},
			expectedOutput: []*WrappedUIContext{
				{
					Err:     newError("this is the longer detail and is the real error"),
					Subject: "some poor diag detail",
					Context: &UIErrorContext{contexts: []string{"HCL Range: test.hcl:0,0-0"}},
				},
			},
			name: "basic test 1",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			assert.ElementsMatch(t, tc.expectedOutput, HCLDiagsToWrappedUIContext(tc.inputDiags), tc.name)
		})
	}
}
