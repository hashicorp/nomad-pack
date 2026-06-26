// Copyright IBM Corp. 2023, 2026
// SPDX-License-Identifier: MPL-2.0

package source

import (
	"strings"

	"github.com/hashicorp/consul/api"
	nomadapi "github.com/hashicorp/nomad/api"
	vaultapi "github.com/hashicorp/vault/api"
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
	// must not open a connection; no remote reads happen until
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

// VaultSourceConfig holds the parsed configuration for a Vault KV v2 variable
// source.
type VaultSourceConfig struct {
	// Priority is the precedence level applied to the built source.
	Priority int

	// Address is the Vault HTTP address. When empty, the address from the
	// standard Vault environment configuration is used.
	Address string

	// Mount is the KV v2 engine mount point under which the secret lives.
	Mount string

	// Path is the secret path within the mount. All variables for the pack are
	// stored as fields of the single secret at <Mount>/<Path>.
	Path string
}

// Build implements SourceConfig by constructing a VaultSource.
func (v VaultSourceConfig) Build() (VariableSource, error) {
	apiCfg := vaultapi.DefaultConfig()
	if v.Address != "" {
		// Vault's API address must be a full URL (unlike Consul's host:port),
		// so default to https when the URL omits a scheme.
		addr := v.Address
		if !strings.Contains(addr, "://") {
			addr = "https://" + addr
		}
		apiCfg.Address = addr
	}

	return NewVaultSource(v.Priority, apiCfg, v.Mount, v.Path)
}

// NomadSourceConfig holds the parsed configuration for a Nomad Variables
// variable source.
type NomadSourceConfig struct {
	// Priority is the precedence level applied to the built source.
	Priority int

	// Address is the Nomad HTTP address. When empty, the address from the
	// standard Nomad environment configuration is used.
	Address string

	// Path is the Nomad Variable path. All variables for the pack are stored as
	// items of the single Nomad Variable at this path.
	Path string
}

// Build implements SourceConfig by constructing a NomadSource.
//
// When no host is supplied (the nomad:///path form), the address and every
// other client setting come entirely from the standard Nomad environment via
// nomadapi.DefaultConfig() — NOMAD_ADDR, NOMAD_TOKEN, NOMAD_NAMESPACE, and so
// on flow through unchanged. NOMAD_ADDR may point at a unix socket (unix://…),
// so running Pack inside a Nomad task and aiming it at the task's API socket
// needs no special handling here: the nomad/api client dials the socket
// natively and the workload's token authenticates as usual.
func (n NomadSourceConfig) Build() (VariableSource, error) {
	apiCfg := nomadapi.DefaultConfig()
	if n.Address != "" {
		// A host taken from the URL is always a TCP host:port. Nomad's API
		// address must be a full URL, so default to http when it omits a scheme —
		// matching Nomad's own default address (http://127.0.0.1:4646). Use the
		// nomad:///path form with NOMAD_ADDR for a unix socket or https.
		addr := n.Address
		if !strings.Contains(addr, "://") {
			addr = "http://" + addr
		}
		apiCfg.Address = addr
	}

	return NewNomadSource(n.Priority, apiCfg, n.Path)
}
