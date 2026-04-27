// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package cli

import (
	"os"
	"testing"

	"github.com/shoenig/test/must"
)

func TestConsulKVConfig_NewConsulClient(t *testing.T) {
	tests := []struct {
		name      string
		envVars   map[string]string
		config    ConsulKVConfig
		wantAddr  string
		wantToken string
		wantErr   bool
	}{
		{
			name: "CLI flags override environment variables",
			envVars: map[string]string{
				"CONSUL_HTTP_ADDR":  "https://env.example.com:8501",
				"CONSUL_HTTP_TOKEN": "env-token",
			},
			config: ConsulKVConfig{
				Address: "https://cli.example.com:8501",
				Token:   "cli-token",
			},
			wantAddr:  "https://cli.example.com:8501",
			wantToken: "cli-token",
			wantErr:   false,
		},
		{
			name: "uses environment variables when CLI flags not set",
			envVars: map[string]string{
				"CONSUL_HTTP_ADDR":  "https://env.example.com:8501",
				"CONSUL_HTTP_TOKEN": "env-token",
			},
			config:    ConsulKVConfig{},
			wantAddr:  "https://env.example.com:8501",
			wantToken: "env-token",
			wantErr:   false,
		},
		{
			name:      "uses defaults when nothing configured",
			envVars:   map[string]string{},
			config:    ConsulKVConfig{},
			wantAddr:  "127.0.0.1:8500", // Consul SDK default
			wantToken: "",
			wantErr:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Set environment variables
			for k, v := range tt.envVars {
				os.Setenv(k, v)
				defer os.Unsetenv(k)
			}

			client, err := tt.config.NewConsulClient()
			if tt.wantErr {
				must.Error(t, err)
				return
			}

			must.NoError(t, err)
			must.NotNil(t, client)

		})
	}
}

func TestGetConsulClient(t *testing.T) {
	tests := []struct {
		name       string
		envVars    map[string]string
		config     ConsulKVConfig
		wantClient bool
		wantErr    bool
	}{
		{
			name: "creates client when address provided via CLI",
			config: ConsulKVConfig{
				Address: "https://consul.example.com:8501",
			},
			wantClient: true,
			wantErr:    false,
		},
		{
			name: "creates client when address in environment",
			envVars: map[string]string{
				"CONSUL_HTTP_ADDR": "https://consul.example.com:8501",
			},
			config:     ConsulKVConfig{},
			wantClient: true,
			wantErr:    false,
		},
		{
			name:       "returns nil when no Consul configured",
			envVars:    map[string]string{},
			config:     ConsulKVConfig{},
			wantClient: false,
			wantErr:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Set environment variables
			for k, v := range tt.envVars {
				os.Setenv(k, v)
				defer os.Unsetenv(k)
			}

			client, err := getConsulClient(&tt.config, nil, nil)
			if tt.wantErr {
				must.Error(t, err)
				return
			}

			must.NoError(t, err)
			if tt.wantClient {
				must.NotNil(t, client)
			} else {
				must.Nil(t, client)
			}

		})
	}
}
