// Copyright IBM Corp. 2021, 2025
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
	"github.com/hashicorp/nomad-pack/internal/pkg/variable/decoder"
	"github.com/hashicorp/nomad-pack/internal/pkg/variable/internal/hclhelp"
	"github.com/hashicorp/nomad-pack/internal/pkg/variable/parser/config"
	"github.com/hashicorp/nomad-pack/internal/pkg/variable/schema"
	"github.com/hashicorp/nomad-pack/sdk/pack"
	"github.com/hashicorp/nomad-pack/sdk/pack/variables"
	"github.com/spf13/afero"
	"github.com/zclconf/go-cty/cty"
)

const VarEnvPrefix = "NOMAD_PACK_VAR_"

// ParserV1 can parse, merge, and validate HCL variables from multiple different
// sources.
type ParserV1 struct {
	fs  afero.Afero
	cfg *config.ParserConfig

	// rootVars contains all the root variable declared by all parent and child
	// packs that are being parsed. The first map is keyed by the pack name,
	// the second is by the variable name.
	rootVars map[string]map[string]*variables.Variable

	// envOverrideVars, fileOverrideVars, cliOverrideVars are the override
	// variables. The maps are keyed by the pack name they are associated to.
	envOverrideVars  map[string][]*variables.Variable
	fileOverrideVars map[string][]*variables.Variable
	cliOverrideVars  map[string][]*variables.Variable
}

func NewParserV1(cfg *config.ParserConfig) (*ParserV1, error) {

	// Ensure the parent name is set, otherwise we can't parse correctly.
	if cfg.ParentName == "" {
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

	return &ParserV1{
		fs: afero.Afero{
			Fs: afero.OsFs{},
		},
		cfg:              cfg,
		rootVars:         make(map[string]map[string]*variables.Variable),
		envOverrideVars:  make(map[string][]*variables.Variable),
		fileOverrideVars: make(map[string][]*variables.Variable),
		cliOverrideVars:  make(map[string][]*variables.Variable),
	}, nil
}

func (p *ParserV1) Parse() (*ParsedVariables, hcl.Diagnostics) {

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
		fileOverrideDiags := p.parseOverridesFile(fileOverride)
		diags = packdiags.SafeDiagnosticsExtend(diags, fileOverrideDiags)
	}

	for k, v := range p.cfg.FlagOverrides {
		cliOverrideDiags := p.parseCLIVariable(k, v)
		diags = packdiags.SafeDiagnosticsExtend(diags, cliOverrideDiags)
	}

	if diags.HasErrors() {
		return nil, diags
	}

	// Iterate all our override variables and merge these into our root
	// variables with the CLI taking highest priority.
	for _, override := range []map[string][]*variables.Variable{p.envOverrideVars, p.fileOverrideVars, p.cliOverrideVars} {
		for packName, variables := range override {
			for _, v := range variables {
				existing, exists := p.rootVars[packName][v.Name.String()]
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
	out.LoadV1Result(p.rootVars)
	return out, diags
}

func (p *ParserV1) loadOverrideFile(file string) (hcl.Body, hcl.Diagnostics) {

	src, err := p.fs.ReadFile(file)
	// FIXME - Workaround for ending heredoc with no linefeed.
	// Variables files shouldn't care about the extra linefeed, but jamming one
	// in all the time feels bad.
	src = append(src, "\n"...)
	if err != nil {
		return nil, hcl.Diagnostics{
			{
				Severity: hcl.DiagError,
				Summary:  "Failed to read file",
				Detail:   fmt.Sprintf("The file %q could not be read.", file),
			},
		}
	}

	return p.loadPackFile(&pack.File{Path: file, Content: src})
}

// loadPackFile takes a pack.File and parses this using a hclparse.Parser. The
// file can be either HCL and JSON format.
func (p *ParserV1) loadPackFile(file *pack.File) (hcl.Body, hcl.Diagnostics) {

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

func (p *ParserV1) parseOverridesFile(file string) hcl.Diagnostics {

	body, diags := p.loadOverrideFile(file)
	if body == nil {
		return diags
	}

	if diags == nil {
		diags = hcl.Diagnostics{}
	}

	attrs, hclDiags := body.JustAttributes()
	diags = packdiags.SafeDiagnosticsExtend(diags, hclDiags)

	for _, attr := range attrs {

		// Grab the expression value. If we have errors performing this we
		// cannot continue reliably.
		expr, valDiags := attr.Expr.Value(nil)
		if valDiags.HasErrors() {
			diags = packdiags.SafeDiagnosticsExtend(diags, valDiags)
			continue
		}

		// Identify whether this variable represents overrides concerned with
		// a dependent pack and then handle it accordingly.
		isPackVar, packVarDiags := p.isPackVariableObject(pack.ID(attr.Name), expr.Type())
		diags = packdiags.SafeDiagnosticsExtend(diags, packVarDiags)
		p.handleOverrideVar(isPackVar, attr, expr)
	}

	return diags
}

func (p *ParserV1) handleOverrideVar(isPackVar bool, attr *hcl.Attribute, expr cty.Value) {
	if isPackVar {
		p.handlePackVariableObject(attr.Name, expr, attr.Range)
	} else {
		v := variables.Variable{
			Name:      variables.ID(attr.Name),
			Type:      expr.Type(),
			Value:     expr,
			DeclRange: attr.Range,
		}
		p.fileOverrideVars[p.cfg.ParentName] = append(p.fileOverrideVars[p.cfg.ParentName], &v)
	}
}

func (p *ParserV1) handlePackVariableObject(name string, expr cty.Value, declRange hcl.Range) {
	for k := range expr.Type().AttributeTypes() {
		av := expr.GetAttr(k)
		v := variables.Variable{
			Name:      variables.ID(k),
			Type:      av.Type(),
			Value:     av,
			DeclRange: declRange,
		}
		p.fileOverrideVars[name] = append(p.fileOverrideVars[name], &v)
	}
}

func (p *ParserV1) isPackVariableObject(name pack.ID, typ cty.Type) (bool, hcl.Diagnostics) {

	// Check whether the name has an associated entry within the root variable
	// mapping which indicates whether it's a pack object.
	if _, ok := p.cfg.RootVariableFiles[name]; !ok {
		return false, nil
	}
	return typ.IsObjectType(), nil
}

func (p *ParserV1) parseEnvVariable(name string, rawVal string) hcl.Diagnostics {
	// Split the name to see if we have a namespace CLI variable for a child
	// pack and set the default packVarName.
	splitName := strings.SplitN(name, ".", 2)
	packVarName := []string{p.cfg.ParentName, name}

	switch len(splitName) {
	case 1:
		// Fallthrough, nothing to do or see.
	case 2:
		// We are dealing with a namespaced variable. Overwrite the preset
		// values of packVarName.
		packVarName[0] = splitName[0]
		packVarName[1] = splitName[1]
	default:
		// We cannot handle a splitName where the variable includes more than
		// one separator.
		return hcl.Diagnostics{
			{
				Severity: hcl.DiagError,
				Summary:  fmt.Sprintf("Invalid %s option", strings.TrimRight(VarEnvPrefix, "_")),
				Detail:   fmt.Sprintf("The given environment variable %s%s=%s is not correctly specified. The variable name must not have more than one dot `.` separator.", VarEnvPrefix, name, rawVal),
			},
		}
	}

	// Generate a filename based on the CLI var, so we have some context for any
	// HCL diagnostics.
	fakeRange := hcl.Range{Filename: fmt.Sprintf("<value for var.%s from environment>", name)}

	// If the variable has not been configured in the root then ignore it. This
	// is a departure from the way in which flags and var-files are handled.
	// The environment might contain NOMAD_PACK_VAR variables used for other
	// packs that might be run on the same system but are not used with this
	// particular pack.
	existing, exists := p.rootVars[packVarName[0]][packVarName[1]]
	if !exists {
		return nil
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
		Name:      variables.ID(packVarName[1]),
		Type:      val.Type(),
		Value:     val,
		DeclRange: fakeRange,
	}
	p.envOverrideVars[packVarName[0]] = append(p.envOverrideVars[packVarName[0]], &v)

	return nil
}

func (p *ParserV1) parseCLIVariable(name string, rawVal string) hcl.Diagnostics {
	// Split the name to see if we have a namespace CLI variable for a child
	// pack and set the default packVarName.
	splitName := strings.SplitN(name, ".", 2)
	packVarName := []string{p.cfg.ParentName, name}

	switch len(splitName) {
	case 1:
		// Fallthrough, nothing to do or see.
	case 2:
		// We are dealing with a namespaced variable. Overwrite the preset
		// values of packVarName.
		packVarName[0] = splitName[0]
		packVarName[1] = splitName[1]
	default:
		// We cannot handle a splitName where the variable includes more than
		// one separator.
		return hcl.Diagnostics{
			{
				Severity: hcl.DiagError,
				Summary:  "Invalid -var option",
				Detail:   fmt.Sprintf("The given -var option %s=%s is not correctly specified. The variable name must not have more than one dot `.` separator.", name, rawVal),
			},
		}
	}

	// Generate a filename based on the CLI var, so we have some context for any
	// HCL diagnostics.
	fakeRange := hcl.Range{Filename: fmt.Sprintf("<value for var.%s from arguments>", name)}

	// If the variable has not been configured in the root then exit. This is a
	// standard requirement, especially because we would be unable to ensure a
	// consistent type.
	existing, exists := p.rootVars[packVarName[0]][packVarName[1]]
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
		Name:      variables.ID(packVarName[1]),
		Type:      val.Type(),
		Value:     val,
		DeclRange: fakeRange,
	}
	p.cliOverrideVars[packVarName[0]] = append(p.cliOverrideVars[packVarName[0]], &v)

	return nil
}

func (p *ParserV1) parseRootFiles() hcl.Diagnostics {

	var diags hcl.Diagnostics

	// Iterate all our root variable files.
	for name, file := range p.cfg.RootVariableFiles {

		hclBody, loadDiags := p.loadPackFile(file)
		diags = packdiags.SafeDiagnosticsExtend(diags, loadDiags)

		content, contentDiags := hclBody.Content(schema.VariableFileSchema)
		diags = packdiags.SafeDiagnosticsExtend(diags, contentDiags)

		rootVars, parseDiags := p.parseRootBodyContent(content)
		diags = packdiags.SafeDiagnosticsExtend(diags, parseDiags)

		// If we don't have any errors processing the file, and it's content,
		// add an entry.
		if !diags.HasErrors() {
			// The v2 loader returns pack names in dotted ancestor form,
			// grap the last element of the string
			parts := strings.Split(name.String(), ".")
			name := parts[len(parts)-1]
			p.rootVars[name] = rootVars
		}
	}

	return diags
}

// parseRootBodyContent process the body of a root variables file, parsing
// each variable block found.
func (p *ParserV1) parseRootBodyContent(body *hcl.BodyContent) (map[string]*variables.Variable, hcl.Diagnostics) {

	packRootVars := map[string]*variables.Variable{}

	var diags hcl.Diagnostics

	// Due to the parsing that uses variableFileSchema, it is safe to assume
	// every block has a type "variable".
	for _, block := range body.Blocks {
		cfg, cfgDiags := decoder.DecodeVariableBlock(block)
		diags = packdiags.SafeDiagnosticsExtend(diags, cfgDiags)
		if cfg != nil {
			packRootVars[cfg.Name.String()] = cfg
		}
	}
	return packRootVars, diags
}
