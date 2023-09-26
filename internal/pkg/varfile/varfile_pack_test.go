package varfile_test

import (
	"os"
	"testing"

	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/nomad-pack/internal/pkg/varfile"
	"github.com/hashicorp/nomad-pack/internal/pkg/varfile/fixture"
	"github.com/hashicorp/nomad-pack/sdk/pack/variables"
	"github.com/shoenig/test/must"
)

func TestVarfile_ProcessPackVarfiles(t *testing.T) {
	ovrds := make(variables.Overrides)
	fm, d := varfile.Decode("foo.hcl", []byte(`foo="bar"`), nil, &ovrds)
	if d.HasErrors() {
		dw := hcl.NewDiagnosticTextWriter(os.Stderr, fm, 40, false)
		t.Log(dw.WriteDiagnostics(d))
		t.FailNow()
	}
	must.Len[*variables.Override](t, 1, ovrds["foo.hcl"])
	must.Eq(t, "bar", ovrds["foo.hcl"][0].Value.AsString())
}

func TestVarfile_DecodeVariableOverrides(t *testing.T) {
	dr := varfile.DecodeVariableOverrides(fixture.JSONFiles["myPack"])
	dw := hcl.NewDiagnosticTextWriter(os.Stderr, dr.HCLFiles, 80, false)
	dw.WriteDiagnostics(dr.Diags)
}
