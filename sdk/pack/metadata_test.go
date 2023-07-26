// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package pack

import (
	"testing"

	"github.com/shoenig/test/must"
)

func TestMetadata_ConvertToMapInterface(t *testing.T) {
	testCases := []struct {
		name           string
		inputMetadata  *Metadata
		expectedOutput map[string]any
	}{
		{
			name: "all metadata values populated",
			inputMetadata: &Metadata{
				App: &MetadataApp{
					URL: "https://example.com",
				},
				Pack: &MetadataPack{
					Name:        "Example",
					Description: "The most basic, yet awesome, example",
					Version:     "v0.0.1",
				},
				Integration: &MetadataIntegration{
					Name:       "Example",
					Identifier: "nomad/hashicorp/example",
					Flags:      []string{"foo", "bar"},
				},
			},
			expectedOutput: map[string]any{
				"nomad_pack": map[string]any{
					"app": map[string]any{
						"url": "https://example.com",
					},
					"pack": map[string]any{
						"name":        "Example",
						"description": "The most basic, yet awesome, example",
						"version":     "v0.0.1",
					},
					"integration": map[string]any{
						"identifier": "nomad/hashicorp/example",
						"flags":      []string{"foo", "bar"},
						"name":       "Example",
					},
					"dependencies": []map[string]any{},
				},
			},
		},
		{
			name: "all metadata values with deps",
			inputMetadata: &Metadata{
				App: &MetadataApp{
					URL: "https://example.com",
				},
				Pack: &MetadataPack{
					Name:        "Example",
					Description: "The most basic, yet awesome, example",
					Version:     "v0.0.1",
				},
				Integration: &MetadataIntegration{
					Name:       "Example",
					Identifier: "nomad/hashicorp/example",
					Flags:      []string{"foo", "bar"},
				},
				Dependencies: []*Dependency{
					{
						Name:    "dep1",
						Enabled: pointerOf(true),
					},
					{
						Name:    "dep1",
						Alias:   "dep2",
						Enabled: pointerOf(true),
					},
				},
			},
			expectedOutput: map[string]any{
				"nomad_pack": map[string]any{
					"app": map[string]any{
						"url": "https://example.com",
					},
					"pack": map[string]any{
						"name":        "Example",
						"description": "The most basic, yet awesome, example",
						"version":     "v0.0.1",
					},
					"integration": map[string]any{
						"identifier": "nomad/hashicorp/example",
						"flags":      []string{"foo", "bar"},
						"name":       "Example",
					},
					"dependencies": []map[string]any{
						{
							"dep1": map[string]any{
								"name":    "dep1",
								"alias":   "",
								"source":  "",
								"enabled": pointerOf(true),
							},
						},
						{
							"dep2": map[string]any{
								"name":    "dep1",
								"alias":   "dep2",
								"source":  "",
								"enabled": pointerOf(true),
							},
						},
					},
				},
			},
		},
		{
			// TODO test added to cover graceful failure while we're in the process of
			// retiring "Author" and "URL" metadata fields. Can be removed in the future.
			name: "author and url fields ignored gracefully",
			inputMetadata: &Metadata{
				App: &MetadataApp{
					URL: "https://example.com",
				},
				Pack: &MetadataPack{
					Name:    "Example",
					URL:     "https://example.com",
					Version: "v0.0.1",
				},
				Integration: &MetadataIntegration{},
			},
			expectedOutput: map[string]any{
				"nomad_pack": map[string]any{
					"app": map[string]any{
						"url": "https://example.com",
					},
					"pack": map[string]any{
						"name":        "Example",
						"description": "",
						"version":     "v0.0.1",
					},
					"integration": map[string]any{
						"identifier": "",
						"flags":      []string(nil),
						"name":       "",
					},
					"dependencies": []map[string]any{},
				},
			},
		},
		{
			name: "some metadata values populated",
			inputMetadata: &Metadata{
				App: &MetadataApp{
					URL:    "https://example.com",
					Author: "The Nomad Team",
				},
				Pack: &MetadataPack{
					URL: "https://example.com",
				},
				Integration: &MetadataIntegration{},
			},
			expectedOutput: map[string]any{
				"nomad_pack": map[string]any{
					"app":          map[string]any{"url": "https://example.com"},
					"pack":         map[string]any{"name": "", "description": "", "version": ""},
					"integration":  map[string]any{"identifier": "", "flags": []string(nil), "name": ""},
					"dependencies": []map[string]any{},
				},
			},
		},
	}

	for _, tc := range testCases {
		actualOutput := tc.inputMetadata.ConvertToMapInterface()
		must.Eq(t, tc.expectedOutput, actualOutput, must.Sprint(tc.name))
	}
}

func TestMetadata_Validate(t *testing.T) {
	testCases := []struct {
		inputMetadata *Metadata
		expectError   bool
		name          string
	}{
		{
			inputMetadata: &Metadata{
				App: &MetadataApp{
					URL: "https://example.com",
				},
				Pack: &MetadataPack{
					Name:        "Example",
					Description: "The most basic, yet awesome, example",
				},
				Integration: &MetadataIntegration{},
			},
			expectError: false,
			name:        "valid metadata",
		},
		{
			inputMetadata: nil,
			expectError:   true,
			name:          "nil guard",
		},
	}

	for _, tc := range testCases {
		err := tc.inputMetadata.Validate()
		if tc.expectError {
			must.NotNil(t, err, must.Sprint(tc.name))
		} else {
			must.Nil(t, err, must.Sprint(tc.name))
		}
	}
}
