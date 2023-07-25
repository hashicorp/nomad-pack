package varfile_test

import (
	"testing"

	"github.com/davecgh/go-spew/spew"
	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/nomad-pack/internal/pkg/varfile"
	"github.com/shoenig/test/must"
	"github.com/zclconf/go-cty/cty"
)

func TestVarfile_DecodeHCL(t *testing.T) {
	type exp struct {
		dLen  int
		diags hcl.Diagnostics
		oLen  int
		oMap  varfile.Overrides
	}
	testCases := []struct {
		name string
		src  []byte
		exp  exp
	}{
		{
			name: "empty",
			src:  []byte{},
			exp:  exp{},
		},
		{
			name: "comment only",
			src:  []byte("# just a comment"),
			exp:  exp{},
		},
		{
			name: "single override",
			src:  []byte(`mypack.foo = "bar"`),
			exp: exp{
				dLen: 0,
				oLen: 1,
				oMap: varfile.Overrides{
					"embedded.hcl": []*varfile.Override{
						{
							Name:  "foo",
							Path:  "mypack",
							Type:  cty.String,
							Value: cty.StringVal("bar"),
							Range: hcl.Range{
								Filename: "embedded.hcl",
								Start:    hcl.Pos{Line: 1, Column: 1, Byte: 0},
								End:      hcl.Pos{Line: 1, Column: 19, Byte: 18},
							},
						},
					},
				},
			},
		},
		{
			name: "missing equal",
			src:  []byte(`mypack.foo "bar"`),
			exp: exp{
				dLen: 1,
				diags: hcl.Diagnostics{
					&hcl.Diagnostic{
						Severity: hcl.DiagError,
						Summary:  `Missing key/value separator`,
						Detail:   `Expected an equals sign ("=") to mark the beginning of the attribute value.`,
						Subject: &hcl.Range{
							Filename: "embedded.hcl",
							Start:    hcl.Pos{Line: 1, Column: 12, Byte: 11},
							End:      hcl.Pos{Line: 1, Column: 13, Byte: 12},
						},
						Context: &hcl.Range{
							Filename: "embedded.hcl",
							Start:    hcl.Pos{Line: 1, Column: 1, Byte: 0},
							End:      hcl.Pos{Line: 1, Column: 13, Byte: 12},
						},
						EvalContext: nil,
						Extra:       varfile.DiagExtraFixup{Fixed: true},
					},
				},
			},
		},
		{
			name: "error only",
			src:  []byte("boom"),
			exp: exp{
				dLen: 1,
				oLen: 0,
			},
		},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			tc := tc
			om := make(varfile.Overrides)
			_, diags := varfile.Decode("embedded.hcl", tc.src, nil, &om)
			must.Len(t, tc.exp.dLen, diags, must.Sprintf("slice values: %v", diags))

			if len(tc.exp.diags) > 0 {
				for i, o := range diags {
					e := tc.exp.diags[i]
					must.Eq(t, e, o)
				}
			}
			// There should always be a single element map for these tests having the
			// filename as the key
			switch tc.exp.oLen {
			case 0:
				must.MapLen(t, 0, om, must.Sprintf("map contents: %v", spew.Sdump(om)))
			default:
				must.MapLen(t, 1, om, must.Sprintf("map contents: %v", spew.Sdump(om)))
				must.MapContainsKey(t, om, "embedded.hcl")
			}

			if len(tc.exp.oMap) > 0 {
				// Extract the expect slice
				eSlice := tc.exp.oMap["embedded.hcl"]
				// Extract the slice
				oSlice := om["embedded.hcl"]
				must.SliceLen(t, tc.exp.oLen, oSlice, must.Sprintf("slice values: %v", oSlice))

				for i, o := range oSlice {
					e := eSlice[i]
					must.True(t, e.Equal(o))
				}
			}
		})
	}
}
