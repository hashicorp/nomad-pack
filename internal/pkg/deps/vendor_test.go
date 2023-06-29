package deps

import "testing"

func TestVendor(t *testing.T) {
	tests := []struct {
		name    string
		wantErr bool
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := Vendor(); (err != nil) != tt.wantErr {
				t.Errorf("Vendor() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func Test_sourceToPath(t *testing.T) {
	tests := []struct {
		name    string
		source  string
		want    string
		wantErr bool
	}{
		{
			"git path",
			"git://github.com/hashicorp/hello_world_pack.git",
			"hello_world_pack",
			false,
		},
		{
			"relative path, single dir",
			"./relative_path",
			"",
			true,
		},
		{
			"relative path, multiple dirs",
			"./relative_path/relative_nested_path",
			"",
			true,
		},
		{
			"normal url",
			"https://s3.amazonaws.com/pack",
			"pack",
			false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := sourceToPath(tt.source)
			if (err != nil) != tt.wantErr {
				t.Errorf("sourceToPath() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("sourceToPath() = %v, want %v", got, tt.want)
			}
		})
	}
}
