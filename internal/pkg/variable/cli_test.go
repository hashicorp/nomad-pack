package variable

import (
	"os"
	"path"
	"testing"

	"github.com/hashicorp/hcl/v2"
	"github.com/spf13/afero"
	"github.com/stretchr/testify/assert"
	"github.com/zclconf/go-cty/cty"
)

func TestParser_parseCLIVariable(t *testing.T) {
	testCases := []struct {
		inputParser     *Parser
		inputName       string
		inputRawVal     string
		expectedError   bool
		expectedCLIVars map[string][]*Variable
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
			name: "non-namespaced variable",
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
			assert.NotNil(t, actualErr, tc.name)
		} else {
			assert.Nil(t, actualErr, tc.name)
			assert.Equal(t, tc.expectedCLIVars, tc.inputParser.cliOverrideVars, tc.name)
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
	// FIXME: Find the fixture folder in a less janky way
	cwd, _ := os.Getwd()
	fixturePath := path.Join(cwd, "../../../fixtures/variables-with-heredoc/vars.hcl")
	b, diags := inputParser.loadOverrideFile(fixturePath)
	assert.NotNil(t, b)
	assert.Empty(t, diags)
}
