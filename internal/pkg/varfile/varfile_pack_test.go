package varfile_test

import (
	"fmt"
	"os"
	"testing"

	"github.com/davecgh/go-spew/spew"
	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/nomad-pack/internal/pkg/varfile"
	"github.com/hashicorp/nomad-pack/internal/pkg/varfile/fixture"
	"github.com/shoenig/test/must"
)

func TestVarfile_ProcessPackVarfiles(t *testing.T) {
	fmt.Println("Testing")
	ovrds := make(varfile.Overrides)
	fm, d := varfile.Decode("foo.hcl", []byte(`foo="bar"`), nil, &ovrds)
	if d.HasErrors() {
		dw := hcl.NewDiagnosticTextWriter(os.Stderr, fm, 40, false)
		t.Log(dw.WriteDiagnostics(d))
		t.FailNow()
	}
	spew.Dump(ovrds)
	must.Len(t, 1, ovrds["foo.hcl"])
	must.Eq(t, "bar", ovrds["foo.hcl"][0].Value.AsString())
}

func TestVarfile_DecodeVariableOverrides(t *testing.T) {
	dr := varfile.DecodeVariableOverrides(fixture.JSONFiles["myPack"])
	spew.Dump(dr)
	dw := hcl.NewDiagnosticTextWriter(os.Stderr, dr.HCLFiles, 80, false)
	dw.WriteDiagnostics(dr.Diags)
}
