// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package variable

import (
	"errors"
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/hclparse"
	"github.com/hashicorp/nomad-pack/internal/pkg/errors/packdiags"
	"github.com/hashicorp/nomad-pack/internal/pkg/varfile"
	"github.com/hashicorp/nomad-pack/sdk/pack"
	"github.com/spf13/afero"
)

// Parser can parse, merge, and validate HCL variables from multiple different
// sources.
type Parser struct {
	fs  afero.Afero
	cfg *ParserConfig

	// rootVars contains all the root variable declared by all parent and child
	// packs that are being parsed. The first map is keyed by the pack name,
	// the second is by the variable name.
	rootVars map[PackID]map[VariableID]*Variable

	// envOverrideVars, fileOverrideVars, cliOverrideVars are the override
	// variables. The maps are keyed by the pack name they are associated to.
	envOverrideVars  map[PackID][]*Variable
	fileOverrideVars map[PackID][]*Variable
	cliOverrideVars  map[PackID][]*Variable
}

// ParserConfig contains details of the numerous sources of variables which
// should be parsed and merged according to the expected strategy.
type ParserConfig struct {

	// ParentPackID is the PackID of the parent pack.
	ParentPackID PackID

	// RootVariableFiles contains a map of root variable files, keyed by their
	// absolute pack name. "«root pack name».«child pack».«grandchild pack»"
	RootVariableFiles map[PackID]*pack.File

	// EnvOverrides are key=value variables and take the lowest precedence of
	// all sources. If the same key is supplied twice, the last wins.
	EnvOverrides map[string]string

	// FileOverrides is a list of files which contain variable overrides in the
	// form key=value. The files will be stored before processing to ensure a
	// consistent processing experience. Overrides here will replace any
	// default root declarations.
	FileOverrides []string

	// CLIOverrides are key=value variables and take the highest precedence of
	// all sources. If the same key is supplied twice, the last wins.
	CLIOverrides map[string]string

	// IgnoreMissingVars determines whether we error or not on variable overrides
	// that don't have corresponding vars in the pack.
	IgnoreMissingVars bool
}

func NewParser(cfg *ParserConfig) (*Parser, error) {

	// Ensure the parent name is set, otherwise we can't parse correctly.
	if cfg.ParentPackID == "" {
		return nil, errors.New("variable parser config requires ParentName to be set")
	}

	// Sort the file overrides to ensure variable merging is consistent on
	// multiple passes.
	sort.Strings(cfg.FileOverrides)
	for _, file := range cfg.FileOverrides {
		_, err := os.Stat(file)
		if err != nil {
			return nil, fmt.Errorf("variable file %q not found", file)
		}
	}

	return &Parser{
		fs: afero.Afero{
			Fs: afero.OsFs{},
		},
		cfg:              cfg,
		rootVars:         make(map[PackID]map[VariableID]*Variable),
		envOverrideVars:  make(PackIDKeyedVarMap),
		fileOverrideVars: make(PackIDKeyedVarMap),
		cliOverrideVars:  make(PackIDKeyedVarMap),
	}, nil
}

type PackIDKeyedVarMap map[PackID][]*Variable

func (p PackIDKeyedVarMap) Variables(k PackID) []*Variable { return p[k] }
func (p PackIDKeyedVarMap) AsMapOfStringToVariable() map[string][]*Variable {
	var o map[string][]*Variable = make(map[string][]*Variable)
	for k, v := range p {
		o[string(k)] = v
	}
	return o
}

func (p *Parser) Parse() (*ParsedVariables, hcl.Diagnostics) {

	// Parse the root variables. If we encounter an error here, we are unable
	// to reliably continue.
	diags := p.parseRootFiles()
	if diags.HasErrors() {
		return nil, diags
	}

	// Parse env, file, and CLI overrides.
	for k, v := range p.cfg.EnvOverrides {
		envOverrideDiags := p.parseEnvVariable(k, v)
		diags = safeDiagnosticsExtend(diags, envOverrideDiags)
	}

	for _, fileOverride := range p.cfg.FileOverrides {
		_, fileOverrideDiags := p.newParseOverridesFile(fileOverride)
		diags = safeDiagnosticsExtend(diags, fileOverrideDiags)
	}

	for k, v := range p.cfg.CLIOverrides {
		cliOverrideDiags := p.parseCLIVariable(k, v)
		diags = safeDiagnosticsExtend(diags, cliOverrideDiags)
	}

	if diags.HasErrors() {
		return nil, diags
	}

	// Iterate all our override variables and merge these into our root
	// variables with the CLI taking highest priority.
	for _, override := range []map[PackID][]*Variable{p.envOverrideVars, p.fileOverrideVars, p.cliOverrideVars} {
		for packName, variables := range override {
			for _, v := range variables {
				existing, exists := p.rootVars[packName][v.Name]
				if !exists {
					if !p.cfg.IgnoreMissingVars {
						diags = diags.Append(packdiags.DiagMissingRootVar(v.Name.String(), v.DeclRange.Ptr()))
					}
					continue
				}
				if mergeDiags := existing.Merge(v); mergeDiags.HasErrors() {
					diags = diags.Extend(mergeDiags)
				}
			}
		}
	}

	return &ParsedVariables{Vars: p.rootVars}, diags
}

func (p *Parser) newParseOverridesFile(file string) (map[string]*hcl.File, hcl.Diagnostics) {
	var diags hcl.Diagnostics

	src, err := p.fs.ReadFile(file)
	if err != nil {
		return nil, diags.Append(packdiags.DiagFileNotFound(file))
	}

	ovrds := make(varfile.Overrides)

	// Decode into the local recipient object
	if hfm, vfDiags := varfile.Decode(file, src, nil, &ovrds); vfDiags.HasErrors() {
		return hfm, vfDiags.Extend(diags)
	}
	for _, o := range ovrds[varfile.PackID(file)] {
		// Identify whether this variable override is for a dependency pack
		// and then handle it accordingly.
		p.newHandleOverride(o)
	}
	return nil, diags
}

func (p *Parser) newHandleOverride(o *varfile.Override) {
	// Is Pack Variable Object?
	// Check whether the name has an associated entry within the root variable
	// mapping which indicates whether it's a pack object.
	if _, ok := p.cfg.RootVariableFiles[o.Path]; ok {
		p.newHandleOverrideVar(o)
	}
}

func (p *Parser) newHandleOverrideVar(o *varfile.Override) {
	v := Variable{
		Name:      o.Name,
		Type:      o.Type,
		Value:     o.Value,
		DeclRange: o.Range,
	}
	p.fileOverrideVars[o.Path] = append(p.fileOverrideVars[o.Path], &v)
}

// loadPackFile takes a pack.File and parses this using a hclparse.Parser. The
// file can be either HCL and JSON format.
func (p *Parser) loadPackFile(file *pack.File) (hcl.Body, hcl.Diagnostics) {

	var (
		hclFile *hcl.File
		diags   hcl.Diagnostics
	)

	// Instantiate a new parser each time. Using the same parser where variable
	// names collide from different packs will cause problems.
	hclParser := hclparse.NewParser()

	// Depending on the fix extension, use the correct HCL parser.
	switch {
	case strings.HasSuffix(file.Name, ".json"):
		hclFile, diags = hclParser.ParseJSON(file.Content, file.Path)
	default:
		hclFile, diags = hclParser.ParseHCL(file.Content, file.Path)
	}

	// If the returned file or body is nil, then we'll return a non-nil empty
	// body, so we'll meet our contract that nil means an error reading the
	// file.
	if hclFile == nil || hclFile.Body == nil {
		return hcl.EmptyBody(), diags
	}

	return hclFile.Body, diags
}
