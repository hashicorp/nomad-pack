// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package decoder

import (
	"testing"

	"github.com/davecgh/go-spew/spew"
	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/hclparse"
	"github.com/hashicorp/nomad-pack/internal/pkg/variable/schema"
	"github.com/hashicorp/nomad-pack/sdk/pack/variables"
	"github.com/hashicorp/nomad/ci"
	"github.com/shoenig/test/must"
	"github.com/zclconf/go-cty/cty"
)

func TestDecoder_DecodeVariableBlock(t *testing.T) {
	ci.Parallel(t)

	testCases := []struct {
		name        string
		input       *hcl.Block
		expectOut   *variables.Variable
		expectDiags hcl.Diagnostics
		shouldErr   bool
	}{
		{
			name:        "passes/on nil block",
			input:       &hcl.Block{},
			expectOut:   nil,
			expectDiags: hcl.Diagnostics{},
		},
		{
			name:  "passes/on minimal block",
			input: testGetHCLBlock(t, testLoadPackFile(t, []byte(goodMinimalVariableHCL))),
			expectOut: func() *variables.Variable {
				out := variables.Variable{
					Name: "good",
					DeclRange: hcl.Range{
						Filename: "/fake/test/path",
						Start:    hcl.Pos{Line: 1, Column: 1, Byte: 0},
						End:      hcl.Pos{Line: 1, Column: 18, Byte: 17},
					},
				}
				return &out
			}(),
			expectDiags: hcl.Diagnostics{},
		},
		{
			name:  "passes/on good block",
			input: testGetHCLBlock(t, testLoadPackFile(t, []byte(goodCompleteVariableHCL))),
			expectOut: func() *variables.Variable {
				out := variables.Variable{
					Name: "example",
					DeclRange: hcl.Range{
						Filename: "/fake/test/path",
						Start:    hcl.Pos{Line: 2, Column: 1, Byte: 1},
						End:      hcl.Pos{Line: 2, Column: 19, Byte: 19},
					},
				}
				out.SetDefault(cty.StringVal("default"))
				out.SetDescription("an example variable")
				out.SetType(cty.String)
				out.Value = cty.StringVal("default")
				return &out
			}(),
			expectDiags: hcl.Diagnostics{},
		},
		{
			name:      "fails/on bad content",
			input:     testGetHCLBlock(t, testLoadPackFile(t, []byte(badContent))),
			expectOut: nil,
			expectDiags: hcl.Diagnostics{&hcl.Diagnostic{
				Severity: hcl.DiagError,
				Summary:  "Unsupported block type",
				Detail:   "Blocks of type \"bad\" are not expected here.",
				Subject: &hcl.Range{
					Filename: "/fake/test/path",
					Start:    hcl.Pos{Line: 2, Column: 2, Byte: 22},
					End:      hcl.Pos{Line: 2, Column: 5, Byte: 25},
				},
			}},
		},
		{
			name:      "fails/on bad name",
			input:     testGetHCLBlock(t, testLoadPackFile(t, []byte(badNameText))),
			expectOut: nil,
			expectDiags: hcl.Diagnostics{&hcl.Diagnostic{
				Severity: hcl.DiagError,
				Summary:  "Invalid variable name",
				Detail:   "Name must start with a letter or underscore and may contain only letters, digits, underscores, and dashes.",
				Subject: &hcl.Range{
					Filename: "/fake/test/path",
					Start:    hcl.Pos{Line: 1, Column: 1, Byte: 0},
					End:      hcl.Pos{Line: 1, Column: 17, Byte: 16},
				},
			}},
		},
		{
			name:      "fails/on bad description type",
			input:     testGetHCLBlock(t, testLoadPackFile(t, []byte(badDescriptionType))),
			expectOut: nil,
			expectDiags: hcl.Diagnostics{&hcl.Diagnostic{
				Severity: hcl.DiagError,
				Summary:  "Invalid type for description",
				Detail:   "The description attribute is expected to be of type string, got bool",
				Subject: &hcl.Range{
					Filename: "/fake/test/path",
					Start:    hcl.Pos{Line: 2, Column: 2, Byte: 18},
					End:      hcl.Pos{Line: 2, Column: 20, Byte: 36},
				},
			}},
		},
	}
	for _, tc := range testCases {
		tc := tc // capture range variable
		t.Run(tc.name, func(t *testing.T) {
			ci.Parallel(t)
			out, diags := DecodeVariableBlock(tc.input)
			must.Eq(t, tc.expectOut, out)
			if tc.expectDiags != nil {
				spew.Config.DisableMethods = true
				must.SliceContainsAll(t, tc.expectDiags, diags, must.Sprint(spew.Sprint(tc.expectDiags)), must.Sprint(spew.Sprintf("%v", diags)))
			}
		})
	}
}

const goodMinimalVariableHCL = `variable "good" {}`

const goodCompleteVariableHCL = `variable "example" {
	type        = string
	default     = "default"
	description = "an example variable"
}`

const badContent = `variable "example" {
	bad {}
}`

const badNameText = `variable "!bad!" {}`

const badDescriptionType = `variable "bad" {
	description = true
}`

// loadPackFile takes a pack.File and parses this using a hclparse.Parser. The
// file can be either HCL and JSON format.
func testLoadPackFile(t *testing.T, b []byte) hcl.Body {
	t.Helper()

	var (
		hclFile *hcl.File
		diags   hcl.Diagnostics
	)

	hclParser := hclparse.NewParser()
	hclFile, diags = hclParser.ParseHCL(b, "/fake/test/path")

	must.Len(t, 0, diags, must.Sprint(diags.Error()))

	// If the returned file or body is nil, then we'll return a non-nil empty
	// body, so we'll meet our contract that nil means an error reading the
	// file.
	if hclFile == nil || hclFile.Body == nil {
		return hcl.EmptyBody()
	}

	must.Len(t, 0, diags, must.Sprint(diags.Error()))
	return hclFile.Body
}

func testGetHCLBlock(t *testing.T, in hcl.Body) *hcl.Block {
	t.Helper()
	b, diags := in.Content(schema.VariableFileSchema)
	must.Len(t, 0, diags, must.Sprint(diags.Error()))
	must.True(t, len(b.Blocks) >= 1)
	return b.Blocks[0]
}
