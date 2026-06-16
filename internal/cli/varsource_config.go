// Copyright IBM Corp. 2023, 2026
// SPDX-License-Identifier: MPL-2.0

package cli

import (
	"fmt"
	"net/url"
	"strings"

	"github.com/hashicorp/consul/api"
	"github.com/hashicorp/nomad-pack/internal/pkg/variable/source"
)

// parseVarSourceConfigs parses variable source URLs into typed source configs.
// Only the configuration is parsed here; no remote connections are made. The
// returned configs are built into live sources lazily, at render time, by the
// variable parser.
//
// Supported URL formats:
//   - consul:///prefix              (uses default Consul address from env)
//   - consul://host:port/prefix     (uses the specified Consul address)
//
// See parseConsulSourceConfig for the full set of Consul options.
func parseVarSourceConfigs(urls []string) ([]source.SourceConfig, error) {
	if len(urls) == 0 {
		return nil, nil
	}

	configs := make([]source.SourceConfig, 0, len(urls))

	for _, urlStr := range urls {
		cfg, err := parseVarSourceConfig(urlStr)
		if err != nil {
			return nil, fmt.Errorf("failed to parse var-source %q: %w", urlStr, err)
		}
		configs = append(configs, cfg)
	}

	return configs, nil
}

// parseVarSourceConfig parses a single variable source URL into a typed config.
func parseVarSourceConfig(urlStr string) (source.SourceConfig, error) {
	u, err := url.Parse(urlStr)
	if err != nil {
		return nil, fmt.Errorf("invalid URL: %w", err)
	}

	switch u.Scheme {
	case "consul":
		return parseConsulSourceConfig(u)
	default:
		return nil, fmt.Errorf("unsupported scheme %q (supported: consul)", u.Scheme)
	}
}

// parseConsulSourceConfig creates configuration from a consul:// URL.
//
// The URL follows standard URL rules.
//   - consul:///path/to/vars        -> path="path/to/vars", address from env
//   - consul://localhost:8500/path  -> path="path", address="localhost:8500"
//
// By default the pack ID is appended to the path at fetch time:
// <path>/<pack-id>/<variable-name>. Pass ?full-path=true to use the path as-is
// without appending the pack ID. An optional ?token= overrides the ACL token.
//
// Examples:
//   - consul:///nomad-pack                    -> nomad-pack/{pack-id}/{var-name}
//   - consul:///my/custom/path?full-path=true -> my/custom/path/{var-name}
func parseConsulSourceConfig(u *url.URL) (source.SourceConfig, error) {
	cfg := source.ConsulSourceConfig{Priority: source.PriorityConsul}
	defaultConfig := api.DefaultConfig()
	cfg.Address = defaultConfig.Address
	cfg.Token = defaultConfig.Token

	// An explicit host in the URL overrides the environment address.
	if u.Host != "" {
		cfg.Address = u.Host
	}

	path := strings.Trim(u.Path, "/")
	if path == "" {
		return nil, fmt.Errorf("consul URL must include a path (e.g., consul:///nomad-pack)")
	}
	cfg.Path = path

	query := u.Query()
	if token := query.Get("token"); token != "" {
		cfg.Token = token
	}

	cfg.IncludePackID = query.Get("full-path") != "true"

	return cfg, nil
}
