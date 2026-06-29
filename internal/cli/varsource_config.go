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
//   - consul:///path                  (uses the Consul environment address)
//   - consul://host:port/path         (uses the specified Consul address)
//   - vault:///mount/path             (uses the Vault environment address)
//   - vault://host:port/mount/path    (uses the specified Vault address)
//   - nomad:///path                   (uses the Nomad environment address)
//   - nomad://host:port/path          (uses the specified Nomad address)
func parseVarSourceConfigs(urls []string) ([]source.SourceConfig, error) {
	if len(urls) == 0 {
		return nil, nil
	}

	configs := make([]source.SourceConfig, 0, len(urls))

	for i, urlStr := range urls {
		// Each source's priority is its position on the command line, so a later
		// --var-source overrides an earlier one for the same variable. The base
		// keeps every external source below local input (env, files, CLI flags).
		priority := source.PriorityExternalBase + i
		cfg, err := parseVarSourceConfig(urlStr, priority)
		if err != nil {
			return nil, fmt.Errorf("var-source %q: %w", urlStr, err)
		}
		configs = append(configs, cfg)
	}

	return configs, nil
}

// parseVarSourceConfig parses a single variable source URL into a typed config.
// priority is the precedence assigned to the built source.
func parseVarSourceConfig(urlStr string, priority int) (source.SourceConfig, error) {
	u, err := url.Parse(urlStr)
	if err != nil {
		return nil, fmt.Errorf("invalid URL: %w", err)
	}

	host := u.Host
	path := strings.Trim(u.Path, "/")

	switch u.Scheme {
	case "consul":
		return parseConsulSourceConfig(host, path, priority)
	case "vault":
		return parseVaultSourceConfig(host, path, priority)
	case "nomad":
		return parseNomadSourceConfig(host, path, priority)
	default:
		return nil, fmt.Errorf("unsupported scheme %q (supported: consul, vault, nomad)", u.Scheme)
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
func parseConsulSourceConfig(host, path string, priority int) (source.SourceConfig, error) {
	if path == "" {
		return nil, fmt.Errorf("consul URL must include a path (e.g., consul:///nomad-pack)")
	}

	return source.ConsulSourceConfig{
		Priority: priority,
		Address:  host,
		Path:     path,
	}, nil
}

// parseVaultSourceConfig builds a Vault KV v2 source config from the host and
// path of a vault:// URL. The first path segment is the KV v2 mount point and
// the remainder is the secret path; all variables for the pack are stored as
// fields of the single secret at <mount>/<path>. A non-empty host overrides the
// Vault environment address; the rest of the Vault configuration, including the
// token, comes from the standard Vault environment configuration (VAULT_ADDR,
// VAULT_TOKEN, and so on) when the source is built.
//   - vault:///secret/path/to/vars        -> host="",               mount="secret", path="path/to/vars"
//   - vault://localhost:8200/secret/path  -> host="localhost:8200", mount="secret", path="path"
func parseVaultSourceConfig(host, path string, priority int) (source.SourceConfig, error) {
	mount, secretPath, ok := strings.Cut(path, "/")
	if !ok || mount == "" || secretPath == "" {
		return nil, fmt.Errorf("vault URL must include a mount and path (e.g., vault:///secret/nomad-pack)")
	}

	return source.VaultSourceConfig{
		Priority: priority,
		Address:  host,
		Mount:    mount,
		Path:     secretPath,
	}, nil
}

// parseNomadSourceConfig builds a Nomad Variables source config from the host
// and path of a nomad:// URL. The path is the Nomad Variable path; all variables
// for the pack are stored as items of the single Nomad Variable at <path>. A
// non-empty host overrides the Nomad environment address; the rest of the Nomad
// configuration, including the token and namespace, comes from the standard
// Nomad environment configuration (NOMAD_ADDR, NOMAD_TOKEN, NOMAD_NAMESPACE, and
// so on) when the source is built.
//   - nomad:///path/to/vars        -> host="",               path="path/to/vars"
//   - nomad://localhost:4646/path  -> host="localhost:4646", path="path"
func parseNomadSourceConfig(host, path string, priority int) (source.SourceConfig, error) {
	if path == "" {
		return nil, fmt.Errorf("nomad URL must include a path (e.g., nomad:///nomad-pack)")
	}

	return source.NomadSourceConfig{
		Priority: priority,
		Address:  host,
		Path:     path,
	}, nil
}
