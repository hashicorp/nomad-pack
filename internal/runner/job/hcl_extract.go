package job

import (
	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/hclparse"
	"github.com/zclconf/go-cty/cty"
)

// ExtractJobRegionNamespace parses the rendered HCL job spec and returns the
// region and namespace values from the top-level job block only.
// This avoids falsely treating any pack variable named "region" or "namespace"
// as Nomad client settings.
func ExtractJobRegionNamespace(hclText string) (region, namespace string, err error) {
	parser := hclparse.NewParser()
	file, diag := parser.ParseHCL([]byte(hclText), "job.hcl")
	if diag.HasErrors() {
		return "", "", diag
	}

	// The job block has one label (the job name), so LabelNames must be declared
	// for PartialContent to match job blocks correctly.
	content, _, diag := file.Body.PartialContent(&hcl.BodySchema{
		Blocks: []hcl.BlockHeaderSchema{{Type: "job", LabelNames: []string{"name"}}},
	})
	if diag.HasErrors() {
		return "", "", diag
	}

	ctx := &hcl.EvalContext{}

	for _, block := range content.Blocks {
		if block.Type == "job" {
			// Use PartialContent on the job body so that nested blocks (group, task, etc.)
			// are left in the "remain" body without triggering errors. Only region and
			// namespace attributes are extracted explicitly.
			jobContent, _, _ := block.Body.PartialContent(&hcl.BodySchema{
				Attributes: []hcl.AttributeSchema{
					{Name: "region", Required: false},
					{Name: "namespace", Required: false},
				},
			})
			if regAttr, ok := jobContent.Attributes["region"]; ok {
				val, diag := regAttr.Expr.Value(ctx)
				if !diag.HasErrors() && val.Type() == cty.String {
					region = val.AsString()
				}
			}
			if nsAttr, ok := jobContent.Attributes["namespace"]; ok {
				val, diag := nsAttr.Expr.Value(ctx)
				if !diag.HasErrors() && val.Type() == cty.String {
					namespace = val.AsString()
				}
			}
			break // only inspect the first job block
		}
	}
	return region, namespace, nil
}
