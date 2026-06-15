// Copyright IBM Corp. 2023, 2026
// SPDX-License-Identifier: MPL-2.0

package cli

import (
	"fmt"
	"net/url"
	"strings"

	"github.com/hashicorp/consul/api"
)

// VarSourceConfig represents parsed configuration for an external variable source.
// This is a lightweight struct that only holds configuration, not actual connections.
type VarSourceConfig struct {
	Type   string // "consul", "vault", "nomad"
	Config any    // Type-specific configuration
}

// ConsulSourceConfig holds configuration for a Consul KV variable source.
type ConsulSourceConfig struct {
	Address       string // Consul address (from URL or env)
	Token         string // Consul token (from URL query or env)
	Path          string // KV path (can include pack-id or be full path)
	IncludePackID bool   // If true, append /{pack-id}/ to path
}

// parseVarSourceConfigs parses variable source URLs into configuration structs.
// Supported URL formats:
//   - consul:///prefix  (uses default Consul address from env)
//   - consul://host:port/prefix  (uses specified Consul address)
//
// Examples:
//   - consul:///nomad-pack
//   - consul:///config
//   - consul://localhost:8500/config
func parseVarSourceConfigs(urls []string) ([]VarSourceConfig, error) {
	if len(urls) == 0 {
		return nil, nil
	}

	configs := make([]VarSourceConfig, 0, len(urls))

	for _, urlStr := range urls {
		cfg, err := parseVarSourceConfig(urlStr)
		if err != nil {
			return nil, fmt.Errorf("failed to parse var-source %q: %w", urlStr, err)
		}
		configs = append(configs, cfg)
	}

	return configs, nil
}

// parseVarSourceConfig parses a single variable source URL into configuration.
func parseVarSourceConfig(urlStr string) (VarSourceConfig, error) {
	// Parse the URL
	u, err := url.Parse(urlStr)
	if err != nil {
		return VarSourceConfig{}, fmt.Errorf("invalid URL: %w", err)
	}

	// Determine source type from scheme
	switch u.Scheme {
	case "consul":
		cfg, err := parseConsulSourceConfig(u)
		if err != nil {
			return VarSourceConfig{}, err
		}
		return VarSourceConfig{
			Type:   "consul",
			Config: cfg,
		}, nil
	default:
		return VarSourceConfig{}, fmt.Errorf("unsupported scheme %q (supported: consul)", u.Scheme)
	}
}

// parseConsulSourceConfig creates configuration from a consul:// URL.
//
// URL format follows standard URL rules:
//   - consul:///path/to/vars        -> path="/path/to/vars", address=from env (note 3 slashes)
//   - consul://localhost:8500/path  -> path="/path", address="localhost:8500"
//
// By default, pack-id is appended to the path: <path>/<pack-id>/<variable-name>
// Use ?full-path=true to use the path as-is without appending pack-id.
//
// Examples:
//   - consul:///nomad-pack                    -> nomad-pack/{pack-id}/{var-name}
//   - consul:///my/custom/path?full-path=true -> my/custom/path/{var-name}
func parseConsulSourceConfig(u *url.URL) (ConsulSourceConfig, error) {
	cfg := ConsulSourceConfig{}

	// Get default address from environment
	defaultConfig := api.DefaultConfig()
	cfg.Address = defaultConfig.Address
	cfg.Token = defaultConfig.Token

	// If host is specified, use it as the Consul address
	if u.Host != "" {
		cfg.Address = u.Host
	}

	// The path is the KV path
	path := strings.Trim(u.Path, "/")
	if path == "" {
		return cfg, fmt.Errorf("consul URL must include a path (e.g., consul:///nomad-pack)")
	}

	cfg.Path = path

	// Parse query parameters for additional config
	query := u.Query()
	if token := query.Get("token"); token != "" {
		cfg.Token = token
	}

	// Check if user wants full path control (no pack-id appended)
	if query.Get("full-path") == "true" {
		cfg.IncludePackID = false
	} else {
		// Default: append pack-id to path
		cfg.IncludePackID = true
	}

	return cfg, nil
}
