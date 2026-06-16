// Copyright IBM Corp. 2023, 2026
// SPDX-License-Identifier: MPL-2.0

package source

import (
	"github.com/hashicorp/consul/api"
)

// SourceConfig is a parsed, lazily-evaluated configuration for a variable
// source. Parsing a config (for example from a --var-source URL) is kept
// separate from Build, which constructs the concrete VariableSource and may
// open remote clients. This separation lets callers parse and validate source
// configuration without performing any network I/O until variables are
// actually needed for rendering.
type SourceConfig interface {
	// Build constructs the concrete VariableSource described by this config.
	// Implementations may create API clients here, but must not perform
	// remote reads; reads happen in VariableSource.Fetch.
	Build() (VariableSource, error)
}

// ConsulSourceConfig holds the parsed configuration for a Consul KV variable
// source. It is a plain value type with no live connections, making it safe to
// pass across package boundaries without import cycles or guessing.
type ConsulSourceConfig struct {
	// Priority is the precedence level applied to the built source.
	Priority int

	// Address is the Consul HTTP address. When empty, the address from the
	// standard Consul environment configuration is used.
	Address string

	// Token is the Consul ACL token. When empty, the token from the standard
	// Consul environment configuration is used.
	Token string

	// Path is the Consul KV prefix under which variables are stored.
	Path string

	// IncludePackID controls whether the pack ID is appended to Path when
	// building the lookup key (<Path>/<pack-id>/<var-name>). When false, the
	// user-supplied Path is used verbatim (<Path>/<var-name>).
	IncludePackID bool
}

// Build implements SourceConfig by constructing a ConsulSource. It creates the
// Consul API client but performs no remote reads.
func (c ConsulSourceConfig) Build() (VariableSource, error) {
	apiCfg := api.DefaultConfig()
	if c.Address != "" {
		apiCfg.Address = c.Address
	}
	if c.Token != "" {
		apiCfg.Token = c.Token
	}

	return NewConsulSource(c.Priority, apiCfg, c.Path, c.IncludePackID)
}
