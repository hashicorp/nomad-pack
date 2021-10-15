package variable

import (
	"errors"
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/hclparse"
	"github.com/hashicorp/nomad-pack/pkg/pack"
	"github.com/spf13/afero"
	"github.com/zclconf/go-cty/cty"
)

// Parser can parse, merge, and validate HCL variables from multiple different
// sources.
type Parser struct {
	fs  afero.Afero
	cfg *ParserConfig

	// rootVars contains all the root variable declared by all parent and child
	// packs that are being parsed. The first map is keyed by the pack name,
	// the second is by the variable name.
	rootVars map[string]map[string]*Variable

	// fileOverrideVars and cliOverrideVars are the override variables. The
	// maps are keyed by the pack name they are associated to.
	fileOverrideVars map[string][]*Variable
	cliOverrideVars  map[string][]*Variable
}

// ParserConfig contains details of the numerous sources of variables which
// should be parsed and merged according to the expected strategy.
type ParserConfig struct {

	// ParentName is the name representing the parent pack.
	ParentName string

	// RootVariableFiles contains a map of root variable files, keyed by their
	// pack name.
	RootVariableFiles map[string]*pack.File

	// FileOverrides is a list of files which contain variable overrides in the
	// form key=value. The files will be stored before processing to ensure a
	// consistent processing experience. Overrides here will replace any
	// default root declarations.
	FileOverrides []string

	// CLIOverrides are key=value variables and take the highest precedence of
	// all sources. If the same key is supplied twice, the last wins.
	CLIOverrides map[string]string
}

func NewParser(cfg *ParserConfig) (*Parser, error) {

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
			return nil, errors.New(fmt.Sprintf("Variable file `%s` not found", file))
		}
	}

	return &Parser{
		fs: afero.Afero{
			Fs: afero.OsFs{},
		},
		cfg:              cfg,
		rootVars:         make(map[string]map[string]*Variable),
		fileOverrideVars: make(map[string][]*Variable),
		cliOverrideVars:  make(map[string][]*Variable),
	}, nil
}

func (p *Parser) Parse() (*ParsedVariables, hcl.Diagnostics) {

	// Parse the root variables. If we encounter an error here, we are unable
	// to reliably continue.
	diags := p.parseRootFiles()
	if diags.HasErrors() {
		return nil, diags
	}

	// Parse file and CLI overrides.
	for _, fileOverride := range p.cfg.FileOverrides {
		fileOverrideDiags := p.parseOverridesFile(fileOverride)
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
	for _, override := range []map[string][]*Variable{p.fileOverrideVars, p.cliOverrideVars} {
		for packName, variables := range override {
			for _, v := range variables {
				existing, exists := p.rootVars[packName][v.Name]
				if !exists {
					diags = diags.Append(diagnosticMissingRootVar(v.Name, v.DeclRange.Ptr()))
					continue
				}
				if mergeDiags := existing.merge(v); mergeDiags.HasErrors() {
					diags = diags.Extend(mergeDiags)
				}
			}
		}
	}

	return &ParsedVariables{Vars: p.rootVars}, diags
}

func (p *Parser) loadOverrideFile(file string) (hcl.Body, hcl.Diagnostics) {

	src, err := p.fs.ReadFile(file)
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

func (p *Parser) parseOverridesFile(file string) hcl.Diagnostics {

	body, diags := p.loadOverrideFile(file)
	if body == nil {
		return diags
	}

	if diags == nil {
		diags = hcl.Diagnostics{}
	}

	attrs, hclDiags := body.JustAttributes()
	diags = safeDiagnosticsExtend(diags, hclDiags)

	for _, attr := range attrs {

		// Grab the expression value. If we have errors performing this we
		// cannot continue reliably.
		expr, valDiags := attr.Expr.Value(nil)
		if valDiags.HasErrors() {
			diags = safeDiagnosticsExtend(diags, valDiags)
			continue
		}

		// Identify whether this variable represents overrides concerned with
		// a dependent pack and then handle it accordingly.
		isPackVar, packVarDiags := p.isPackVariableObject(attr.Name, expr.Type())
		diags = safeDiagnosticsExtend(diags, packVarDiags)
		p.handleOverrideVar(isPackVar, attr, expr)
	}

	return diags
}

func (p *Parser) handleOverrideVar(isPackVar bool, attr *hcl.Attribute, expr cty.Value) {
	if isPackVar {
		p.handlePackVariableObject(attr.Name, expr, attr.Range)
	} else {
		v := Variable{
			Name:      attr.Name,
			Type:      expr.Type(),
			Value:     expr,
			DeclRange: attr.Range,
		}
		p.fileOverrideVars[p.cfg.ParentName] = append(p.fileOverrideVars[p.cfg.ParentName], &v)
	}
}

func (p *Parser) handlePackVariableObject(name string, expr cty.Value, declRange hcl.Range) {
	for k := range expr.Type().AttributeTypes() {
		av := expr.GetAttr(k)
		v := Variable{
			Name:      k,
			Type:      av.Type(),
			Value:     av,
			DeclRange: declRange,
		}
		p.fileOverrideVars[name] = append(p.fileOverrideVars[name], &v)
	}
}

func (p *Parser) isPackVariableObject(name string, typ cty.Type) (bool, hcl.Diagnostics) {

	// Check whether the name has an associated entry within the root variable
	// mapping which indicates whether it's a pack object.
	if _, ok := p.cfg.RootVariableFiles[name]; !ok {
		return false, nil
	}
	return typ.IsObjectType(), nil
}
