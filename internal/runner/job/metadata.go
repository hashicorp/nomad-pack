// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package job

import (
	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/gohcl"
	"github.com/hashicorp/hcl/v2/hclsyntax"
	"github.com/hashicorp/hcl/v2/hclwrite"
	"github.com/hashicorp/nomad/jobspec2/hclutil"
	"github.com/zclconf/go-cty/cty"
)

const (
	PackPathKey           = "pack.path"
	PackNameKey           = "pack.name"
	PackRegistryKey       = "pack.registry"
	PackDeploymentNameKey = "pack.deployment_name"
	PackJobKey            = "pack.job"
	PackRefKey            = "pack.version"
)

// setHCLMeta sets the nomad-pack metadata in the HCL job definition, merging
// the values with any existing meta block or attribute. If the parsing fails,
// or the HCL is invalid, the original job is returned unmodified so that the
// errors can be caught later during job parsing.
func (r *Runner) setHCLMeta(job string) string {
	file, diags := hclsyntax.ParseConfig([]byte(job), "", hcl.Pos{Line: 1, Column: 1})
	if diags.HasErrors() {
		return job
	}
	content, diags := file.Body.Content(&hcl.BodySchema{
		Blocks: []hcl.BlockHeaderSchema{
			{Type: "job", LabelNames: []string{""}},
		},
	})
	if diags.HasErrors() {
		return job
	}

	result := map[string]cty.Value{}
	deleteBlock := false
	for _, b := range content.Blocks {
		cont, _, diags := b.Body.PartialContent(&hcl.BodySchema{
			Attributes: []hcl.AttributeSchema{
				{Name: "meta", Required: false},
			},
			Blocks: []hcl.BlockHeaderSchema{
				{Type: "meta"},
			},
		})
		if diags.HasErrors() {
			return job
		}

		if len(cont.Blocks) == 0 && len(cont.Attributes) == 0 {
			continue
		}

		// If an meta attribute is defined, we will merge the values with the
		// nomad-pack ones.  If a meta block exists but no attribute, we need to
		// extract the value from the block for merging.  If both are defined by
		// the user, the attribute takes precedence and the block is ignored.
		// This is invalid HCL and will be caught during the job parsing, but
		// prevents deletion of user defined values.
		if len(cont.Attributes) == 0 && len(cont.Blocks) > 0 {
			deleteBlock = true

			b := hclutil.BlocksAsAttrs(b.Body)
			attrs, diags := b.JustAttributes()
			if diags.HasErrors() {
				return job
			}
			r := []map[string]cty.Value{}
			diag := gohcl.DecodeExpression(attrs["meta"].Expr, nil, &r)
			if diag.HasErrors() {
				return job
			}
			result = r[0]
		} else {
			metaExpr := cont.Attributes["meta"].Expr
			diag := gohcl.DecodeExpression(metaExpr, nil, &result)
			if diag.HasErrors() {
				return job
			}
		}
	}

	wFile, _ := hclwrite.ParseConfig([]byte(job), "", hcl.Pos{Line: 1, Column: 1})
	rootBody := wFile.Body()

	var jobBlock *hclwrite.Block
	for _, b := range rootBody.Blocks() {
		if b.Type() == "job" {
			jobBlock = b
			break
		}
	}

	if jobBlock == nil {
		return job
	}
	jobBody := jobBlock.Body()

	result[PackPathKey] = cty.StringVal(r.runnerCfg.PathPath)
	result[PackNameKey] = cty.StringVal(r.runnerCfg.PackName)
	result[PackRegistryKey] = cty.StringVal(r.runnerCfg.RegistryName)
	result[PackDeploymentNameKey] = cty.StringVal(r.runnerCfg.DeploymentName)
	result[PackJobKey] = cty.StringVal(jobBlock.Labels()[0])
	result[PackRefKey] = cty.StringVal(r.runnerCfg.PackRef)

	jobBody.SetAttributeValue("meta", cty.ObjectVal(result))

	// If we extracted the meta data from a block, delete it so we don't return invalid HCL
	if deleteBlock {
		jobBody.RemoveBlock(jobBody.FirstMatchingBlock("meta", nil))
	}

	return string(wFile.Bytes())
}
