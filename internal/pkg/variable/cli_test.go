// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package variable

import (
	"fmt"
	"os"
	"path"
	"strings"
	"testing"

	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/nomad-pack/internal/pkg/loader"
	"github.com/hashicorp/nomad-pack/sdk/pack"
	"github.com/shoenig/test/must"
	"github.com/spf13/afero"
	"github.com/zclconf/go-cty/cty"
)

func TestParser_parseFlagVariable(t *testing.T) {
	testCases := []struct {
		inputParser      *Parser
		inputName        string
		inputRawVal      string
		expectedError    bool
		expectedFlagVars map[PackID][]*Variable
		expectedEnvVars  map[PackID][]*Variable
		name             string
	}{
		{
			name: "non-namespaced variable",
			inputParser: &Parser{
				fs:  afero.Afero{Fs: afero.OsFs{}},
				cfg: &ParserConfig{ParentPackID: "example"},
				rootVars: map[PackID]map[VariableID]*Variable{
					"example": {
						"region": &Variable{
							Name:      "region",
							Type:      cty.String,
							Value:     cty.StringVal("vlc"),
							DeclRange: hcl.Range{Filename: "<value for var region from arguments>"},
						},
					},
				},
				flagOverrideVars: make(map[PackID][]*Variable),
				envOverrideVars:  make(map[PackID][]*Variable),
			},
			inputName:        "region",
			inputRawVal:      "vlc",
			expectedError:    true,
			expectedFlagVars: map[PackID][]*Variable{},
			expectedEnvVars:  make(map[PackID][]*Variable),
		},
		{
			name: "namespaced variable",
			inputParser: &Parser{
				fs:  afero.Afero{Fs: afero.OsFs{}},
				cfg: &ParserConfig{ParentPackID: "example"},
				rootVars: map[PackID]map[VariableID]*Variable{
					"example": {
						"region": &Variable{
							Name:      "region",
							Type:      cty.String,
							Value:     cty.StringVal("vlc"),
							DeclRange: hcl.Range{Filename: "<value for var region from arguments>"},
						},
					},
				},
				flagOverrideVars: make(map[PackID][]*Variable),
			},
			inputName:     "example.region",
			inputRawVal:   "vlc",
			expectedError: false,
			expectedFlagVars: map[PackID][]*Variable{
				"example": {
					{
						Name:      "region",
						Type:      cty.String,
						Value:     cty.StringVal("vlc"),
						DeclRange: hcl.Range{Filename: "<value for var example.region from arguments>"},
					},
				},
			},
		},
		{
			inputParser: &Parser{
				fs:               afero.Afero{Fs: afero.OsFs{}},
				cfg:              &ParserConfig{ParentPackID: "example"},
				rootVars:         map[PackID]map[VariableID]*Variable{},
				flagOverrideVars: make(map[PackID][]*Variable),
			},
			inputName:        "example.region",
			inputRawVal:      "vlc",
			expectedError:    true,
			expectedFlagVars: map[PackID][]*Variable{},
			name:             "root variable absent",
		},
		{
			name: "unconvertable variable",
			inputParser: &Parser{
				fs:  afero.Afero{Fs: afero.OsFs{}},
				cfg: &ParserConfig{ParentPackID: "example"},
				rootVars: map[PackID]map[VariableID]*Variable{
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
				flagOverrideVars: make(map[PackID][]*Variable),
			},
			inputName:        "example.region",
			inputRawVal:      "vlc",
			expectedError:    true,
			expectedFlagVars: map[PackID][]*Variable{},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			tc := tc
			actualErr := tc.inputParser.parseFlagVariable(tc.inputName, tc.inputRawVal)

			if tc.expectedError {
				must.NotNil(t, actualErr)
				return
			}
			must.Nil(t, actualErr, must.Sprintf("actualErr: %v", actualErr))
			must.MapEq(t, tc.expectedFlagVars, tc.inputParser.flagOverrideVars)
		})
	}
}

func TestParser_parseEnvVariable(t *testing.T) {
	type testCase struct {
		inputParser      *Parser
		envKey           string
		envValue         string
		expectedError    bool
		expectedFlagVars map[PackID][]*Variable
		expectedEnvVars  map[PackID][]*Variable
		name             string
	}

	withDefault := func(e, d string) string {
		t.Helper()
		if e == "" {
			return d
		}
		return e
	}

	getEnvKey := func(tc testCase) string {
		t.Helper()
		return withDefault(tc.envKey, "NOMAD_PACK_VAR_example.region")
	}

	getEnvValue := func(tc testCase) string {
		t.Helper()
		return withDefault(tc.envValue, "vlc")
	}

	setTestEnvKeyForVar := func(t *testing.T, tc testCase) string {
		t.Helper()
		var k string = getEnvKey(tc)
		var v string = getEnvValue(tc)
		t.Logf("setting %s to %s", k, v)
		t.Setenv(k, v)
		return strings.TrimPrefix(k, VarEnvPrefix)
	}

	testCases := []testCase{
		{
			name:   "non-namespaced variable",
			envKey: "NOMAD_PACK_VAR_region",
			inputParser: &Parser{
				fs:  afero.Afero{Fs: afero.OsFs{}},
				cfg: &ParserConfig{ParentPackID: "example"},
				rootVars: map[PackID]map[VariableID]*Variable{
					"example": {
						"region": &Variable{
							Name:      "region",
							Type:      cty.String,
							Value:     cty.StringVal("vlc"),
							DeclRange: hcl.Range{Filename: "<value for var region from arguments>"},
						},
					},
				},
				flagOverrideVars: make(map[PackID][]*Variable),
				envOverrideVars:  make(map[PackID][]*Variable),
			},
			expectedError:    true,
			expectedFlagVars: map[PackID][]*Variable{},
			expectedEnvVars:  make(map[PackID][]*Variable),
		},
		{
			name: "namespaced variable",
			inputParser: &Parser{
				fs:  afero.Afero{Fs: afero.OsFs{}},
				cfg: &ParserConfig{ParentPackID: "example"},
				rootVars: map[PackID]map[VariableID]*Variable{
					"example": {
						"region": &Variable{
							Name:      "region",
							Type:      cty.String,
							Value:     cty.StringVal("vlc"),
							DeclRange: hcl.Range{Filename: "<value for var example.region from arguments>"},
						},
					},
				},
				flagOverrideVars: make(map[PackID][]*Variable),
				envOverrideVars:  make(map[PackID][]*Variable),
			},
			expectedError:    false,
			expectedFlagVars: map[PackID][]*Variable{},
			expectedEnvVars: map[PackID][]*Variable{
				"example": {
					{
						Name:      "region",
						Type:      cty.String,
						Value:     cty.StringVal("vlc"),
						DeclRange: hcl.Range{Filename: "<value for var example.region from environment>"},
					},
				},
			},
		},
		{
			name: "root variable absent",
			inputParser: &Parser{
				fs:               afero.Afero{Fs: afero.OsFs{}},
				cfg:              &ParserConfig{ParentPackID: "example"},
				rootVars:         map[PackID]map[VariableID]*Variable{},
				flagOverrideVars: make(map[PackID][]*Variable),
				envOverrideVars:  make(map[PackID][]*Variable),
			},
			expectedError:    true,
			expectedFlagVars: map[PackID][]*Variable{},
			expectedEnvVars:  map[PackID][]*Variable{},
		},
		{
			name:     "unconvertable variable",
			envKey:   "NOMAD_PACK_VAR_example.region",
			envValue: `{region: "dc1}`,
			inputParser: &Parser{
				fs:  afero.Afero{Fs: afero.OsFs{}},
				cfg: &ParserConfig{ParentPackID: "example"},
				rootVars: map[PackID]map[VariableID]*Variable{
					"example": {
						"region": &Variable{
							Name: "region",
							Type: cty.DynamicPseudoType,
							Value: cty.MapVal(map[string]cty.Value{
								"region": cty.StringVal("dc1"),
							}),
							DeclRange: hcl.Range{Filename: "<value for var example.region from arguments>"},
						},
					},
				},
				flagOverrideVars: make(map[PackID][]*Variable),
				envOverrideVars:  make(map[PackID][]*Variable),
			},
			expectedError:    true,
			expectedFlagVars: map[PackID][]*Variable{},
			expectedEnvVars:  map[PackID][]*Variable{},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			mapKey := setTestEnvKeyForVar(t, tc)

			em := GetVarsFromEnv()
			must.MapLen(t, 1, em)
			must.MapContainsKey(t, em, mapKey)

			tV := em[mapKey]
			actualErr := tc.inputParser.parseEnvVariable(getEnvKey(tc), tV)

			if tc.expectedError {
				t.Logf(actualErr.Error())
				must.NotNil(t, actualErr)
				return
			}
			must.Nil(t, actualErr, must.Sprintf("actualErr: %v", actualErr))
			must.MapEq(t, tc.expectedFlagVars, tc.inputParser.flagOverrideVars)
			must.MapEq(t, tc.expectedEnvVars, tc.inputParser.envOverrideVars)
		})
	}
}

func TestParser_parseHeredocAtEOF(t *testing.T) {
	inputParser := &Parser{
		fs: afero.Afero{Fs: afero.OsFs{}},
		cfg: &ParserConfig{
			RootVariableFiles: map[pack.PackID]*pack.File{},
		},
		rootVars:         map[PackID]map[VariableID]*Variable{},
		fileOverrideVars: make(map[PackID][]*Variable),
	}

	fixtureRoot := Fixture("variable_test")
	p, err := loader.Load(fixtureRoot + "/variable_test")
	must.NoError(t, err)
	must.NotNil(t, p)

	inputParser.cfg.RootVariableFiles = p.RootVariableFiles()

	_, diags := inputParser.newParseOverridesFile(fixtureRoot + "/heredoc.vars.hcl")
	must.False(t, diags.HasErrors(), must.Sprintf("diags: %v", diags))
	must.Len(t, 1, inputParser.fileOverrideVars["variable_test_pack"])
	must.Eq(t, "heredoc\n", inputParser.fileOverrideVars["variable_test_pack"][0].Value.AsString())
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

type testParserOption func(*Parser)

func WithEnvVar(key, value string) testParserOption {
	return func(p *Parser) {
		p.envOverrideVars["example"] = append(p.envOverrideVars["example"], NewStringVariable(key, value, "env"))
	}
}

func WithCliVar(key, value string) testParserOption {
	return func(p *Parser) {
		p.flagOverrideVars["example"] = append(p.flagOverrideVars["example"], NewStringVariable(key, value, "cli"))
	}
}

func WithFileVar(key, value string) testParserOption {
	return func(p *Parser) {
		p.flagOverrideVars["example"] = append(p.flagOverrideVars["example"], NewStringVariable(key, value, "file"))
	}
}

func NewTestInputParser(opts ...testParserOption) *Parser {

	p := &Parser{
		fs:  afero.Afero{Fs: afero.OsFs{}},
		cfg: &ParserConfig{ParentPackID: "example"},
		rootVars: map[PackID]map[VariableID]*Variable{
			"example": {
				"input": &Variable{
					Name:      "input",
					Type:      cty.String,
					Value:     cty.StringVal("root"),
					DeclRange: hcl.Range{Filename: "<value for var.input from rootVars>"},
				},
			},
		},
		envOverrideVars:  make(map[PackID][]*Variable),
		fileOverrideVars: make(map[PackID][]*Variable),
		flagOverrideVars: make(map[PackID][]*Variable),
	}

	// Loop through each option
	for _, opt := range opts {
		opt(p)
	}

	return p
}

func NewStringVariable(key, value, kind string) *Variable {
	return &Variable{
		Name:      VariableID(key),
		Type:      cty.String,
		Value:     cty.StringVal(value),
		DeclRange: hcl.Range{Filename: fmt.Sprintf("<value for var %s from %s>", key, kind)},
	}
}
