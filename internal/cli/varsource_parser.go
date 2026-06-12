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

// parseVarSources parses variable source URLs and creates VariableSource instances.
// Supported URL formats:
//   - consul://prefix/pack-id  (fetches from Consul KV at prefix/pack-id/*)
//   - consul://prefix          (uses pack name from command)
//
// Examples:
//   - consul://nomad-pack/myapp
//   - consul://config
func parseVarSources(urls []string, packName string) ([]source.VariableSource, error) {
	if len(urls) == 0 {
		return nil, nil
	}

	sources := make([]source.VariableSource, 0, len(urls))

	for _, urlStr := range urls {
		src, err := parseVarSource(urlStr, packName)
		if err != nil {
			return nil, fmt.Errorf("failed to parse var-source %q: %w", urlStr, err)
		}
		sources = append(sources, src)
	}

	return sources, nil
}

// parseVarSource parses a single variable source URL.
func parseVarSource(urlStr string, packName string) (source.VariableSource, error) {
	// Parse the URL
	u, err := url.Parse(urlStr)
	if err != nil {
		return nil, fmt.Errorf("invalid URL: %w", err)
	}

	// Determine source type from scheme
	switch u.Scheme {
	case "consul":
		return parseConsulSource(u, packName)
	default:
		return nil, fmt.Errorf("unsupported scheme %q (supported: consul)", u.Scheme)
	}
}

// parseConsulSource creates a ConsulSource from a consul:// URL.
// URL format: consul:///prefix[/pack-id] or consul://host/prefix[/pack-id]
//
// If pack-id is omitted, uses the pack name from the command.
// The prefix is where variables are stored in Consul KV.
//
// Examples:
//   - consul:///nomad-pack/myapp  -> prefix="nomad-pack/myapp", host=default
//   - consul://localhost:8500/config -> prefix="config", host="localhost:8500"
func parseConsulSource(u *url.URL, packName string) (source.VariableSource, error) {
	// Create Consul client configuration
	// This will read from CONSUL_HTTP_ADDR, CONSUL_HTTP_TOKEN env vars
	config := api.DefaultConfig()

	// Determine prefix path
	// If host is empty, treat host+path as the prefix (consul://nomad-pack/myapp)
	// If host is set, use path as prefix (consul://localhost:8500/nomad-pack/myapp)
	var prefix string

	if u.Host == "" {
		// No host specified - this shouldn't happen with consul://prefix format
		// but handle it gracefully
		path := strings.Trim(u.Path, "/")
		if path == "" {
			return nil, fmt.Errorf("consul URL must include a prefix path (e.g., consul:///nomad-pack)")
		}
		prefix = path
	} else {
		// Host is specified - could be actual host or the prefix
		// Check if it looks like a host (has port or is localhost/IP)
		if strings.Contains(u.Host, ":") || u.Host == "localhost" || strings.Contains(u.Host, ".") {
			// Looks like a real host
			config.Address = u.Host
			path := strings.Trim(u.Path, "/")
			if path == "" {
				return nil, fmt.Errorf("consul URL must include a prefix path after host")
			}
			prefix = path
		} else {
			// Host is actually the prefix (consul://nomad-pack/myapp)
			// When URL is consul://nomad-pack (no slashes after scheme),
			// Go's url.Parse treats "nomad-pack" as the host, not the path.
			// We detect this case and treat it as the prefix instead.
			// Combine host and path to get full prefix
			path := strings.Trim(u.Path, "/")
			if path != "" {
				prefix = u.Host + "/" + path
			} else {
				prefix = u.Host
			}
		}
	}

	// Parse query parameters for additional config
	query := u.Query()
	if token := query.Get("token"); token != "" {
		config.Token = token
	}

	// Create the Consul source with priority between file and CLI
	consulSource, err := source.NewConsulSource(source.PriorityConsul, config, prefix)
	if err != nil {
		return nil, fmt.Errorf("failed to create Consul source: %w", err)
	}

	return consulSource, nil
}
