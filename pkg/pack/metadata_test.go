package pack

import (
	"testing"

	"github.com/stretchr/testify/assert"
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
					URL:    "https://example.com",
					Author: "Timothy J. Berners-Lee",
				},
				Pack: &MetadataPack{
					Name:        "Example",
					Description: "The most basic, yet awesome, example",
					URL:         "https://example.com",
					Version:     "v0.0.1",
				},
			},
			expectedOutput: map[string]interface{}{
				"nomad_pack": map[string]interface{}{
					"app": map[string]interface{}{
						"url":    "https://example.com",
						"author": "Timothy J. Berners-Lee",
					},
					"pack": map[string]interface{}{
						"name":        "Example",
						"description": "The most basic, yet awesome, example",
						"url":         "https://example.com",
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
						"url":    "https://example.com",
						"author": "",
					},
					"pack": map[string]interface{}{
						"name":        "Example",
						"description": "",
						"url":         "https://example.com",
						"version":     "v0.0.1",
					},
				},
			},
			name: "some metadata values populated",
		},
	}

	for _, tc := range testCases {
		actualOutput := tc.inputMetadata.ConvertToMapInterface()
		assert.Equal(t, tc.expectedOutput, actualOutput, tc.name)
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
					URL:    "https://example.com",
					Author: "Timothy J. Berners-Lee",
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
			assert.NotNil(t, err, tc.name)
		} else {
			assert.Nil(t, err, tc.name)
		}
	}
}
