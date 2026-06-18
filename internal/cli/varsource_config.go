// Copyright IBM Corp. 2023, 2026
// SPDX-License-Identifier: MPL-2.0

package cli

import (
	"fmt"
	"net/url"
	"strings"

	"github.com/hashicorp/nomad-pack/internal/pkg/variable/source"
)

// parseVarSourceConfigs parses variable source URLs into typed source configs.
// Only the configuration is parsed here; no remote connections are made. The
// returned configs are built into live sources lazily, at render time, by the
// variable parser.
//
// Supported URL formats:
//   - consul:///path              (uses the Consul environment address)
//   - consul://host:port/path     (uses the specified Consul address)
func parseVarSourceConfigs(urls []string) ([]source.SourceConfig, error) {
	if len(urls) == 0 {
		return nil, nil
	}

	configs := make([]source.SourceConfig, 0, len(urls))

	for _, urlStr := range urls {
		cfg, err := parseVarSourceConfig(urlStr)
		if err != nil {
			return nil, fmt.Errorf("var-source %q: %w", urlStr, err)
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

	host := u.Host
	path := strings.Trim(u.Path, "/")

	switch u.Scheme {
	case "consul":
		return parseConsulSourceConfig(host, path)
	default:
		return nil, fmt.Errorf("unsupported scheme %q (supported: consul)", u.Scheme)
	}
}

// parseConsulSourceConfig builds a Consul KV source config from the host and
// path of a consul:// URL. Each variable is read from <path>/<variable-name>.
// A non-empty host overrides the Consul environment address; the rest of the
// Consul configuration, including the ACL token, comes from the standard Consul
// environment configuration (CONSUL_HTTP_ADDR, CONSUL_HTTP_TOKEN, and so on)
// when the source is built.
//   - consul:///path/to/vars        -> host="",              path="path/to/vars"
//   - consul://localhost:8500/path  -> host="localhost:8500", path="path"
func parseConsulSourceConfig(host, path string) (source.SourceConfig, error) {
	if path == "" {
		return nil, fmt.Errorf("consul URL must include a path (e.g., consul:///nomad-pack)")
	}

	return source.ConsulSourceConfig{
		Priority: source.PriorityConsul,
		Address:  host,
		Path:     path,
	}, nil
}
