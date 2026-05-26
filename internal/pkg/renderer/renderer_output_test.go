// Copyright IBM Corp. 2023, 2026
// SPDX-License-Identifier: MPL-2.0

package renderer

import (
	"testing"

	"github.com/hashicorp/nomad-pack/internal/pkg/variable/parser"
	"github.com/hashicorp/nomad-pack/sdk/pack"
	"github.com/hashicorp/nomad-pack/sdk/pack/variables"
	"github.com/shoenig/test/must"
	"github.com/zclconf/go-cty/cty"
)

// TestRenderOutput_V1Parser_Success is the primary regression test.
// Without the fix, this test fails with "nil pointer evaluating parser.PackContextable.job_name"
// because RenderOutput() uses V2 context even when --parser-v1 flag is set.
func TestRenderOutput_V1Parser_Success(t *testing.T) {
	p := &pack.Pack{
		Metadata: &pack.Metadata{
			Pack: &pack.MetadataPack{Name: "simple_raw_exec"},
			App:  &pack.MetadataApp{},
		},
		TemplateFiles: []*pack.File{
			{
				Name:    "simple_raw_exec/templates/job.nomad.tpl",
				Content: []byte(`job "[[ .my.job_name ]]" { type = "batch" }`),
			},
		},
		OutputTemplateFile: &pack.File{
			Name:    "outputs.tpl",
			Content: []byte("[[ .simple_raw_exec.job_name ]] deployed."),
		},
	}

	pv := &parser.ParsedVariables{}
	err := pv.LoadV1Result(map[string]map[string]*variables.Variable{
		"simple_raw_exec": {
			"job_name": {Name: "job_name", Value: cty.StringVal("my-job")},
		},
	})
	must.NoError(t, err)

	r := &Renderer{}
	_, err = r.Render(p, pv)
	must.NoError(t, err)

	output, err := r.RenderOutput()
	must.NoError(t, err)
	must.Eq(t, "my-job deployed.", output)
}

func TestRenderOutput_V1Parser_WithVariable(t *testing.T) {
	p := &pack.Pack{
		Metadata: &pack.Metadata{
			Pack: &pack.MetadataPack{Name: "testpack"},
			App:  &pack.MetadataApp{},
		},
		TemplateFiles: []*pack.File{
			{
				Name:    "testpack/templates/job.nomad.tpl",
				Content: []byte(`job "[[ .my.job_name ]]" { type = "batch" }`),
			},
		},
		OutputTemplateFile: &pack.File{
			Name:    "outputs.tpl",
			Content: []byte("Job [[ .testpack.job_name ]] is ready."),
		},
	}

	pv := &parser.ParsedVariables{}
	err := pv.LoadV1Result(map[string]map[string]*variables.Variable{
		"testpack": {
			"job_name": {Name: "job_name", Value: cty.StringVal("test-job")},
		},
	})
	must.NoError(t, err)

	r := &Renderer{}
	_, err = r.Render(p, pv)
	must.NoError(t, err)

	output, err := r.RenderOutput()
	must.NoError(t, err)
	must.Eq(t, "Job test-job is ready.", output)
}

func TestRenderOutput_V1Parser_MultipleVariables(t *testing.T) {
	p := &pack.Pack{
		Metadata: &pack.Metadata{
			Pack: &pack.MetadataPack{Name: "mypack"},
			App:  &pack.MetadataApp{},
		},
		TemplateFiles: []*pack.File{
			{
				Name:    "mypack/templates/job.nomad.tpl",
				Content: []byte(`job "[[ .my.job_name ]]" { type = "batch" }`),
			},
		},
		OutputTemplateFile: &pack.File{
			Name:    "outputs.tpl",
			Content: []byte("Deployed [[ .mypack.job_name ]] to [[ .mypack.region ]]."),
		},
	}

	pv := &parser.ParsedVariables{}
	err := pv.LoadV1Result(map[string]map[string]*variables.Variable{
		"mypack": {
			"job_name": {Name: "job_name", Value: cty.StringVal("my-app")},
			"region":   {Name: "region", Value: cty.StringVal("us-east-1")},
		},
	})
	must.NoError(t, err)

	r := &Renderer{}
	_, err = r.Render(p, pv)
	must.NoError(t, err)

	output, err := r.RenderOutput()
	must.NoError(t, err)
	must.Eq(t, "Deployed my-app to us-east-1.", output)
}

func TestRenderOutput_V1Parser_ComplexTemplate(t *testing.T) {
	p := &pack.Pack{
		Metadata: &pack.Metadata{
			Pack: &pack.MetadataPack{Name: "webapp"},
			App:  &pack.MetadataApp{},
		},
		TemplateFiles: []*pack.File{
			{
				Name:    "webapp/templates/job.nomad.tpl",
				Content: []byte(`job "[[ .my.job_name ]]" { type = "service" }`),
			},
		},
		OutputTemplateFile: &pack.File{
			Name: "outputs.tpl",
			Content: []byte(`Deployment Summary:
- Job: [[ .webapp.job_name ]]
- Count: [[ .webapp.count ]]
- Status: Ready`),
		},
	}

	pv := &parser.ParsedVariables{}
	err := pv.LoadV1Result(map[string]map[string]*variables.Variable{
		"webapp": {
			"job_name": {Name: "job_name", Value: cty.StringVal("web-server")},
			"count":    {Name: "count", Value: cty.NumberIntVal(3)},
		},
	})
	must.NoError(t, err)

	r := &Renderer{}
	_, err = r.Render(p, pv)
	must.NoError(t, err)

	output, err := r.RenderOutput()
	must.NoError(t, err)
	must.StrContains(t, output, "web-server")
	must.StrContains(t, output, "3")
}

func TestRenderOutput_V2Parser_Success(t *testing.T) {
	p := &pack.Pack{
		Metadata: &pack.Metadata{
			Pack: &pack.MetadataPack{Name: "testpack"},
			App:  &pack.MetadataApp{},
		},
		TemplateFiles: []*pack.File{
			{
				Name:    "testpack/templates/job.nomad.tpl",
				Content: []byte(`job "test-job" { type = "batch" }`),
			},
		},
		OutputTemplateFile: &pack.File{
			Name:    "outputs.tpl",
			Content: []byte("Deployment successful!"),
		},
		Path: "/test/path",
	}

	pv := &parser.ParsedVariables{}
	v2Vars := map[pack.ID]map[variables.ID]*variables.Variable{}
	err := pv.LoadV2Result(v2Vars)
	must.NoError(t, err)

	r := &Renderer{}
	_, err = r.Render(p, pv)
	must.NoError(t, err)

	output, err := r.RenderOutput()
	must.NoError(t, err)
	must.Eq(t, "Deployment successful!", output)
}

func TestRenderOutput_V2Parser_InvalidTemplateSyntax(t *testing.T) {
	p := &pack.Pack{
		Metadata: &pack.Metadata{
			Pack: &pack.MetadataPack{Name: "testpack"},
			App:  &pack.MetadataApp{},
		},
		TemplateFiles: []*pack.File{
			{
				Name:    "testpack/templates/job.nomad.tpl",
				Content: []byte(`job "test" { type = "batch" }`),
			},
		},
		OutputTemplateFile: &pack.File{
			Name:    "outputs.tpl",
			Content: []byte("[[ .my.job_name "),
		},
	}

	pv := &parser.ParsedVariables{}
	v2Vars := map[pack.ID]map[variables.ID]*variables.Variable{}
	err := pv.LoadV2Result(v2Vars)
	must.NoError(t, err)

	r := &Renderer{}
	_, err = r.Render(p, pv)
	must.NoError(t, err)

	output, err := r.RenderOutput()
	must.Error(t, err)
	must.Eq(t, "", output)
}
