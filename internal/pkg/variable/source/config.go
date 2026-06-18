// Copyright IBM Corp. 2023, 2026
// SPDX-License-Identifier: MPL-2.0

package source

import (
	"github.com/hashicorp/consul/api"
)

// SourceConfig is a parsed, lazily-evaluated configuration for a variable
// source. Parsing a config (for example from a --var-source URL) is kept
// separate from Build, which constructs the concrete VariableSource. Build does
// only local work, such as building API client structs; it opens no connection.
// No network I/O happens until variables are actually fetched for rendering, so
// callers can parse and validate source configuration up front for free.
type SourceConfig interface {
	// Build constructs the concrete VariableSource described by this config.
	// Implementations may construct API clients here, but constructing a client
	// does not open a connection; no remote reads happen until
	// VariableSource.Fetch.
	Build() (VariableSource, error)
}

// ConsulSourceConfig holds the parsed configuration for a Consul KV variable
// source. It is a plain value type with no live connections, making it safe to
// pass across package boundaries without import cycles.
type ConsulSourceConfig struct {
	// Priority is the precedence level applied to the built source.
	Priority int

	// Address is the Consul HTTP address. When empty, the address from the
	// standard Consul environment configuration is used.
	Address string

	// Path is the Consul KV path under which variables are stored. Each
	// variable is read from <Path>/<var-name>.
	Path string
}

// Build implements SourceConfig by constructing a ConsulSource. It builds the
// Consul API client struct but opens no connection and performs no remote
// reads; those happen later in Fetch.
func (c ConsulSourceConfig) Build() (VariableSource, error) {
	apiCfg := api.DefaultConfig()
	if c.Address != "" {
		apiCfg.Address = c.Address
	}

	return NewConsulSource(c.Priority, apiCfg, c.Path)
}
