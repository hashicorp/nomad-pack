// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package parser

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
	"github.com/hashicorp/nomad-pack/internal/pkg/variable/decoder"
	"github.com/hashicorp/nomad-pack/internal/pkg/variable/envloader"
	"github.com/hashicorp/nomad-pack/internal/pkg/variable/internal/hclhelp"
	"github.com/hashicorp/nomad-pack/internal/pkg/variable/parser/config"
	"github.com/hashicorp/nomad-pack/internal/pkg/variable/schema"
	"github.com/hashicorp/nomad-pack/sdk/pack"
	"github.com/hashicorp/nomad-pack/sdk/pack/variables"
	"github.com/spf13/afero"
	"github.com/zclconf/go-cty/cty"
)

type ParserV2 struct {
	fs  afero.Afero
	cfg *config.ParserConfig

	// rootVars contains all the root variable declared by all parent and child
	// packs that are being parsed. The first map is keyed by the pack name,
	// the second is by the variable name.
	rootVars map[pack.ID]map[variables.ID]*variables.Variable

	// envOverrideVars, fileOverrideVars, cliOverrideVars are the override
	// variables. The maps are keyed by the pack name they are associated to.
	envOverrideVars  variables.PackIDKeyedVarMap
	fileOverrideVars variables.PackIDKeyedVarMap
	flagOverrideVars variables.PackIDKeyedVarMap
}

func NewParserV2(cfg *config.ParserConfig) (*ParserV2, error) {

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

	return &ParserV2{
		fs: afero.Afero{
			Fs: afero.OsFs{},
		},
		cfg:              cfg,
		rootVars:         make(map[pack.ID]map[variables.ID]*variables.Variable),
		envOverrideVars:  make(variables.PackIDKeyedVarMap),
		fileOverrideVars: make(variables.PackIDKeyedVarMap),
		flagOverrideVars: make(variables.PackIDKeyedVarMap),
	}, nil
}

func (p *ParserV2) Parse() (*ParsedVariables, hcl.Diagnostics) {

	// Parse the root variables. If we encounter an error here, we are unable
	// to reliably continue.
	diags := p.parseRootFiles()
	if diags.HasErrors() {
		return nil, diags
	}

	// Parse env, file, and CLI overrides.
	for k, v := range p.cfg.EnvOverrides {
		envOverrideDiags := p.parseEnvVariable(k, v)
		diags = packdiags.SafeDiagnosticsExtend(diags, envOverrideDiags)
	}

	for _, fileOverride := range p.cfg.FileOverrides {
		_, fileOverrideDiags := p.newParseOverridesFile(fileOverride)
		diags = packdiags.SafeDiagnosticsExtend(diags, fileOverrideDiags)
	}

	for k, v := range p.cfg.FlagOverrides {
		flagOverrideDiags := p.parseFlagVariable(k, v)
		diags = packdiags.SafeDiagnosticsExtend(diags, flagOverrideDiags)
	}

	if diags.HasErrors() {
		return nil, diags
	}

	// Iterate all our override variables and merge these into our root
	// variables with the CLI taking highest priority.
	for _, override := range []variables.PackIDKeyedVarMap{p.envOverrideVars, p.fileOverrideVars, p.flagOverrideVars} {
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

	out := new(ParsedVariables)
	out.LoadV2Result(p.rootVars)

	return out, diags
}

func (p *ParserV2) newParseOverridesFile(file string) (map[string]*hcl.File, hcl.Diagnostics) {
	var diags hcl.Diagnostics

	src, err := p.fs.ReadFile(file)
	if err != nil {
		return nil, diags.Append(packdiags.DiagFileNotFound(file))
	}

	ovrds := make(variables.Overrides)

	// Decode into the local recipient object
	root := p.cfg.ParentPack
	if hfm, vfDiags := varfile.Decode(root, file, src, nil, &ovrds); vfDiags.HasErrors() {
		return hfm, vfDiags.Extend(diags)
	}
	for _, o := range ovrds[pack.ID(file)] {
		// Identify whether this variable override is for a dependency pack
		// and then handle it accordingly.
		p.newHandleOverride(o)
	}
	return nil, diags
}

func (p *ParserV2) newHandleOverride(o *variables.Override) {
	// Is Pack Variable Object?
	// Check whether the name has an associated entry within the root variable
	// mapping which indicates whether it's a pack object.
	if _, ok := p.cfg.RootVariableFiles[o.Path]; ok {
		p.newHandleOverrideVar(o)
	}
}

func (p *ParserV2) newHandleOverrideVar(o *variables.Override) {
	v := variables.Variable{
		Name:      o.Name,
		Type:      o.Type,
		Value:     o.Value,
		DeclRange: o.Range,
	}
	p.fileOverrideVars[o.Path] = append(p.fileOverrideVars[o.Path], &v)
}

// loadPackFile takes a pack.File and parses this using a hclparse.Parser. The
// file can be either HCL and JSON format.
func (p *ParserV2) loadPackFile(file *pack.File) (hcl.Body, hcl.Diagnostics) {

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

func (p *ParserV2) parseRootFiles() hcl.Diagnostics {

	var diags hcl.Diagnostics

	// Iterate all our root variable files.
	for name, file := range p.cfg.RootVariableFiles {

		hclBody, loadDiags := p.loadPackFile(file)
		diags = packdiags.SafeDiagnosticsExtend(diags, loadDiags)

		content, contentDiags := hclBody.Content(schema.VariableFileSchema)
		diags = packdiags.SafeDiagnosticsExtend(diags, contentDiags)

		rootVars, parseDiags := p.parseRootBodyContent(content)
		diags = packdiags.SafeDiagnosticsExtend(diags, parseDiags)

		// If we don't have any errors processing the file, and its content,
		// add an entry.
		if !diags.HasErrors() {
			p.rootVars[name] = rootVars
		}
	}

	return diags
}

// parseRootBodyContent process the body of a root variables file, parsing
// each variable block found.
func (p *ParserV2) parseRootBodyContent(body *hcl.BodyContent) (map[variables.ID]*variables.Variable, hcl.Diagnostics) {

	packRootVars := map[variables.ID]*variables.Variable{}

	var diags hcl.Diagnostics

	// Due to the parsing that uses variableFileSchema, it is safe to assume
	// every block has a type "variable".
	for _, block := range body.Blocks {
		cfg, cfgDiags := decoder.DecodeVariableBlock(block)
		diags = packdiags.SafeDiagnosticsExtend(diags, cfgDiags)
		if cfg != nil {
			packRootVars[cfg.Name] = cfg
		}
	}
	return packRootVars, diags
}

func (p *ParserV2) parseEnvVariable(name string, rawVal string) hcl.Diagnostics {
	return p.parseVariableImpl(name, rawVal, p.envOverrideVars, name, "environment")

}
func (p *ParserV2) parseFlagVariable(name string, rawVal string) hcl.Diagnostics {
	return p.parseVariableImpl(name, rawVal, p.flagOverrideVars, "-var", "arguments")
}

func (p *ParserV2) parseVariableImpl(name, rawVal string, tgt variables.PackIDKeyedVarMap, typeTxt, rangeDesc string) hcl.Diagnostics {
	if rangeDesc == "environment" {
		name = strings.TrimPrefix(name, envloader.DefaultPrefix)
	}

	// Split the name to see if we have a namespace CLI variable for a child
	// pack and set the default packVarName.
	splitName := strings.Split(name, ".")

	// Generate a filename based on the incoming var, so we have some context for
	// any HCL diagnostics.

	// Get a reasonable count for the lines in the provided value. You'd think
	// these had to be flat, but naaah.
	lines := strings.Split(rawVal, "\n")
	lc := len(lines)
	endCol := len(lines[lc-1])

	fakeRange := hcl.Range{
		Filename: fmt.Sprintf("<value for var %s from %s>", name, rangeDesc),
		Start:    hcl.Pos{Line: 1, Column: 1, Byte: 0},
		End:      hcl.Pos{Line: lc, Column: endCol, Byte: len(rawVal)},
	}

	var varPID pack.ID
	var varVID variables.ID

	if len(splitName) > 1 {
		// TODO: This is another part that needs to be smart about parsing into the
		// names so we could potentially set a value inside of an object.
		varPID = p.cfg.ParentPack.ID().Join(
			pack.ID("." + strings.Join(splitName[0:len(splitName)-1], ".")),
		)
		varVID = variables.ID(splitName[len(splitName)-1])
	} else {
		// There are no dots in the path; it must refer to the root pack.
		varPID = p.cfg.ParentPack.ID()
		varVID = variables.ID(splitName[0])
	}

	// If the variable has not been configured in the root then exit. This is a
	// standard requirement, especially because we would be unable to ensure a
	// consistent type.
	existing, exists := p.rootVars[varPID][varVID]

	if !exists {
		return hcl.Diagnostics{packdiags.DiagMissingRootVar(name, &fakeRange)}
	}

	expr, diags := hclhelp.ExpressionFromVariableDefinition(fakeRange.Filename, rawVal, existing.Type)
	if diags.HasErrors() {
		return diags
	}

	val, diags := expr.Value(nil)
	if diags.HasErrors() {
		return diags
	}

	// If our stored type isn't cty.NilType then attempt to covert the override
	// variable, so we know they are compatible.
	if existing.Type != cty.NilType {
		var err *hcl.Diagnostic
		val, err = hclhelp.ConvertValUsingType(val, existing.Type, expr.Range().Ptr())
		if err != nil {
			return hcl.Diagnostics{err}
		}
	}

	// We have a verified override variable.
	v := variables.Variable{
		Name:      varVID,
		Type:      val.Type(),
		Value:     val,
		DeclRange: fakeRange,
	}
	tgt[varPID] = append(tgt[varPID], &v)

	return nil
}
