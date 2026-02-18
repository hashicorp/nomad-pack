// Copyright IBM Corp. 2021, 2025
// SPDX-License-Identifier: MPL-2.0

package errors

import (
	"testing"

	"github.com/hashicorp/hcl/v2"
	"github.com/shoenig/test/must"
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
			must.Eq(t, tc.expectedOutput, tc.inputWrappedUIContext.Error())
		})
	}
}

func TestWrappedUIContext_HCLDiagsToWrappedUIContext(t *testing.T) {
	testCases := []struct {
		inputDiags              hcl.Diagnostics
		expectedSummary         string
		expectedDetail          string
		expectedHCLRangeContext string
		name                    string
	}{
		{
			inputDiags: hcl.Diagnostics{
				{
					Summary: "some poor diag detail",
					Detail:  "this is the longer detail and is the real error",
					Subject: &hcl.Range{Filename: "test.hcl"},
				},
			},
			expectedSummary:         "some poor diag detail",
			expectedDetail:          "this is the longer detail and is the real error",
			expectedHCLRangeContext: "HCL Range: test.hcl",
			name:                    "subject with no line data (filename only)",
		},
		{
			inputDiags: hcl.Diagnostics{
				{
					Summary: "variable type error",
					Detail:  "duplicate type attribute",
					Subject: &hcl.Range{
						Filename: "variables.hcl",
						Start:    hcl.Pos{Line: 3, Column: 3, Byte: 45},
						End:      hcl.Pos{Line: 3, Column: 7, Byte: 49},
					},
				},
			},
			expectedSummary:         "variable type error",
			expectedDetail:          "duplicate type attribute",
			expectedHCLRangeContext: "HCL Range: variables.hcl:3,3-7",
			name:                    "subject with line and column data (same line)",
		},
		{
			inputDiags: hcl.Diagnostics{
				{
					Summary: "context fallback test",
					Detail:  "error with context range",
					Context: &hcl.Range{
						Filename: "override.hcl",
						Start:    hcl.Pos{Line: 5, Column: 1, Byte: 100},
						End:      hcl.Pos{Line: 5, Column: 10, Byte: 109},
					},
				},
			},
			expectedSummary:         "context fallback test",
			expectedDetail:          "error with context range",
			expectedHCLRangeContext: "HCL Range: override.hcl:5,1-10",
			name:                    "context range fallback when subject is nil",
		},
		{
			inputDiags: hcl.Diagnostics{
				{
					Summary: "no range test",
					Detail:  "error with no range information",
				},
			},
			expectedSummary:         "no range test",
			expectedDetail:          "error with no range information",
			expectedHCLRangeContext: "",
			name:                    "no subject or context range (no context added)",
		},
		{
			inputDiags: hcl.Diagnostics{
				{
					Summary: "multi-line range test",
					Detail:  "error spanning multiple lines",
					Subject: &hcl.Range{
						Filename: "complex.hcl",
						Start:    hcl.Pos{Line: 2, Column: 5, Byte: 50},
						End:      hcl.Pos{Line: 4, Column: 8, Byte: 100},
					},
				},
			},
			expectedSummary:         "multi-line range test",
			expectedDetail:          "error spanning multiple lines",
			expectedHCLRangeContext: "HCL Range: complex.hcl:2,5-4,8",
			name:                    "subject with multi-line range",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := HCLDiagsToWrappedUIContext(tc.inputDiags)
			must.True(t, len(result) == 1)
			must.Eq(t, result[0].Subject, tc.expectedSummary)
			must.Eq(t, result[0].Err.Error(), tc.expectedDetail)
			ctxAll := result[0].Context.GetAll()
			if tc.expectedHCLRangeContext == "" {
				must.True(t, len(ctxAll) == 0)
			} else {
				must.True(t, len(ctxAll) == 1)
				must.Eq(t, ctxAll[0], tc.expectedHCLRangeContext)
			}
		})
	}
}
