// Copyright IBM Corp. 2021, 2025
// SPDX-License-Identifier: MPL-2.0

package parser

import (
	"path"
	"strings"
	"testing"

	"github.com/hashicorp/nomad-pack/internal/pkg/loader"
	"github.com/hashicorp/nomad-pack/internal/pkg/variable/parser/config"
	"github.com/hashicorp/nomad-pack/sdk/pack"
	"github.com/shoenig/test/must"
)

type testPackManagerConfig struct {
	Path            string
	VariableFiles   []string
	VariableCLIArgs map[string]string
	VariableEnvVars map[string]string
	UseParserV1     bool
}

type testPackManager struct {
	T   *testing.T
	cfg *testPackManagerConfig

	// loadedPack is unavailable until the loadAndValidatePacks func is run.
	loadedPack *pack.Pack
}

func newTestPackManager(t *testing.T, path string, useParserV1 bool) *testPackManager {
	return &testPackManager{
		T: t,
		cfg: &testPackManagerConfig{
			Path:        path,
			UseParserV1: useParserV1,
		},
	}
}

func (pm *testPackManager) ProcessVariables() *ParsedVariables {
	t := pm.T

	// LoadAndValidatePacks uses variables to hide the implementation from publishing
	// itself within the test. This has to be all CHONKY because manager depends
	// on variables, so we can't create a real pack manager to handle the pack
	// state.
	var loadAndValidatePacks = func() (*pack.Pack, error) {

		parentPack, err := loader.Load(pm.cfg.Path)
		must.NoError(t, err)
		must.NoError(t, parentPack.Validate())

		// Using the input path to the parent pack, define the path where
		// dependencies are stored.
		depsPath := path.Join(pm.cfg.Path, "deps")

		// This spectacular line is because the value doesn't exist to be recursed
		// until runtime, so we have to declare it here for futire happiness.
		var loadAndValidatePackR func(cur *pack.Pack, depsPath string) error

		loadAndValidatePackR = func(cur *pack.Pack, depsPath string) error {

			for _, dep := range cur.Metadata.Dependencies {
				// Load and validate the dependency pack.
				packPath := path.Join(depsPath, path.Clean(dep.Name))
				depPack, err := loader.Load(packPath)
				must.NoError(t, err)
				must.NoError(t, depPack.Validate())
				cur.AddDependency(dep.ID(), depPack)
				must.NoError(t, loadAndValidatePackR(depPack, path.Join(packPath, "deps")))
			}

			return nil
		}
		must.NoError(t, loadAndValidatePackR(parentPack, depsPath))

		return parentPack, nil
	}

	loadedPack, err := loadAndValidatePacks()
	must.NoError(t, err)

	pm.loadedPack = loadedPack

	// Root vars are nested under the pack name, which is currently the pack name
	// without the version.
	parentName, _, _ := strings.Cut(path.Base(pm.cfg.Path), "@")

	pCfg := &config.ParserConfig{
		Version:           config.V2,
		ParentPack:        loadedPack,
		RootVariableFiles: loadedPack.RootVariableFiles(),
		EnvOverrides:      pm.cfg.VariableEnvVars,
		FileOverrides:     pm.cfg.VariableFiles,
		FlagOverrides:     pm.cfg.VariableCLIArgs,
	}

	if pm.cfg.UseParserV1 {
		pCfg.Version = config.V1
		pCfg.ParentName = parentName
	}

	variableParser, err := NewParser(pCfg)
	must.NoError(t, err)

	parsedVars, diags := variableParser.Parse()
	must.False(t, diags.HasErrors())

	return parsedVars
}
