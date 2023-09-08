// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package parser

import (
	"fmt"
	"path"
	"strings"
	"testing"

	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/nomad-pack/internal/pkg/loader"
	"github.com/hashicorp/nomad-pack/internal/pkg/testfixture"
	"github.com/hashicorp/nomad-pack/internal/pkg/variable/envloader"
	"github.com/hashicorp/nomad-pack/internal/pkg/variable/parser/config"
	"github.com/hashicorp/nomad-pack/sdk/pack"
	"github.com/hashicorp/nomad-pack/sdk/pack/variables"
	"github.com/shoenig/test/must"
	"github.com/spf13/afero"
	"github.com/zclconf/go-cty/cty"
)

func TestParserV2_parseFlagVariable(t *testing.T) {
	testCases := []struct {
		inputParser      *ParserV2
		inputName        string
		inputRawVal      string
		expectedError    bool
		expectedFlagVars map[pack.ID][]*variables.Variable
		expectedEnvVars  map[pack.ID][]*variables.Variable
		name             string
	}{
		{
			name: "non-namespaced variable",
			inputParser: &ParserV2{
				fs:  afero.Afero{Fs: afero.OsFs{}},
				cfg: &config.ParserConfig{ParentPackID: "example"},
				rootVars: map[pack.ID]map[variables.ID]*variables.Variable{
					"example": {
						"region": &variables.Variable{
							Name:      "region",
							Type:      cty.String,
							Value:     cty.StringVal("vlc"),
							DeclRange: hcl.Range{Filename: "<value for var region from arguments>"},
						},
					},
				},
				flagOverrideVars: make(map[pack.ID][]*variables.Variable),
				envOverrideVars:  make(map[pack.ID][]*variables.Variable),
			},
			inputName:        "region",
			inputRawVal:      "vlc",
			expectedError:    true,
			expectedFlagVars: map[pack.ID][]*variables.Variable{},
			expectedEnvVars:  make(map[pack.ID][]*variables.Variable),
		},
		{
			name: "namespaced variable",
			inputParser: &ParserV2{
				fs:  afero.Afero{Fs: afero.OsFs{}},
				cfg: &config.ParserConfig{ParentPackID: "example"},
				rootVars: map[pack.ID]map[variables.ID]*variables.Variable{
					"example": {
						"region": &variables.Variable{
							Name:      "region",
							Type:      cty.String,
							Value:     cty.StringVal("vlc"),
							DeclRange: hcl.Range{Filename: "<value for var region from arguments>"},
						},
					},
				},
				flagOverrideVars: make(map[pack.ID][]*variables.Variable),
			},
			inputName:     "example.region",
			inputRawVal:   "vlc",
			expectedError: false,
			expectedFlagVars: map[pack.ID][]*variables.Variable{
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
			inputParser: &ParserV2{
				fs:               afero.Afero{Fs: afero.OsFs{}},
				cfg:              &config.ParserConfig{ParentPackID: "example"},
				rootVars:         map[pack.ID]map[variables.ID]*variables.Variable{},
				flagOverrideVars: make(map[pack.ID][]*variables.Variable),
			},
			inputName:        "example.region",
			inputRawVal:      "vlc",
			expectedError:    true,
			expectedFlagVars: map[pack.ID][]*variables.Variable{},
			name:             "root variable absent",
		},
		{
			name: "unconvertable variable",
			inputParser: &ParserV2{
				fs:  afero.Afero{Fs: afero.OsFs{}},
				cfg: &config.ParserConfig{ParentPackID: "example"},
				rootVars: map[pack.ID]map[variables.ID]*variables.Variable{
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
				flagOverrideVars: make(map[pack.ID][]*variables.Variable),
			},
			inputName:        "example.region",
			inputRawVal:      "vlc",
			expectedError:    true,
			expectedFlagVars: map[pack.ID][]*variables.Variable{},
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

func TestParserV2_parseEnvVariable(t *testing.T) {
	type testCase struct {
		inputParser      *ParserV2
		envKey           string
		envValue         string
		expectedError    bool
		expectedFlagVars map[pack.ID][]*variables.Variable
		expectedEnvVars  map[pack.ID][]*variables.Variable
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
		return strings.TrimPrefix(k, envloader.DefaultPrefix)
	}

	testCases := []testCase{
		{
			name:   "non-namespaced variable",
			envKey: "NOMAD_PACK_VAR_region",
			inputParser: &ParserV2{
				fs:  afero.Afero{Fs: afero.OsFs{}},
				cfg: &config.ParserConfig{ParentPackID: "example"},
				rootVars: map[pack.ID]map[variables.ID]*variables.Variable{
					"example": {
						"region": &variables.Variable{
							Name:      "region",
							Type:      cty.String,
							Value:     cty.StringVal("vlc"),
							DeclRange: hcl.Range{Filename: "<value for var region from arguments>"},
						},
					},
				},
				flagOverrideVars: make(map[pack.ID][]*variables.Variable),
				envOverrideVars:  make(map[pack.ID][]*variables.Variable),
			},
			expectedError:    true,
			expectedFlagVars: map[pack.ID][]*variables.Variable{},
			expectedEnvVars:  make(map[pack.ID][]*variables.Variable),
		},
		{
			name: "namespaced variable",
			inputParser: &ParserV2{
				fs:  afero.Afero{Fs: afero.OsFs{}},
				cfg: &config.ParserConfig{ParentPackID: "example"},
				rootVars: map[pack.ID]map[variables.ID]*variables.Variable{
					"example": {
						"region": &variables.Variable{
							Name:      "region",
							Type:      cty.String,
							Value:     cty.StringVal("vlc"),
							DeclRange: hcl.Range{Filename: "<value for var example.region from arguments>"},
						},
					},
				},
				flagOverrideVars: make(map[pack.ID][]*variables.Variable),
				envOverrideVars:  make(map[pack.ID][]*variables.Variable),
			},
			expectedError:    false,
			expectedFlagVars: map[pack.ID][]*variables.Variable{},
			expectedEnvVars: map[pack.ID][]*variables.Variable{
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
			inputParser: &ParserV2{
				fs:               afero.Afero{Fs: afero.OsFs{}},
				cfg:              &config.ParserConfig{ParentPackID: "example"},
				rootVars:         map[pack.ID]map[variables.ID]*variables.Variable{},
				flagOverrideVars: make(map[pack.ID][]*variables.Variable),
				envOverrideVars:  make(map[pack.ID][]*variables.Variable),
			},
			expectedError:    true,
			expectedFlagVars: map[pack.ID][]*variables.Variable{},
			expectedEnvVars:  map[pack.ID][]*variables.Variable{},
		},
		{
			name:     "unconvertable variable",
			envKey:   "NOMAD_PACK_VAR_example.region",
			envValue: `{region: "dc1}`,
			inputParser: &ParserV2{
				fs:  afero.Afero{Fs: afero.OsFs{}},
				cfg: &config.ParserConfig{ParentPackID: "example"},
				rootVars: map[pack.ID]map[variables.ID]*variables.Variable{
					"example": {
						"region": &variables.Variable{
							Name: "region",
							Type: cty.DynamicPseudoType,
							Value: cty.MapVal(map[string]cty.Value{
								"region": cty.StringVal("dc1"),
							}),
							DeclRange: hcl.Range{Filename: "<value for var example.region from arguments>"},
						},
					},
				},
				flagOverrideVars: make(map[pack.ID][]*variables.Variable),
				envOverrideVars:  make(map[pack.ID][]*variables.Variable),
			},
			expectedError:    true,
			expectedFlagVars: map[pack.ID][]*variables.Variable{},
			expectedEnvVars:  map[pack.ID][]*variables.Variable{},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			mapKey := setTestEnvKeyForVar(t, tc)

			em := envloader.New().GetVarsFromEnv()
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

func TestParserV2_parseHeredocAtEOF(t *testing.T) {
	inputParser := &ParserV2{
		fs: afero.Afero{Fs: afero.OsFs{}},
		cfg: &config.ParserConfig{
			RootVariableFiles: map[pack.ID]*pack.File{},
		},
		rootVars:         map[pack.ID]map[variables.ID]*variables.Variable{},
		fileOverrideVars: make(map[pack.ID][]*variables.Variable),
	}

	fixtureRoot := testfixture.AbsPath(t, "v2/variable_test")
	p, err := loader.Load(fixtureRoot + "/variable_test")
	must.NoError(t, err)
	must.NotNil(t, p)

	inputParser.cfg.RootVariableFiles = p.RootVariableFiles()

	_, diags := inputParser.newParseOverridesFile(path.Join(fixtureRoot, "/heredoc.vars.hcl"))
	must.False(t, diags.HasErrors(), must.Sprintf("diags: %v", diags))
	must.Len(t, 1, inputParser.fileOverrideVars["variable_test_pack"])
	must.Eq(t, "heredoc\n", inputParser.fileOverrideVars["variable_test_pack"][0].Value.AsString())
}

func TestParserV2_VariableOverrides(t *testing.T) {
	testcases := []struct {
		Name   string
		Parser *ParserV2
		Expect string
	}{
		{
			Name:   "no override",
			Parser: NewTestInputParserV2(),
			Expect: "root",
		},
		{
			Name:   "env override",
			Parser: NewTestInputParserV2(WithEnvVar("input", "env")),
			Expect: "env",
		},
		{
			Name:   "file override",
			Parser: NewTestInputParserV2(WithFileVar("input", "file")),
			Expect: "file",
		},
		{
			Name:   "flag override",
			Parser: NewTestInputParserV2(WithCliVar("input", "flag")),
			Expect: "flag",
		},
		{
			Name: "file opaques env",
			Parser: NewTestInputParserV2(
				WithEnvVar("input", "env"),
				WithFileVar("input", "file"),
			),
			Expect: "file",
		},
		{
			Name: "flag opaques env",
			Parser: NewTestInputParserV2(
				WithEnvVar("input", "env"),
				WithCliVar("input", "flag"),
			),
			Expect: "flag",
		},
		{
			Name: "flag opaques file",
			Parser: NewTestInputParserV2(
				WithFileVar("input", "file"),
				WithCliVar("input", "flag"),
			),
			Expect: "flag",
		},
		{
			Name: "flag opaques env and file",
			Parser: NewTestInputParserV2(
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

			must.Eq(t, tc.Expect, pv.v2Vars["example"]["input"].Value.AsString())
		})
	}
}

type testParserV2Option func(*ParserV2)

func WithEnvVar(key, value string) testParserV2Option {
	return func(p *ParserV2) {
		p.envOverrideVars["example"] = append(p.envOverrideVars["example"], NewStringVariableV2(key, value, "env"))
	}
}

func WithCliVar(key, value string) testParserV2Option {
	return func(p *ParserV2) {
		p.flagOverrideVars["example"] = append(p.flagOverrideVars["example"], NewStringVariableV2(key, value, "cli"))
	}
}

func WithFileVar(key, value string) testParserV2Option {
	return func(p *ParserV2) {
		p.flagOverrideVars["example"] = append(p.flagOverrideVars["example"], NewStringVariableV2(key, value, "file"))
	}
}

func NewTestInputParserV2(opts ...testParserV2Option) *ParserV2 {

	p := &ParserV2{
		fs:  afero.Afero{Fs: afero.OsFs{}},
		cfg: &config.ParserConfig{ParentPackID: "example"},
		rootVars: map[pack.ID]map[variables.ID]*variables.Variable{
			"example": {
				"input": &variables.Variable{
					Name:      "input",
					Type:      cty.String,
					Value:     cty.StringVal("root"),
					DeclRange: hcl.Range{Filename: "<value for var input from rootVars>"},
				},
			},
		},
		envOverrideVars:  make(map[pack.ID][]*variables.Variable),
		fileOverrideVars: make(map[pack.ID][]*variables.Variable),
		flagOverrideVars: make(map[pack.ID][]*variables.Variable),
	}

	// Loop through each option
	for _, opt := range opts {
		opt(p)
	}

	return p
}

func NewStringVariableV2(key, value, kind string) *variables.Variable {
	return &variables.Variable{
		Name:      variables.ID(key),
		Type:      cty.String,
		Value:     cty.StringVal(value),
		DeclRange: hcl.Range{Filename: fmt.Sprintf("<value for var %s from %s>", key, kind)},
	}
}
