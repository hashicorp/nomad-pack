// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package varfile

import (
	"strings"
	"testing"

	"github.com/davecgh/go-spew/spew"
	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/nomad-pack/sdk/pack"
	"github.com/hashicorp/nomad-pack/sdk/pack/variables"
	"github.com/shoenig/test/must"
	"github.com/zclconf/go-cty/cty"
)

func testpack(p ...string) *pack.Pack {
	name := strings.Join(p, ".")
	if name == "" {
		name = "example"
	}

	return &pack.Pack{
		Metadata: &pack.Metadata{
			Pack: &pack.MetadataPack{
				Name: name,
			},
		},
	}
}

func TestVarfile_DecodeHCL(t *testing.T) {
	spew.Config.DisableMethods = true
	type exp struct {
		dLen  int
		diags hcl.Diagnostics
		oLen  int
		oMap  variables.Overrides
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
			src:  []byte(`foo = "bar"`),
			exp: exp{
				dLen: 0,
				oLen: 1,
				oMap: variables.Overrides{
					"embedded.hcl": []*variables.Override{
						{
							Name:  "foo",
							Path:  "mypack",
							Type:  cty.String,
							Value: cty.StringVal("bar"),
							Range: hcl.Range{
								Filename: "embedded.hcl",
								Start:    hcl.Pos{Line: 1, Column: 1, Byte: 0},
								End:      hcl.Pos{Line: 1, Column: 12, Byte: 11},
							},
						},
					},
				},
			},
		},
		{
			name: "missing equal",
			src:  []byte(`foo "bar"`),
			exp: exp{
				dLen: 1,
				diags: hcl.Diagnostics{
					&hcl.Diagnostic{
						Severity: hcl.DiagError,
						Summary:  `Missing key/value separator`,
						Detail:   `Expected an equals sign ("=") to mark the beginning of the attribute value.`,
						Subject: &hcl.Range{
							Filename: "embedded.hcl",
							Start:    hcl.Pos{Line: 1, Column: 5, Byte: 4},
							End:      hcl.Pos{Line: 1, Column: 6, Byte: 5},
						},
						Context: &hcl.Range{
							Filename: "embedded.hcl",
							Start:    hcl.Pos{Line: 1, Column: 1, Byte: 0},
							End:      hcl.Pos{Line: 1, Column: 6, Byte: 5},
						},
						EvalContext: nil,
						Extra:       DiagExtraFixup{Fixed: true},
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
			root := testpack("mypack")
			om := make(variables.Overrides)
			_, diags := Decode(root, "embedded.hcl", tc.src, nil, &om)
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
				must.MapLen[variables.Overrides](t, 0, om, must.Sprintf("map contents: %v", spew.Sdump(om)))
			default:
				must.MapLen[variables.Overrides](t, 1, om, must.Sprintf("map contents: %v", spew.Sdump(om)))
				must.MapContainsKey[variables.Overrides](t, om, "embedded.hcl")
			}

			if len(tc.exp.oMap) > 0 {
				// Extract the expect slice
				eSlice := tc.exp.oMap["embedded.hcl"]
				// Extract the slice
				oSlice := om["embedded.hcl"]
				must.SliceLen[*variables.Override](t, tc.exp.oLen, oSlice, must.Sprintf("slice values: %v", oSlice))

				for i, o := range oSlice {
					e := eSlice[i]
					must.True(t, e.Equal(o), must.Sprintf("e: %+v\no: %+v\n", spew.Sprintf("%+v", e), spew.Sprintf("%+v", o)))
				}
			}
		})
	}
}

func TestVarfile_DecodeResult_Merge(t *testing.T) {
	d1 := DecodeResult{
		Overrides: variables.Overrides{
			"p1": []*variables.Override{{Name: "o1"}, {Name: "o2"}},
		},
	}
	t.Run("errors when redefined", func(t *testing.T) {
		dr := DecodeResult{
			Overrides: variables.Overrides{
				"p1": []*variables.Override{{Name: "o1"}},
			},
		}

		dr.Merge(d1)
		must.True(t, dr.Diags.HasErrors())
		must.ErrorContains(t, dr.Diags, "variable o1 can not be redefined")
	})
}

func TestVarFile_Merge_Good(t *testing.T) {
	d1 := DecodeResult{
		Overrides: variables.Overrides{
			"p1": []*variables.Override{{Name: "o1"}, {Name: "o2"}},
		},
	}

	t.Run("succeeds for", func(t *testing.T) {
		t.Run("okay for variables of same name in different pack", func(t *testing.T) {
			dr := DecodeResult{
				Overrides: variables.Overrides{
					"p2": []*variables.Override{{Name: "o1"}, {Name: "o2"}},
				},
			}
			dr.Merge(d1)
			must.False(t, dr.Diags.HasErrors())
			must.Len[*variables.Override](t, 2, dr.Overrides["p1"])
			must.Len[*variables.Override](t, 2, dr.Overrides["p2"])
		})

		t.Run("okay for repeated pointers to same override", func(t *testing.T) {
			dr := DecodeResult{
				Overrides: variables.Overrides{
					"p2": []*variables.Override{{Name: "o1"}, {Name: "o2"}},
				},
			}
			dr2 := dr
			dr2.Merge(dr)
			must.False(t, dr.Diags.HasErrors())
			must.Len[*variables.Override](t, 2, dr.Overrides["p2"])
		})

		t.Run("for nil overrides", func(t *testing.T) {
			dr := DecodeResult{
				Overrides: variables.Overrides{
					"p2": []*variables.Override{{Name: "o1"}, {Name: "o2"}},
				},
			}
			d2 := DecodeResult{
				Overrides: variables.Overrides{},
			}
			dr.Merge(d2)
			must.False(t, dr.Diags.HasErrors())
			must.Len[*variables.Override](t, 2, dr.Overrides["p2"])
		})

		t.Run("for nil override pointer", func(t *testing.T) {

			d1 := DecodeResult{
				Overrides: variables.Overrides{
					"p1": []*variables.Override{{Name: "o1"}, {Name: "o2"}},
				},
			}
			var nilPtr *variables.Override
			d := DecodeResult{
				Overrides: variables.Overrides{
					"p1": []*variables.Override{nilPtr},
				},
			}

			d1.Merge(d)
			must.False(t, d1.Diags.HasErrors())
			must.Len(t, 2, d1.Overrides["p1"])
		})
	})
}
