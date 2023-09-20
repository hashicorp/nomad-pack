package varfile

import (
	"testing"

	"github.com/davecgh/go-spew/spew"
	"github.com/hashicorp/hcl/v2"
	"github.com/shoenig/test/must"
	"github.com/zclconf/go-cty/cty"
)

func TestVarfile_DecodeHCL(t *testing.T) {
	type exp struct {
		dLen  int
		diags hcl.Diagnostics
		oLen  int
		oMap  Overrides
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
				oMap: Overrides{
					"embedded.hcl": []*Override{
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
			tc := tc
			om := make(Overrides)
			_, diags := Decode("embedded.hcl", tc.src, nil, &om)
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

func TestVarfile_DecodeResult_Merge(t *testing.T) {
	d1 := DecodeResult{
		Overrides: Overrides{
			"p1": []*Override{{Name: "o1"}, {Name: "o2"}},
		},
	}
	t.Run("errors when redefined", func(t *testing.T) {
		dr := DecodeResult{
			Overrides: Overrides{
				"p1": []*Override{{Name: "o1"}},
			},
		}

		dr.Merge(d1)
		must.True(t, dr.Diags.HasErrors())
		must.ErrorContains(t, dr.Diags, "variable o1 can not be redefined")
	})
}

func TestVarFile_Merge_Good(t *testing.T) {
	d1 := DecodeResult{
		Overrides: Overrides{
			"p1": []*Override{{Name: "o1"}, {Name: "o2"}},
		},
	}

	t.Run("succeeds for", func(t *testing.T) {
		t.Run("okay for variables of same name in different pack", func(t *testing.T) {
			dr := DecodeResult{
				Overrides: Overrides{
					"p2": []*Override{{Name: "o1"}, {Name: "o2"}},
				},
			}
			dr.Merge(d1)
			must.False(t, dr.Diags.HasErrors())
			must.Len(t, 2, dr.Overrides["p1"])
			must.Len(t, 2, dr.Overrides["p2"])
		})

		t.Run("okay for repeated pointers to same override", func(t *testing.T) {
			dr := DecodeResult{
				Overrides: Overrides{
					"p2": []*Override{{Name: "o1"}, {Name: "o2"}},
				},
			}
			dr2 := dr
			dr2.Merge(dr)
			must.False(t, dr.Diags.HasErrors())
			must.Len(t, 2, dr.Overrides["p2"])
		})

		t.Run("for nil overrides", func(t *testing.T) {
			dr := DecodeResult{
				Overrides: Overrides{
					"p2": []*Override{{Name: "o1"}, {Name: "o2"}},
				},
			}
			d2 := DecodeResult{
				Overrides: Overrides{},
			}
			dr.Merge(d2)
			must.False(t, dr.Diags.HasErrors())
			must.Len(t, 2, dr.Overrides["p2"])
			must.MapNotContainsKey(t, d2.Overrides, "p1")
		})

		//TODO: Investigatethis broken test.
		// t.Run("for nil override pointer", func(t *testing.T) {

		// 	ov := &Override{
		// 		Name:  "datacenter",
		// 		Path:  "simple_raw_exec_1",
		// 		Type:  cty.List(cty.String),
		// 		Value: cty.ListValEmpty(cty.String),
		// 	}

		// 	dr := DecodeResult{Overrides: Overrides{"p1": []*Override{ov, ov}}}

		// 	dr.Merge(dr)
		// 	must.False(t, dr.Diags.HasErrors())
		// 	must.Len(t, 1, dr.Overrides["p1"])
		// })
	})
}
