package job

import (
	"testing"

	"github.com/shoenig/test/must"
)

func TestExtractJobRegionNamespace(t *testing.T) {
	tests := []struct {
		name    string
		hcl     string
		wantReg string
		wantNS  string
	}{
		{
			name: "both region and namespace present",
			hcl: `
					job "foo" {
  					region    = "us"
  					namespace = "dev"
			}`,
			wantReg: "us",
			wantNS:  "dev",
		},
		{
			name: "only region",
			hcl: `
					job "foo" {
  					region = "eu"
			}`,
			wantReg: "eu",
			wantNS:  "",
		},
		{
			name: "only namespace",
			hcl: `
					job "foo" {
  					namespace = "prod"
			}`,
			wantReg: "",
			wantNS:  "prod",
		},
		{
			name: "neither present",
			hcl: `
					job "foo" {
  					group "bar" {
    				task "baz" {}
  					}
			}`,
			wantReg: "",
			wantNS:  "",
		},
		{
			name: "region and namespace as pack variables should not be picked up",
			hcl: `
					variable "region" {
  					  default = "should_not_pickup"
					}
					variable "namespace" {
  					  default = "should_not_pickup"
					}
					job "foo" {}`,
			wantReg: "",
			wantNS:  "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			reg, ns, err := ExtractJobRegionNamespace(tt.hcl)
			must.NoError(t, err)
			must.Eq(t, tt.wantReg, reg)
			must.Eq(t, tt.wantNS, ns)
		})
	}
}
