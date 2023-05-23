// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package pack

import (
	"testing"

	"github.com/shoenig/test/must"
)

func TestMetadata_ConvertToMapInterface(t *testing.T) {
	testCases := []struct {
		inputMetadata  *Metadata
		expectedOutput map[string]interface{}
		name           string
	}{
		{
			inputMetadata: &Metadata{
				App: &MetadataApp{
					URL: "https://example.com",
				},
				Pack: &MetadataPack{
					Name:        "Example",
					Description: "The most basic, yet awesome, example",
					Version:     "v0.0.1",
				},
			},
			expectedOutput: map[string]interface{}{
				"nomad_pack": map[string]interface{}{
					"app": map[string]interface{}{
						"url": "https://example.com",
					},
					"pack": map[string]interface{}{
						"name":        "Example",
						"description": "The most basic, yet awesome, example",
						"version":     "v0.0.1",
					},
				},
			},
			name: "all metadata values populated",
		},
		{
			inputMetadata: &Metadata{
				App: &MetadataApp{
					URL: "https://example.com",
				},
				Pack: &MetadataPack{
					Name:    "Example",
					URL:     "https://example.com",
					Version: "v0.0.1",
				},
			},
			expectedOutput: map[string]interface{}{
				"nomad_pack": map[string]interface{}{
					"app": map[string]interface{}{
						"url": "https://example.com",
					},
					"pack": map[string]interface{}{
						"name":        "Example",
						"description": "",
						"version":     "v0.0.1",
					},
				},
			},
			name: "some metadata values populated",
		},
		{
			inputMetadata: &Metadata{
				App: &MetadataApp{
					URL:    "https://example.com",
					Author: "The Nomad Team",
				},
				Pack: &MetadataPack{
					URL: "https://example.com",
				},
			},
			expectedOutput: map[string]interface{}{
				"nomad_pack": map[string]interface{}{
					"app": map[string]interface{}{
						"url": "https://example.com",
					},
					"pack": map[string]interface{}{"name": "", "description": "", "version": ""},
				},
			},
			// TODO test added to cover graceful failure while we're in the process of
			// retiring "Author" and "URL" metadata fields. Can be removed in the future.
			name: "author and url fields ignored gracefully",
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
