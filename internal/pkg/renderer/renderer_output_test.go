// Copyright IBM Corp. 2023, 2026
// SPDX-License-Identifier: MPL-2.0

package renderer

import (
	"testing"
	"text/template"

	"github.com/hashicorp/nomad-pack/internal/pkg/variable/parser"
	"github.com/hashicorp/nomad-pack/sdk/pack"
	"github.com/hashicorp/nomad-pack/sdk/pack/variables"
	"github.com/stretchr/testify/require"
	"github.com/zclconf/go-cty/cty"
)

// TestRenderOutput_V1Parser_Success tests that output template rendering
// works correctly with V1 parser. This is the fix for issue #525.
func TestRenderOutput_V1Parser_Success(t *testing.T) {
	// Create a pack with output template using V1 syntax
	p := &pack.Pack{
		Metadata: &pack.Metadata{
			Pack: &pack.MetadataPack{
				Name: "testpack",
			},
			App: &pack.MetadataApp{},
		},
		OutputTemplateFile: &pack.File{
			Name:    "outputs.tpl",
			Content: []byte("Job: {{ .testpack.job_name }}"),
		},
	}

	// Create V1 parsed variables
	pv := &parser.ParsedVariables{}
	v1Vars := map[string]map[string]*variables.Variable{
		"testpack": {
			"job_name": {
				Name:  "job_name",
				Value: cty.StringVal("my-job"),
			},
		},
	}
	err := pv.LoadV1Result(v1Vars)
	require.NoError(t, err)

	// Create renderer with proper template setup
	r := &Renderer{
		pack: p,
		pv:   pv,
	}

	// Initialize template with funcMap
	r.tpl = template.New("test").Funcs(funcMap(r))

	// Render output - should work with V1 parser
	output, err := r.RenderOutput()
	require.NoError(t, err)
	require.Equal(t, "Job: my-job", output)
}

// TestRenderOutput_V2Parser_Success tests that output template rendering
// continues to work with V2 parser (no regression).
func TestRenderOutput_V2Parser_Success(t *testing.T) {
	// Create a pack with simple output template (no variables)
	p := &pack.Pack{
		Metadata: &pack.Metadata{
			Pack: &pack.MetadataPack{
				Name: "testpack",
			},
			App: &pack.MetadataApp{},
		},
		OutputTemplateFile: &pack.File{
			Name:    "outputs.tpl",
			Content: []byte("Deployment successful!"),
		},
		Path: "/test/path",
	}

	// Create V2 parsed variables (empty is fine)
	pv := &parser.ParsedVariables{}
	v2Vars := map[pack.ID]map[variables.ID]*variables.Variable{}
	err := pv.LoadV2Result(v2Vars)
	require.NoError(t, err)

	// Create renderer
	r := &Renderer{
		pack: p,
		pv:   pv,
	}
	r.tpl = template.New("test").Funcs(funcMap(r))

	// Render output - should work without error
	output, err := r.RenderOutput()
	require.NoError(t, err)
	require.Equal(t, "Deployment successful!", output)
}

// TestRenderOutput_NoOutputTemplate tests that when pack has no output
// template, RenderOutput returns empty string without error.
func TestRenderOutput_NoOutputTemplate(t *testing.T) {
	// Create a pack without output template
	p := &pack.Pack{
		Metadata: &pack.Metadata{
			Pack: &pack.MetadataPack{
				Name: "testpack",
			},
			App: &pack.MetadataApp{},
		},
		OutputTemplateFile: nil, // No output template
	}

	// Create V2 parsed variables
	pv := &parser.ParsedVariables{}
	v2Vars := map[pack.ID]map[variables.ID]*variables.Variable{}
	err := pv.LoadV2Result(v2Vars)
	require.NoError(t, err)

	// Create renderer
	r := &Renderer{
		pack: p,
		pv:   pv,
		tpl:  template.New("test"),
	}

	// Render output - should return empty string
	output, err := r.RenderOutput()
	require.NoError(t, err)
	require.Equal(t, "", output)
}

// TestRenderOutput_EmptyOutputTemplate tests that empty output template
// returns empty string without error.
func TestRenderOutput_EmptyOutputTemplate(t *testing.T) {
	// Create a pack with empty output template
	p := &pack.Pack{
		Metadata: &pack.Metadata{
			Pack: &pack.MetadataPack{
				Name: "testpack",
			},
			App: &pack.MetadataApp{},
		},
		OutputTemplateFile: &pack.File{
			Name:    "outputs.tpl",
			Content: []byte(""), // Empty content
		},
	}

	// Create V2 parsed variables
	pv := &parser.ParsedVariables{}
	v2Vars := map[pack.ID]map[variables.ID]*variables.Variable{}
	err := pv.LoadV2Result(v2Vars)
	require.NoError(t, err)

	// Create renderer
	r := &Renderer{
		pack: p,
		pv:   pv,
	}
	r.tpl = template.New("test").Funcs(funcMap(r))

	// Render output - should return empty string
	output, err := r.RenderOutput()
	require.NoError(t, err)
	require.Equal(t, "", output)
}

// TestRenderOutput_InvalidTemplateSyntax tests that malformed template
// syntax returns an error.
func TestRenderOutput_InvalidTemplateSyntax(t *testing.T) {
	// Create a pack with invalid template syntax
	p := &pack.Pack{
		Metadata: &pack.Metadata{
			Pack: &pack.MetadataPack{
				Name: "testpack",
			},
			App: &pack.MetadataApp{},
		},
		OutputTemplateFile: &pack.File{
			Name:    "outputs.tpl",
			Content: []byte("{{ .my.job_name "), // Missing closing braces
		},
	}

	// Create V2 parsed variables
	pv := &parser.ParsedVariables{}
	v2Vars := map[pack.ID]map[variables.ID]*variables.Variable{}
	err := pv.LoadV2Result(v2Vars)
	require.NoError(t, err)

	// Create renderer
	r := &Renderer{
		pack: p,
		pv:   pv,
	}
	r.tpl = template.New("test").Funcs(funcMap(r))

	// Render output - should return error
	output, err := r.RenderOutput()
	require.Error(t, err)
	require.Equal(t, "", output)
}
