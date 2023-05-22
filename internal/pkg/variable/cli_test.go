// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package variable

import (
	"fmt"
	"os"
	"path"
	"testing"

	"github.com/hashicorp/hcl/v2"
	"github.com/shoenig/test/must"
	"github.com/spf13/afero"
	"github.com/zclconf/go-cty/cty"
)

func TestParser_parseCLIVariable(t *testing.T) {
	testCases := []struct {
		inputParser     *Parser
		inputName       string
		inputRawVal     string
		expectedError   bool
		expectedCLIVars map[string][]*Variable
		expectedEnvVars map[string][]*Variable
		name            string
	}{
		{
			inputParser: &Parser{
				fs:  afero.Afero{Fs: afero.OsFs{}},
				cfg: &ParserConfig{ParentName: "example"},
				rootVars: map[string]map[string]*Variable{
					"example": {
						"region": &Variable{
							Name:      "region",
							Type:      cty.String,
							Value:     cty.StringVal("vlc"),
							DeclRange: hcl.Range{Filename: "<value for var.region from arguments>"},
						},
					},
				},
				cliOverrideVars: make(map[string][]*Variable),
				envOverrideVars: make(map[string][]*Variable),
			},
			inputName:     "region",
			inputRawVal:   "vlc",
			expectedError: false,
			expectedCLIVars: map[string][]*Variable{
				"example": {
					{
						Name:      "region",
						Type:      cty.String,
						Value:     cty.StringVal("vlc"),
						DeclRange: hcl.Range{Filename: "<value for var.region from arguments>"},
					},
				},
			},
			expectedEnvVars: make(map[string][]*Variable),
			name:            "non-namespaced variable",
		},
		{
			inputParser: &Parser{
				fs:  afero.Afero{Fs: afero.OsFs{}},
				cfg: &ParserConfig{ParentName: "example"},
				rootVars: map[string]map[string]*Variable{
					"example": {
						"region": &Variable{
							Name:      "region",
							Type:      cty.String,
							Value:     cty.StringVal("vlc"),
							DeclRange: hcl.Range{Filename: "<value for var.region from arguments>"},
						},
					},
				},
				cliOverrideVars: make(map[string][]*Variable),
			},
			inputName:     "example.region",
			inputRawVal:   "vlc",
			expectedError: false,
			expectedCLIVars: map[string][]*Variable{
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
			inputParser: &Parser{
				fs:              afero.Afero{Fs: afero.OsFs{}},
				cfg:             &ParserConfig{ParentName: "example"},
				rootVars:        map[string]map[string]*Variable{},
				cliOverrideVars: make(map[string][]*Variable),
			},
			inputName:       "example.region",
			inputRawVal:     "vlc",
			expectedError:   true,
			expectedCLIVars: map[string][]*Variable{},
			name:            "root variable absent",
		},
		{
			inputParser: &Parser{
				fs:  afero.Afero{Fs: afero.OsFs{}},
				cfg: &ParserConfig{ParentName: "example"},
				rootVars: map[string]map[string]*Variable{
					"example": {
						"region": &Variable{
							Name: "region",
							Type: cty.DynamicPseudoType,
							Value: cty.MapVal(map[string]cty.Value{
								"region": cty.StringVal("dc1"),
							}),
							DeclRange: hcl.Range{Filename: "<value for var.region from arguments>"},
						},
					},
				},
				cliOverrideVars: make(map[string][]*Variable),
			},
			inputName:       "example.region",
			inputRawVal:     "vlc",
			expectedError:   true,
			expectedCLIVars: map[string][]*Variable{},
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

func TestParser_parseHeredocAtEOF(t *testing.T) {
	inputParser := &Parser{
		fs:              afero.Afero{Fs: afero.OsFs{}},
		cfg:             &ParserConfig{ParentName: "example"},
		rootVars:        map[string]map[string]*Variable{},
		cliOverrideVars: make(map[string][]*Variable),
	}
	fixturePath := Fixture("variable_test/heredoc.vars.hcl")
	b, diags := inputParser.loadOverrideFile(fixturePath)
	must.NotNil(t, b)
	must.SliceEmpty(t, diags)
}

func TestParser_VariableOverrides(t *testing.T) {
	testcases := []struct {
		Name   string
		Parser *Parser
		Expect string
	}{
		{
			Name:   "no override",
			Parser: NewTestInputParser(),
			Expect: "root",
		},
		{
			Name:   "env override",
			Parser: NewTestInputParser(WithEnvVar("input", "env")),
			Expect: "env",
		},
		{
			Name:   "file override",
			Parser: NewTestInputParser(WithFileVar("input", "file")),
			Expect: "file",
		},
		{
			Name:   "flag override",
			Parser: NewTestInputParser(WithCliVar("input", "flag")),
			Expect: "flag",
		},
		{
			Name: "file opaques env",
			Parser: NewTestInputParser(
				WithEnvVar("input", "env"),
				WithFileVar("input", "file"),
			),
			Expect: "file",
		},
		{
			Name: "flag opaques env",
			Parser: NewTestInputParser(
				WithEnvVar("input", "env"),
				WithCliVar("input", "flag"),
			),
			Expect: "flag",
		},
		{
			Name: "flag opaques file",
			Parser: NewTestInputParser(
				WithFileVar("input", "file"),
				WithCliVar("input", "flag"),
			),
			Expect: "flag",
		},
		{
			Name: "flag opaques env and file",
			Parser: NewTestInputParser(
				WithEnvVar("input", "env"),
				WithFileVar("input", "file"),
				WithCliVar("input", "flag"),
			),
			Expect: "flag",
		},
	}

	for _, tc := range testcases {
		t.Run(tc.Name, func(t *testing.T) {
			pv, diags := tc.Parser.Parse()
			must.NotNil(t, pv)
			must.SliceEmpty(t, diags)

			must.Eq(t, tc.Expect, pv.Vars["example"]["input"].Value.AsString())
		})
	}
}

func Fixture(fPath string) string {
	// FIXME: Find the fixture folder in a less janky way
	cwd, _ := os.Getwd()
	return path.Join(cwd, "../../../fixtures/", fPath)
}

type TestParserOption func(*Parser)

func WithEnvVar(key, value string) TestParserOption {
	return func(p *Parser) {
		p.envOverrideVars["example"] = append(p.envOverrideVars["example"], NewStringVariable(key, value, "env"))
	}
}

func WithCliVar(key, value string) TestParserOption {
	return func(p *Parser) {
		p.cliOverrideVars["example"] = append(p.cliOverrideVars["example"], NewStringVariable(key, value, "cli"))
	}
}

func WithFileVar(key, value string) TestParserOption {
	return func(p *Parser) {
		p.cliOverrideVars["example"] = append(p.cliOverrideVars["example"], NewStringVariable(key, value, "file"))
	}
}

func NewTestInputParser(opts ...TestParserOption) *Parser {

	p := &Parser{
		fs:  afero.Afero{Fs: afero.OsFs{}},
		cfg: &ParserConfig{ParentName: "example"},
		rootVars: map[string]map[string]*Variable{
			"example": {
				"input": &Variable{
					Name:      "input",
					Type:      cty.String,
					Value:     cty.StringVal("root"),
					DeclRange: hcl.Range{Filename: "<value for var.input from rootVars>"},
				},
			},
		},
		envOverrideVars:  make(map[string][]*Variable),
		fileOverrideVars: make(map[string][]*Variable),
		cliOverrideVars:  make(map[string][]*Variable),
	}

	// Loop through each option
	for _, opt := range opts {
		opt(p)
	}

	return p
}

func NewStringVariable(key, value, kind string) *Variable {
	return &Variable{
		Name:      key,
		Type:      cty.String,
		Value:     cty.StringVal(value),
		DeclRange: hcl.Range{Filename: fmt.Sprintf("<value for var.%s from %s>", key, kind)},
	}
}
