// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package parser

import (
	"fmt"
	"testing"

	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/nomad-pack/internal/pkg/testfixture"
	"github.com/hashicorp/nomad-pack/internal/pkg/variable/parser/config"
	"github.com/hashicorp/nomad-pack/sdk/pack/variables"
	"github.com/shoenig/test/must"
	"github.com/spf13/afero"
	"github.com/zclconf/go-cty/cty"
)

func TestParserV1_parseCLIVariable(t *testing.T) {
	testCases := []struct {
		inputParser     *ParserV1
		inputName       string
		inputRawVal     string
		expectedError   bool
		expectedCLIVars map[string][]*variables.Variable
		expectedEnvVars map[string][]*variables.Variable
		name            string
	}{
		{
			inputParser: &ParserV1{
				fs:  afero.Afero{Fs: afero.OsFs{}},
				cfg: &config.ParserConfig{ParentName: "example"},
				rootVars: map[string]map[string]*variables.Variable{
					"example": {
						"region": &variables.Variable{
							Name:      "region",
							Type:      cty.String,
							Value:     cty.StringVal("vlc"),
							DeclRange: hcl.Range{Filename: "<value for var.region from arguments>"},
						},
					},
				},
				cliOverrideVars: make(map[string][]*variables.Variable),
				envOverrideVars: make(map[string][]*variables.Variable),
			},
			inputName:     "region",
			inputRawVal:   "vlc",
			expectedError: false,
			expectedCLIVars: map[string][]*variables.Variable{
				"example": {
					{
						Name:      "region",
						Type:      cty.String,
						Value:     cty.StringVal("vlc"),
						DeclRange: hcl.Range{Filename: "<value for var.region from arguments>"},
					},
				},
			},
			expectedEnvVars: make(map[string][]*variables.Variable),
			name:            "non-namespaced variable",
		},
		{
			inputParser: &ParserV1{
				fs:  afero.Afero{Fs: afero.OsFs{}},
				cfg: &config.ParserConfig{ParentName: "example"},
				rootVars: map[string]map[string]*variables.Variable{
					"example": {
						"region": &variables.Variable{
							Name:      "region",
							Type:      cty.String,
							Value:     cty.StringVal("vlc"),
							DeclRange: hcl.Range{Filename: "<value for var.region from arguments>"},
						},
					},
				},
				cliOverrideVars: make(map[string][]*variables.Variable),
			},
			inputName:     "example.region",
			inputRawVal:   "vlc",
			expectedError: false,
			expectedCLIVars: map[string][]*variables.Variable{
				"example": {
					{
						Name:      "region",
						Type:      cty.String,
						Value:     cty.StringVal("vlc"),
						DeclRange: hcl.Range{Filename: "<value for var.example.region from arguments>"},
					},
				},
			},
			name: "namespaced variable",
		},
		{
			inputParser: &ParserV1{
				fs:              afero.Afero{Fs: afero.OsFs{}},
				cfg:             &config.ParserConfig{ParentName: "example"},
				rootVars:        map[string]map[string]*variables.Variable{},
				cliOverrideVars: make(map[string][]*variables.Variable),
			},
			inputName:       "example.region",
			inputRawVal:     "vlc",
			expectedError:   true,
			expectedCLIVars: map[string][]*variables.Variable{},
			name:            "root variable absent",
		},
		{
			inputParser: &ParserV1{
				fs:  afero.Afero{Fs: afero.OsFs{}},
				cfg: &config.ParserConfig{ParentName: "example"},
				rootVars: map[string]map[string]*variables.Variable{
					"example": {
						"region": &variables.Variable{
							Name: "region",
							Type: cty.DynamicPseudoType,
							Value: cty.MapVal(map[string]cty.Value{
								"region": cty.StringVal("dc1"),
							}),
							DeclRange: hcl.Range{Filename: "<value for var.region from arguments>"},
						},
					},
				},
				cliOverrideVars: make(map[string][]*variables.Variable),
			},
			inputName:       "example.region",
			inputRawVal:     "vlc",
			expectedError:   true,
			expectedCLIVars: map[string][]*variables.Variable{},
			name:            "unconvertable variable",
		},
	}

	for _, tc := range testCases {
		actualErr := tc.inputParser.parseCLIVariable(tc.inputName, tc.inputRawVal)
		if tc.expectedError {
			must.NotNil(t, actualErr, must.Sprint(tc.name))
		} else {
			must.Nil(t, actualErr, must.Sprint(tc.name))
			must.Eq(t, tc.expectedCLIVars, tc.inputParser.cliOverrideVars, must.Sprint(tc.name))
		}
	}
}

func TestParserV1_parseHeredocAtEOF(t *testing.T) {
	inputParser := &ParserV1{
		fs:              afero.Afero{Fs: afero.OsFs{}},
		cfg:             &config.ParserConfig{ParentName: "example"},
		rootVars:        map[string]map[string]*variables.Variable{},
		cliOverrideVars: make(map[string][]*variables.Variable),
	}
	fixturePath := testfixture.AbsPath(t, "v1/variable_test/heredoc.vars.hcl")
	b, diags := inputParser.loadOverrideFile(fixturePath)
	must.NotNil(t, b)
	must.SliceEmpty(t, diags)
}

func TestParserV1_VariableOverrides(t *testing.T) {
	testcases := []struct {
		Name   string
		Parser *ParserV1
		Expect string
	}{
		{
			Name:   "no override",
			Parser: NewTestInputParserV1(),
			Expect: "root",
		},
		{
			Name:   "env override",
			Parser: NewTestInputParserV1(WithEnvVarV1("input", "env")),
			Expect: "env",
		},
		{
			Name:   "file override",
			Parser: NewTestInputParserV1(WithFileVarV1("input", "file")),
			Expect: "file",
		},
		{
			Name:   "flag override",
			Parser: NewTestInputParserV1(WithCliVarV1("input", "flag")),
			Expect: "flag",
		},
		{
			Name: "file opaques env",
			Parser: NewTestInputParserV1(
				WithEnvVarV1("input", "env"),
				WithFileVarV1("input", "file"),
			),
			Expect: "file",
		},
		{
			Name: "flag opaques env",
			Parser: NewTestInputParserV1(
				WithEnvVarV1("input", "env"),
				WithCliVarV1("input", "flag"),
			),
			Expect: "flag",
		},
		{
			Name: "flag opaques file",
			Parser: NewTestInputParserV1(
				WithFileVarV1("input", "file"),
				WithCliVarV1("input", "flag"),
			),
			Expect: "flag",
		},
		{
			Name: "flag opaques env and file",
			Parser: NewTestInputParserV1(
				WithEnvVarV1("input", "env"),
				WithFileVarV1("input", "file"),
				WithCliVarV1("input", "flag"),
			),
			Expect: "flag",
		},
	}

	for _, tc := range testcases {
		t.Run(tc.Name, func(t *testing.T) {
			pv, diags := tc.Parser.Parse()
			must.NotNil(t, pv)
			must.SliceEmpty(t, diags)

			must.Eq(t, tc.Expect, pv.v1Vars["example"]["input"].Value.AsString())
		})
	}
}

func TestParserV1_parseNestedPack(t *testing.T) {
	fixturePath := testfixture.AbsPath(t, "v1/test_registry/packs/my_alias_test")
	pm := newTestPackManager(t, fixturePath, true)

	pvs := pm.ProcessVariables()
	must.NotNil(t, pvs)
	must.MapNotEmpty(t, pvs.v1Vars)
}

type testParserV1Option func(*ParserV1)

func WithEnvVarV1(key, value string) testParserV1Option {
	return func(p *ParserV1) {
		p.envOverrideVars["example"] = append(p.envOverrideVars["example"], NewStringVariableV1(key, value, "env"))
	}
}

func WithCliVarV1(key, value string) testParserV1Option {
	return func(p *ParserV1) {
		p.cliOverrideVars["example"] = append(p.cliOverrideVars["example"], NewStringVariableV1(key, value, "cli"))
	}
}

func WithFileVarV1(key, value string) testParserV1Option {
	return func(p *ParserV1) {
		p.cliOverrideVars["example"] = append(p.cliOverrideVars["example"], NewStringVariableV1(key, value, "file"))
	}
}

func NewTestInputParserV1(opts ...testParserV1Option) *ParserV1 {

	p := &ParserV1{
		fs:  afero.Afero{Fs: afero.OsFs{}},
		cfg: &config.ParserConfig{ParentName: "example"},
		rootVars: map[string]map[string]*variables.Variable{
			"example": {
				"input": &variables.Variable{
					Name:      "input",
					Type:      cty.String,
					Value:     cty.StringVal("root"),
					DeclRange: hcl.Range{Filename: "<value for var.input from rootVars>"},
				},
			},
		},
		envOverrideVars:  make(map[string][]*variables.Variable),
		fileOverrideVars: make(map[string][]*variables.Variable),
		cliOverrideVars:  make(map[string][]*variables.Variable),
	}

	// Loop through each option
	for _, opt := range opts {
		opt(p)
	}

	return p
}

func NewStringVariableV1(key, value, kind string) *variables.Variable {
	return &variables.Variable{
		Name:      variables.ID(key),
		Type:      cty.String,
		Value:     cty.StringVal(value),
		DeclRange: hcl.Range{Filename: fmt.Sprintf("<value for var.%s from %s>", key, kind)},
	}
}
