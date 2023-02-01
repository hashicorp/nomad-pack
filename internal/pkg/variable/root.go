// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package variable

import (
	"github.com/hashicorp/hcl/v2"
)

func (p *Parser) parseRootFiles() hcl.Diagnostics {

	var diags hcl.Diagnostics

	// Iterate all our root variable files.
	for name, file := range p.cfg.RootVariableFiles {

		hclBody, loadDiags := p.loadPackFile(file)
		diags = safeDiagnosticsExtend(diags, loadDiags)

		content, contentDiags := hclBody.Content(variableFileSchema)
		diags = safeDiagnosticsExtend(diags, contentDiags)

		rootVars, parseDiags := p.parseRootBodyContent(content)
		diags = safeDiagnosticsExtend(diags, parseDiags)

		// If we don't have any errors processing the file, and it's content,
		// add an entry.
		if !diags.HasErrors() {
			p.rootVars[name] = rootVars
		}
	}

	return diags
}

// parseRootBodyContent process the body of a root variables file, parsing
// each variable block found.
func (p *Parser) parseRootBodyContent(body *hcl.BodyContent) (map[string]*Variable, hcl.Diagnostics) {

	packRootVars := map[string]*Variable{}

	var diags hcl.Diagnostics

	// Due to the parsing that uses variableFileSchema, it is safe to assume
	// every block has a type "variable".
	for _, block := range body.Blocks {
		cfg, cfgDiags := decodeVariableBlock(block)
		diags = safeDiagnosticsExtend(diags, cfgDiags)
		if cfg != nil {
			packRootVars[cfg.Name] = cfg
		}
	}
	return packRootVars, diags
}
