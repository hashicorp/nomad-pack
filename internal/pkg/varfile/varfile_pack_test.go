package varfile_test

import (
	"bytes"
	"os"
	"strings"
	"testing"

	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/nomad-pack/internal/pkg/varfile"
	"github.com/hashicorp/nomad-pack/internal/pkg/varfile/fixture"
	"github.com/hashicorp/nomad-pack/sdk/pack"
	"github.com/hashicorp/nomad-pack/sdk/pack/variables"
	"github.com/shoenig/test/must"
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

func TestVarfile_ProcessPackVarfiles(t *testing.T) {
	root := testpack("mypack")
	ovrds := make(variables.Overrides)
	fm, d := varfile.Decode(root, "foo.hcl", []byte(`foo="bar"`), nil, &ovrds)
	if d.HasErrors() {
		dw := hcl.NewDiagnosticTextWriter(os.Stderr, fm, 40, false)
		t.Log(dw.WriteDiagnostics(d))
		t.FailNow()
	}
	must.Len[*variables.Override](t, 1, ovrds["foo.hcl"])
	must.Eq(t, "bar", ovrds["foo.hcl"][0].Value.AsString())
}

func TestVarfile_DecodeVariableOverrides(t *testing.T) {
	root := testpack("mypack")
	dr := varfile.DecodeVariableOverrides(root, fixture.JSONFiles["mypack"])
	must.NotNil(t, dr.Diags)
	must.Len(t, 4, dr.Diags)
	var b bytes.Buffer
	dw := hcl.NewDiagnosticTextWriter(&b, dr.HCLFiles, 80, false)
	for _, d := range dr.Diags {
		dw.WriteDiagnostics(hcl.Diagnostics{d})
		must.StrHasPrefix(t, "Error:", b.String())
		b.Reset()
	}
	dw.WriteDiagnostics(dr.Diags)
}
