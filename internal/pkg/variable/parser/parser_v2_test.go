// Copyright IBM Corp. 2023, 2026
// SPDX-License-Identifier: MPL-2.0

package parser

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path"
	"path/filepath"
	"strings"
	"testing"

	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/nomad-pack/internal/pkg/loader"
	"github.com/hashicorp/nomad-pack/internal/pkg/testfixture"
	"github.com/hashicorp/nomad-pack/internal/pkg/variable/envloader"
	"github.com/hashicorp/nomad-pack/internal/pkg/variable/parser/config"
	"github.com/hashicorp/nomad-pack/sdk/pack"
	"github.com/hashicorp/nomad-pack/sdk/pack/variables"
	"github.com/hashicorp/nomad/ci"
	vault "github.com/hashicorp/vault/api"
	"github.com/shoenig/test/must"
	"github.com/spf13/afero"
	"github.com/zclconf/go-cty/cty"
)

func testpack(p ...string) *pack.Pack {
	name := strings.Join(p, ".")
	if name == "" {
		name = "example"
	}

	return &pack.Pack{
		Metadata: &pack.Metadata{
			Pack: &pack.MetadataPack{
				Name: name,
			},
		},
	}
}

func testVaultClient(t *testing.T, handler http.HandlerFunc) *vault.Client {
	t.Helper()

	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)

	cfg := vault.DefaultConfig()
	cfg.Address = server.URL

	client, err := vault.NewClient(cfg)
	must.NoError(t, err)

	return client
}

func TestParserV2_NewParserV2(t *testing.T) {
	t.Run("fails/with nil config set", func(t *testing.T) {
		p, err := NewParserV2(nil)
		must.Nil(t, p)
		must.Error(t, err)
		must.ErrorContains(t, err, "nil parser configuration")
	})
	t.Run("fails/without ParentPack set", func(t *testing.T) {
		p, err := NewParserV2(&config.ParserConfig{})
		must.Nil(t, p)
		must.Error(t, err)
		must.ErrorContains(t, err, "nil ParentPack")
	})
	t.Run("fails/with missing override file", func(t *testing.T) {
		p, err := NewParserV2(&config.ParserConfig{
			ParentPack:    testpack("example"),
			FileOverrides: []string{"/not/a/real/path/foo.hcl"},
		})
		must.Nil(t, p)
		must.Error(t, err)
		must.ErrorContains(t, err, "error loading variable file")
	})
	t.Run("passes", func(t *testing.T) {
		p, err := NewParserV2(&config.ParserConfig{
			ParentPack: testpack("example"),
		})
		must.NotNil(t, p)
		must.NoError(t, err)
	})
}

func TestParserV2_Parse_VaultVariableSource(t *testing.T) {
	vaultClient := testVaultClient(t, func(w http.ResponseWriter, r *http.Request) {
		must.Eq(t, "/v1/secret/data/myapp", r.URL.Path)

		w.Header().Set("Content-Type", "application/json")
		must.NoError(t, json.NewEncoder(w).Encode(map[string]any{
			"data": map[string]any{
				"data": map[string]any{
					"region": "us-east-1",
				},
			},
		}))
	})

	rootVar := &variables.Variable{
		Name:           "region",
		Type:           cty.String,
		ConstraintType: cty.String,
		Value:          cty.StringVal(""),
		DeclRange:      hcl.Range{Filename: "variables.hcl"},
	}

	p := &ParserV2{
		fs: afero.Afero{Fs: afero.OsFs{}},
		cfg: &config.ParserConfig{
			ParentPack:  testpack("example"),
			VaultClient: vaultClient,
			VariableSource: &config.VariableSourceConfig{
				Type: "vault",
				Vault: &config.VaultVariableSourceConfig{
					Path: "secret/data/myapp",
				},
			},
		},
		rootVars: map[pack.ID]map[variables.ID]*variables.Variable{
			"example": {
				"region": rootVar,
			},
		},
		sourceOverrideVars: make(variables.PackIDKeyedVarMap),
		envOverrideVars:    make(variables.PackIDKeyedVarMap),
		fileOverrideVars:   make(variables.PackIDKeyedVarMap),
		flagOverrideVars:   make(variables.PackIDKeyedVarMap),
	}

	diags := p.parseVariableSourceOverrides()
	must.False(t, diags.HasErrors(), must.Sprintf("unexpected diagnostics: %v", diags))

	sourceVars := p.sourceOverrideVars["example"]
	must.Len(t, 1, sourceVars)

	must.Eq(t, variables.ID("region"), sourceVars[0].Name)
	must.Eq(t, cty.StringVal("us-east-1"), sourceVars[0].Value)
}

func TestParserV2_Parse_EnvOverridesVaultVariableSource(t *testing.T) {
	vaultClient := testVaultClient(t, func(w http.ResponseWriter, r *http.Request) {
		must.Eq(t, "/v1/secret/data/myapp", r.URL.Path)

		w.Header().Set("Content-Type", "application/json")
		must.NoError(t, json.NewEncoder(w).Encode(map[string]any{
			"data": map[string]any{
				"data": map[string]any{
					"region": "vault-region",
				},
			},
		}))
	})

	rootVarFile := &pack.File{
		Name: "variables.hcl",
		Path: "variables.hcl",
		Content: []byte(`
variable "region" {
  type    = string
  default = ""
}
`),
	}

	p, err := NewParserV2(&config.ParserConfig{
		ParentPack: testpack("example"),
		RootVariableFiles: map[pack.ID]*pack.File{
			"example": rootVarFile,
		},
		EnvOverrides: map[string]string{
			"NOMAD_PACK_VAR_region": "env-region",
		},
		VaultClient: vaultClient,
		VariableSource: &config.VariableSourceConfig{
			Type: "vault",
			Vault: &config.VaultVariableSourceConfig{
				Path: "secret/data/myapp",
			},
		},
	})
	must.NoError(t, err)

	parsed, diags := p.Parse()
	must.False(t, diags.HasErrors(), must.Sprintf("unexpected diagnostics: %v", diags))
	must.NotNil(t, parsed)

	got := parsed.GetVars()["example"]["region"]
	must.NotNil(t, got)
	must.Eq(t, cty.StringVal("env-region"), got.Value)
}

func TestParserV2_Parse_FileOverridesVaultVariableSource(t *testing.T) {
	vaultClient := testVaultClient(t, func(w http.ResponseWriter, r *http.Request) {
		must.Eq(t, "/v1/secret/data/myapp", r.URL.Path)

		w.Header().Set("Content-Type", "application/json")
		must.NoError(t, json.NewEncoder(w).Encode(map[string]any{
			"data": map[string]any{
				"data": map[string]any{
					"region": "vault-region",
				},
			},
		}))
	})

	rootVarFile := &pack.File{
		Name: "variables.hcl",
		Path: "variables.hcl",
		Content: []byte(`
variable "region" {
  type    = string
  default = ""
}
`),
	}

	tmpDir := t.TempDir()
	overrideFilePath := path.Join(tmpDir, "region.override.hcl")
	must.NoError(t, os.WriteFile(overrideFilePath, []byte(`
region = "file-region"
`), 0o644))

	p, err := NewParserV2(&config.ParserConfig{
		ParentPack: testpack("example"),
		RootVariableFiles: map[pack.ID]*pack.File{
			"example": rootVarFile,
		},
		FileOverrides: []string{overrideFilePath},
		VaultClient:   vaultClient,
		VariableSource: &config.VariableSourceConfig{
			Type: "vault",
			Vault: &config.VaultVariableSourceConfig{
				Path: "secret/data/myapp",
			},
		},
	})
	must.NoError(t, err)

	parsed, diags := p.Parse()
	must.False(t, diags.HasErrors(), must.Sprintf("unexpected diagnostics: %v", diags))
	must.NotNil(t, parsed)

	got := parsed.GetVars()["example"]["region"]
	must.NotNil(t, got)
	must.Eq(t, cty.StringVal("file-region"), got.Value)
}

func TestParserV2_Parse_FlagOverridesVaultVariableSource(t *testing.T) {
	vaultClient := testVaultClient(t, func(w http.ResponseWriter, r *http.Request) {
		must.Eq(t, "/v1/secret/data/myapp", r.URL.Path)

		w.Header().Set("Content-Type", "application/json")
		must.NoError(t, json.NewEncoder(w).Encode(map[string]any{
			"data": map[string]any{
				"data": map[string]any{
					"region": "vault-region",
				},
			},
		}))
	})

	rootVarFile := &pack.File{
		Name: "variables.hcl",
		Path: "variables.hcl",
		Content: []byte(`
variable "region" {
  type    = string
  default = ""
}
`),
	}

	p, err := NewParserV2(&config.ParserConfig{
		ParentPack: testpack("example"),
		RootVariableFiles: map[pack.ID]*pack.File{
			"example": rootVarFile,
		},
		FlagOverrides: map[string]string{
			"region": "flag-region",
		},
		VaultClient: vaultClient,
		VariableSource: &config.VariableSourceConfig{
			Type: "vault",
			Vault: &config.VaultVariableSourceConfig{
				Path: "secret/data/myapp",
			},
		},
	})
	must.NoError(t, err)

	parsed, diags := p.Parse()
	must.False(t, diags.HasErrors(), must.Sprintf("unexpected diagnostics: %v", diags))
	must.NotNil(t, parsed)

	got := parsed.GetVars()["example"]["region"]
	must.NotNil(t, got)
	must.Eq(t, cty.StringVal("flag-region"), got.Value)
}

func TestParserV2_Parse_VaultVariableSource_IgnoresUndeclaredFields(t *testing.T) {
	vaultClient := testVaultClient(t, func(w http.ResponseWriter, r *http.Request) {
		must.Eq(t, "/v1/secret/data/myapp", r.URL.Path)

		w.Header().Set("Content-Type", "application/json")
		must.NoError(t, json.NewEncoder(w).Encode(map[string]any{
			"data": map[string]any{
				"data": map[string]any{
					"region":   "us-east-1",
					"ignored":  "value",
					"ignored2": 123,
				},
			},
		}))
	})

	rootVarFile := &pack.File{
		Name: "variables.hcl",
		Path: "variables.hcl",
		Content: []byte(`
variable "region" {
  type    = string
  default = ""
}
`),
	}

	p, err := NewParserV2(&config.ParserConfig{
		ParentPack: testpack("example"),
		RootVariableFiles: map[pack.ID]*pack.File{
			"example": rootVarFile,
		},
		VaultClient: vaultClient,
		VariableSource: &config.VariableSourceConfig{
			Type: "vault",
			Vault: &config.VaultVariableSourceConfig{
				Path: "secret/data/myapp",
			},
		},
	})
	must.NoError(t, err)

	parsed, diags := p.Parse()
	must.False(t, diags.HasErrors(), must.Sprintf("unexpected diagnostics: %v", diags))
	must.NotNil(t, parsed)

	got := parsed.GetVars()["example"]["region"]
	must.NotNil(t, got)
	must.Eq(t, cty.StringVal("us-east-1"), got.Value)
}

func TestParserV2_Parse_VaultVariableSource_TypeMismatch(t *testing.T) {
	vaultClient := testVaultClient(t, func(w http.ResponseWriter, r *http.Request) {
		must.Eq(t, "/v1/secret/data/myapp", r.URL.Path)

		w.Header().Set("Content-Type", "application/json")
		must.NoError(t, json.NewEncoder(w).Encode(map[string]any{
			"data": map[string]any{
				"data": map[string]any{
					"count": "not-a-number",
				},
			},
		}))
	})

	rootVarFile := &pack.File{
		Name: "variables.hcl",
		Path: "variables.hcl",
		Content: []byte(`
variable "count" {
  type    = number
  default = 1
}
`),
	}

	p, err := NewParserV2(&config.ParserConfig{
		ParentPack: testpack("example"),
		RootVariableFiles: map[pack.ID]*pack.File{
			"example": rootVarFile,
		},
		VaultClient: vaultClient,
		VariableSource: &config.VariableSourceConfig{
			Type: "vault",
			Vault: &config.VaultVariableSourceConfig{
				Path: "secret/data/myapp",
			},
		},
	})
	must.NoError(t, err)

	parsed, diags := p.Parse()
	must.True(t, diags.HasErrors())
	must.Nil(t, parsed)
}

func TestParserV2_Parse_VaultVariableSource_StringNumberToNumber(t *testing.T) {
	vaultClient := testVaultClient(t, func(w http.ResponseWriter, r *http.Request) {
		must.Eq(t, "/v1/secret/data/myapp", r.URL.Path)

		w.Header().Set("Content-Type", "application/json")
		must.NoError(t, json.NewEncoder(w).Encode(map[string]any{
			"data": map[string]any{
				"data": map[string]any{
					"count": "3",
				},
			},
		}))
	})

	rootVarFile := &pack.File{
		Name: "variables.hcl",
		Path: "variables.hcl",
		Content: []byte(`
variable "count" {
  type    = number
  default = 1
}
`),
	}

	p, err := NewParserV2(&config.ParserConfig{
		ParentPack: testpack("example"),
		RootVariableFiles: map[pack.ID]*pack.File{
			"example": rootVarFile,
		},
		VaultClient: vaultClient,
		VariableSource: &config.VariableSourceConfig{
			Type: "vault",
			Vault: &config.VaultVariableSourceConfig{
				Path: "secret/data/myapp",
			},
		},
	})
	must.NoError(t, err)

	parsed, diags := p.Parse()
	must.False(t, diags.HasErrors(), must.Sprintf("unexpected diagnostics: %v", diags))
	must.NotNil(t, parsed)

	got := parsed.GetVars()["example"]["count"]
	must.NotNil(t, got)
	must.True(t, got.Value.RawEquals(cty.NumberIntVal(3)))
}

func TestParserV2_Parse_VaultVariableSource_FloatNumberToNumber(t *testing.T) {
	vaultClient := testVaultClient(t, func(w http.ResponseWriter, r *http.Request) {
		must.Eq(t, "/v1/secret/data/myapp", r.URL.Path)

		w.Header().Set("Content-Type", "application/json")
		must.NoError(t, json.NewEncoder(w).Encode(map[string]any{
			"data": map[string]any{
				"data": map[string]any{
					"count": float64(3),
				},
			},
		}))
	})

	rootVarFile := &pack.File{
		Name: "variables.hcl",
		Path: "variables.hcl",
		Content: []byte(`
variable "count" {
  type    = number
  default = 1
}
`),
	}

	p, err := NewParserV2(&config.ParserConfig{
		ParentPack: testpack("example"),
		RootVariableFiles: map[pack.ID]*pack.File{
			"example": rootVarFile,
		},
		VaultClient: vaultClient,
		VariableSource: &config.VariableSourceConfig{
			Type: "vault",
			Vault: &config.VaultVariableSourceConfig{
				Path: "secret/data/myapp",
			},
		},
	})
	must.NoError(t, err)

	parsed, diags := p.Parse()
	must.False(t, diags.HasErrors(), must.Sprintf("unexpected diagnostics: %v", diags))
	must.NotNil(t, parsed)

	got := parsed.GetVars()["example"]["count"]
	must.NotNil(t, got)
	must.True(t, got.Value.RawEquals(cty.NumberIntVal(3)))
}

func TestParserV2_parseFlagVariable(t *testing.T) {
	testCases := []struct {
		inputParser      *ParserV2
		inputName        string
		inputRawVal      string
		expectedError    bool
		expectedFlagVars variables.PackIDKeyedVarMap
		expectedEnvVars  variables.PackIDKeyedVarMap
		name             string
	}{
		{
			name: "non-namespaced variable",
			inputParser: &ParserV2{
				fs:  afero.Afero{Fs: afero.OsFs{}},
				cfg: &config.ParserConfig{ParentPack: testpack()},
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
				flagOverrideVars: make(variables.PackIDKeyedVarMap),
			},
			inputName:     "region",
			inputRawVal:   "vlc",
			expectedError: false,
			expectedFlagVars: variables.PackIDKeyedVarMap{
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
			name: "namespaced variable",
			inputParser: &ParserV2{
				fs:  afero.Afero{Fs: afero.OsFs{}},
				cfg: &config.ParserConfig{ParentPack: testpack()},
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
				flagOverrideVars: make(variables.PackIDKeyedVarMap),
			},
			inputName:        "example.region",
			inputRawVal:      "vlc",
			expectedError:    true,
			expectedFlagVars: variables.PackIDKeyedVarMap{},
		},
		{
			inputParser: &ParserV2{
				fs:               afero.Afero{Fs: afero.OsFs{}},
				cfg:              &config.ParserConfig{ParentPack: testpack()},
				rootVars:         map[pack.ID]map[variables.ID]*variables.Variable{},
				flagOverrideVars: make(variables.PackIDKeyedVarMap),
			},
			inputName:        "example.region",
			inputRawVal:      "vlc",
			expectedError:    true,
			expectedFlagVars: variables.PackIDKeyedVarMap{},
			name:             "root variable absent",
		},
		{
			name: "unconvertable variable",
			inputParser: &ParserV2{
				fs:  afero.Afero{Fs: afero.OsFs{}},
				cfg: &config.ParserConfig{ParentPack: testpack()},
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
				flagOverrideVars: make(variables.PackIDKeyedVarMap),
			},
			inputName:        "example.region",
			inputRawVal:      "vlc",
			expectedError:    true,
			expectedFlagVars: variables.PackIDKeyedVarMap{},
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
		expectedFlagVars variables.PackIDKeyedVarMap
		expectedEnvVars  variables.PackIDKeyedVarMap
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
		var k = getEnvKey(tc)
		var v = getEnvValue(tc)
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
				cfg: &config.ParserConfig{ParentPack: testpack()},
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
				envOverrideVars: make(variables.PackIDKeyedVarMap),
			},
			expectedError: false,
			expectedEnvVars: variables.PackIDKeyedVarMap{
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
			name: "namespaced variable",
			inputParser: &ParserV2{
				fs:  afero.Afero{Fs: afero.OsFs{}},
				cfg: &config.ParserConfig{ParentPack: testpack()},
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
				envOverrideVars: make(variables.PackIDKeyedVarMap),
			},
			expectedError:   true,
			expectedEnvVars: variables.PackIDKeyedVarMap{},
		},
		{
			name: "root variable absent",
			inputParser: &ParserV2{
				fs:               afero.Afero{Fs: afero.OsFs{}},
				cfg:              &config.ParserConfig{ParentPack: testpack()},
				rootVars:         map[pack.ID]map[variables.ID]*variables.Variable{},
				flagOverrideVars: make(variables.PackIDKeyedVarMap),
				envOverrideVars:  make(variables.PackIDKeyedVarMap),
			},
			expectedError:    true,
			expectedFlagVars: variables.PackIDKeyedVarMap{},
			expectedEnvVars:  variables.PackIDKeyedVarMap{},
		},
		{
			name:     "unconvertable variable",
			envKey:   "NOMAD_PACK_VAR_example.region",
			envValue: `{region: "dc1}`,
			inputParser: &ParserV2{
				fs:  afero.Afero{Fs: afero.OsFs{}},
				cfg: &config.ParserConfig{ParentPack: testpack()},
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
				flagOverrideVars: make(variables.PackIDKeyedVarMap),
				envOverrideVars:  make(variables.PackIDKeyedVarMap),
			},
			expectedError:    true,
			expectedFlagVars: variables.PackIDKeyedVarMap{},
			expectedEnvVars:  variables.PackIDKeyedVarMap{},
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
				t.Log(actualErr.Error())
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
			ParentPack:        testpack("variable_test_pack"),
			RootVariableFiles: map[pack.ID]*pack.File{},
		},
		rootVars:         map[pack.ID]map[variables.ID]*variables.Variable{},
		fileOverrideVars: make(variables.PackIDKeyedVarMap),
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

func TestParserV2_FileOverridesFollowInputOrder(t *testing.T) {
	tmpDir := t.TempDir()
	first := filepath.Join(tmpDir, "b-first.hcl")
	second := filepath.Join(tmpDir, "a-second.hcl")

	must.NoError(t, os.WriteFile(first, []byte("input = \"from-first\"\n"), 0o644))
	must.NoError(t, os.WriteFile(second, []byte("input = \"from-second\"\n"), 0o644))

	fixturePath := testfixture.AbsPath(t, "v2/variable_test/variable_test")
	pm := newTestPackManager(t, fixturePath, false)
	pm.cfg.VariableFiles = []string{first, second}

	pvs := pm.ProcessVariables()
	must.NotNil(t, pvs)
	must.Eq(t, "from-second", pvs.v2Vars["variable_test_pack"]["input"].Value.AsString())
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
		cfg: &config.ParserConfig{ParentPack: testpack()},
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
		envOverrideVars:  make(variables.PackIDKeyedVarMap),
		fileOverrideVars: make(variables.PackIDKeyedVarMap),
		flagOverrideVars: make(variables.PackIDKeyedVarMap),
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

func TestParsedVariables_GetNomadVars(t *testing.T) {
	ci.Parallel(t)

	t.Run("returns empty map when no nomad variables", func(t *testing.T) {
		pv := &ParsedVariables{
			nomadVars: make(map[pack.ID][]*variables.NomadVariable),
		}
		nvs := pv.GetNomadVars()
		must.NotNil(t, nvs)
		must.MapEmpty(t, nvs)
	})

	t.Run("returns nil when nomadVars is nil", func(t *testing.T) {
		pv := &ParsedVariables{
			nomadVars: nil,
		}
		nvs := pv.GetNomadVars()
		must.Nil(t, nvs)
	})

	t.Run("returns nomad variables map", func(t *testing.T) {
		nv1 := &variables.NomadVariable{
			Name: "test1",
			Path: "nomad/jobs/test1",
		}
		nv2 := &variables.NomadVariable{
			Name: "test2",
			Path: "nomad/jobs/test2",
		}
		pv := &ParsedVariables{
			nomadVars: map[pack.ID][]*variables.NomadVariable{
				"example": {nv1, nv2},
			},
		}
		nvs := pv.GetNomadVars()
		must.Eq(t, 1, len(nvs))
		must.Eq(t, 2, len(nvs["example"]))
		must.Eq(t, "test1", nvs["example"][0].Name)
		must.Eq(t, "test2", nvs["example"][1].Name)
	})
}
