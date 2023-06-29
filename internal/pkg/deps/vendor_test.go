package deps

import (
	"context"
	"testing"

	"github.com/hashicorp/nomad-pack/internal/pkg/cache"
	"github.com/hashicorp/nomad-pack/terminal"
)

func TestVendor(t *testing.T) {
	type args struct {
		ctx         context.Context
		ui          terminal.UI
		globalCache *cache.Cache
		copyToCache bool
		targetPath  string
	}
	tests := []struct {
		name    string
		args    args
		wantErr bool
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := Vendor(tt.args.ctx, tt.args.ui, tt.args.globalCache, tt.args.copyToCache, tt.args.targetPath); (err != nil) != tt.wantErr {
				t.Errorf("Vendor() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
