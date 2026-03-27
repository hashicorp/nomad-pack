// Copyright IBM Corp. 2023, 2026
// SPDX-License-Identifier: MPL-2.0

package variables

import (
	"github.com/zclconf/go-cty/cty"
)

// NomadVariable represents a Nomad Variable
// that should be created during pack deployment
type NomadVariable struct {

	// Name is the label from the nomad_variable block
	Name string

	// Path is where to store variable in Nomad
	Path string

	// Namespace is the Nomad namespace
	Namespace string

	// Items are key-value pairs to store
	Items map[string]cty.Value
}
