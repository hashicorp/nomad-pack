// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package cli

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestConsulKVConfig_LoadFromEnv(t *testing.T) {
	tests := []struct {
		name     string
		envVars  map[string]string
		initial  ConsulKVConfig
		expected ConsulKVConfig
	}{
		{
			name: "load all from environment",
			envVars: map[string]string{
				"CONSUL_HTTP_ADDR":       "https://consul.example.com:8501",
				"CONSUL_HTTP_TOKEN":      "test-token",
				"CONSUL_NAMESPACE":       "test-ns",
				"CONSUL_CACERT":          "/path/to/ca.pem",
				"CONSUL_CLIENT_CERT":     "/path/to/client.pem",
				"CONSUL_CLIENT_KEY":      "/path/to/client-key.pem",
				"CONSUL_TLS_SKIP_VERIFY": "true",
				"CONSUL_TLS_SERVER_NAME": "consul.example.com",
			},
			initial: ConsulKVConfig{},
			expected: ConsulKVConfig{
				Address:       "https://consul.example.com:8501",
				Token:         "test-token",
				Namespace:     "test-ns",
				CACert:        "/path/to/ca.pem",
				ClientCert:    "/path/to/client.pem",
				ClientKey:     "/path/to/client-key.pem",
				TLSSkipVerify: true,
				TLSServerName: "consul.example.com",
			},
		},
		{
			name: "CLI flags override environment variables",
			envVars: map[string]string{
				"CONSUL_HTTP_ADDR":  "https://env.example.com:8501",
				"CONSUL_HTTP_TOKEN": "env-token",
				"CONSUL_CACERT":     "/env/ca.pem",
			},
			initial: ConsulKVConfig{
				Address: "https://flag.example.com:8501",
				Token:   "flag-token",
			},
			expected: ConsulKVConfig{
				Address: "https://flag.example.com:8501",
				Token:   "flag-token",
				CACert:  "/env/ca.pem", // Only this comes from env
			},
		},
		{
			name:    "empty environment uses defaults",
			envVars: map[string]string{},
			initial: ConsulKVConfig{
				Address: "https://flag.example.com:8501",
			},
			expected: ConsulKVConfig{
				Address: "https://flag.example.com:8501",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Set environment variables
			for k, v := range tt.envVars {
				os.Setenv(k, v)
				defer os.Unsetenv(k)
			}

			config := tt.initial
			config.LoadFromEnv()

			assert.Equal(t, tt.expected.Address, config.Address)
			assert.Equal(t, tt.expected.Token, config.Token)
			assert.Equal(t, tt.expected.Namespace, config.Namespace)
			assert.Equal(t, tt.expected.CACert, config.CACert)
			assert.Equal(t, tt.expected.ClientCert, config.ClientCert)
			assert.Equal(t, tt.expected.ClientKey, config.ClientKey)
			assert.Equal(t, tt.expected.TLSSkipVerify, config.TLSSkipVerify)
			assert.Equal(t, tt.expected.TLSServerName, config.TLSServerName)
		})
	}
}

func TestConsulKVConfig_NewConsulClient(t *testing.T) {
	tests := []struct {
		name   string
		config ConsulKVConfig
	}{
		{
			name: "basic config without TLS",
			config: ConsulKVConfig{
				Address: "http://localhost:8500",
			},
		},
		{
			name: "config with TLS skip verify",
			config: ConsulKVConfig{
				Address:       "https://localhost:8501",
				TLSSkipVerify: true,
			},
		},
		{
			name: "config with token",
			config: ConsulKVConfig{
				Address: "http://localhost:8500",
				Token:   "test-token",
			},
		},
		{
			name: "config with namespace",
			config: ConsulKVConfig{
				Address:   "http://localhost:8500",
				Namespace: "test-ns",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client, err := tt.config.NewConsulClient()
			require.NoError(t, err)
			assert.NotNil(t, client)
		})
	}
}
