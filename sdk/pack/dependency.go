package pack

import "github.com/hashicorp/nomad-pack/sdk/helper"

// Dependency is a single dependency of a pack. A pack can have multiple and
// each dependency represents an individual pack. A pack can be used as a
// dependency multiple times. This allows helper pack to define jobspec blocks
// which are used multiple times, with different variable substitutions.
type Dependency struct {

	// Name on the pack dependency which must match the MetadataPack.Name
	// value if the source is empty. Otherwise, the source dictates where the
	// pack is loaded from, allowing the same pack to be used multiple times as
	// a dependency with different variables.
	Name string `hcl:"name,label"`

	// Source is the remote source where the pack can be fetched. This string
	// can follow any format as supported by go-getter or be a local path
	// indicating the pack has already been downloaded.
	Source string `hcl:"source,optional"`

	// Enabled is a boolean flag to determine whether the dependency is
	// available for loading. This allows easy administrative control.
	Enabled *bool `hcl:"enabled,optional"`
}

// validate the Dependency object to ensure it meets requirements and doesn't
// contain invalid or incorrect data.
func (d *Dependency) validate() error {
	if d == nil {
		return nil
	}

	if d.Enabled == nil {
		d.Enabled = helper.BoolToPtr(true)
	}
	return nil
}
