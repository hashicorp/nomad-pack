// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package manager

import (
	"fmt"
	"path"
	"strings"

	"github.com/hashicorp/nomad-pack/internal/pkg/errors"
	"github.com/hashicorp/nomad-pack/internal/pkg/loader"
	"github.com/hashicorp/nomad-pack/internal/pkg/renderer"
	"github.com/hashicorp/nomad-pack/internal/pkg/variable/parser"
	"github.com/hashicorp/nomad-pack/internal/pkg/variable/parser/config"
	"github.com/hashicorp/nomad-pack/sdk/pack"
	"github.com/hashicorp/nomad/api"
)

// Config contains all the user specified parameters needed to correctly run
// the pack manager.
type Config struct {
	Path            string
	VariableFiles   []string
	VariableCLIArgs map[string]string
	VariableEnvVars map[string]string
	UseParserV1     bool
}

// PackManager is responsible for loading, parsing, and rendering a Pack and
// all dependencies.
type PackManager struct {
	cfg      *Config
	client   *api.Client
	renderer *renderer.Renderer

	// loadedPack is unavailable until the loadAndValidatePacks func is run.
	loadedPack *pack.Pack
}

func NewPackManager(cfg *Config, client *api.Client) *PackManager {
	return &PackManager{
		cfg:    cfg,
		client: client,
	}
}

// ProcessVariableFiles creates the map of packs to their respective variables
// definition files. This is used between the variable override file generator
// code and the ProcessTemplates logic in this file.
func (pm *PackManager) ProcessVariableFiles() (*parser.ParsedVariables, []*errors.WrappedUIContext) {
	loadedPack, err := pm.loadAndValidatePacks()
	if err != nil {
		return nil, []*errors.WrappedUIContext{{
			Err:     err,
			Subject: "failed to validate packs",
			Context: errors.NewUIErrorContext(),
		}}
	}

	pm.loadedPack = loadedPack

	// Root vars are nested under the pack name, which is currently the pack name
	// without the version.
	parentName, _, _ := strings.Cut(path.Base(pm.cfg.Path), "@")

	pCfg := &config.ParserConfig{
		Version:           config.V2,
		ParentPack:        pm.loadedPack,
		RootVariableFiles: loadedPack.RootVariableFiles(),
		EnvOverrides:      pm.cfg.VariableEnvVars,
		FileOverrides:     pm.cfg.VariableFiles,
		FlagOverrides:     pm.cfg.VariableCLIArgs,
	}

	if pm.cfg.UseParserV1 {
		pCfg.Version = config.V1
		pCfg.ParentName = parentName
	}

	variableParser, err := parser.NewParser(pCfg)
	if err != nil {
		return nil, []*errors.WrappedUIContext{{
			Err:     err,
			Subject: "failed to instantiate parser",
			Context: errors.NewUIErrorContext(),
		}}
	}

	parsedVars, diags := variableParser.Parse()
	if diags != nil && diags.HasErrors() {
		return nil, errors.HCLDiagsToWrappedUIContext(diags)
	}

	return parsedVars, nil
}

// ProcessTemplates is responsible for running all backend process for the
// PackManager returning an error along with the ProcessedPack. This contains
// all the rendered templates.
//
// TODO(jrasell) figure out whether we want an error or hcl.Diagnostics return
// object. If we stick to an error, then we need to come up with a way of
// nicely formatting them.
func (pm *PackManager) ProcessTemplates(renderAux bool, format bool, ignoreMissingVars bool) (*renderer.Rendered, []*errors.WrappedUIContext) {

	parsedVars, wErr := pm.ProcessVariableFiles()
	if wErr != nil {
		return nil, wErr
	}

	// Pre-test the parsed variables so that we can trust them
	// in rendering and to use for errors later
	tplCtx, diags := parsedVars.ToPackTemplateContext(pm.loadedPack)
	if diags != nil && diags.HasErrors() {
		return nil, errors.HCLDiagsToWrappedUIContext(diags)
	}

	r := new(renderer.Renderer)
	r.Client = pm.client
	r.PackPath = pm.cfg.Path
	pm.renderer = r

	// should auxiliary files be rendered as well?
	pm.renderer.RenderAuxFiles = renderAux

	// should we format before rendering?
	pm.renderer.Format = format

	rendered, err := r.Render(pm.loadedPack, parsedVars)
	if err != nil {
		return nil, []*errors.WrappedUIContext{
			errors.ParseTemplateError(tplCtx, err).ToWrappedUIContext(),
		}
	}
	return rendered, nil
}

// ProcessOutputTemplate performs the output template rendering.
func (pm *PackManager) ProcessOutputTemplate() (string, error) {
	return pm.renderer.RenderOutput()
}

// loadAndValidatePacks triggers the initial parent load and then starts the
// dependent pack loader. The returned pack will therefore be fully populated.
func (pm *PackManager) loadAndValidatePacks() (*pack.Pack, error) {

	parentPack, err := loader.Load(pm.cfg.Path)
	if err != nil {
		return nil, fmt.Errorf("failed to load pack: %v", err)
	}

	if err := parentPack.Validate(); err != nil {
		return nil, fmt.Errorf("failed to validate pack: %v", err)
	}

	// Using the input path to the parent pack, define the path where
	// dependencies are stored.
	depsPath := path.Join(pm.cfg.Path, "deps")

	if err := pm.loadAndValidatePack(parentPack, depsPath); err != nil {
		return nil, fmt.Errorf("failed to load pack dependency: %v", err)
	}

	return parentPack, nil
}

// loadAndValidatePack recursively loads a pack and its dependencies. Errors
// result in an immediate return.
func (pm *PackManager) loadAndValidatePack(cur *pack.Pack, depsPath string) error {

	for _, dep := range cur.Metadata.Dependencies {

		// Skip any dependencies that are not enabled.
		if !*dep.Enabled {
			continue
		}

		// Load and validate the dependency pack.
		packPath := path.Join(depsPath, path.Clean(dep.Name))
		depPack, err := loader.Load(packPath)
		if err != nil {
			return fmt.Errorf("failed to load dependent pack: %v", err)
		}

		if err := depPack.Validate(); err != nil {
			return fmt.Errorf("failed to validate dependent pack: %v", err)
		}

		// Add the dependency to the current pack.
		cur.AddDependency(dep.ID(), depPack)

		// Recursive call.
		if err := pm.loadAndValidatePack(depPack, path.Join(packPath, "deps")); err != nil {
			return err
		}
	}

	return nil
}

func (pm *PackManager) PackName() string {
	if pm.loadedPack != nil {
		return pm.loadedPack.Name()
	}

	name := path.Base(pm.cfg.Path)
	idx := strings.LastIndex(path.Base(pm.cfg.Path), "@")
	if idx != -1 {
		name = path.Base(pm.cfg.Path)[0:idx]
	}
	return name
}

func (pm *PackManager) Metadata() *pack.Metadata {
	if pm.loadedPack == nil {
		return nil
	}
	return pm.loadedPack.Metadata
}
