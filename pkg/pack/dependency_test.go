package pack

import (
	"testing"

	"github.com/hashicorp/nomad-pack/internal/pkg/helper/ptr"
	"github.com/stretchr/testify/assert"
)

func TestDependency_Validate(t *testing.T) {
	testCases := []struct {
		inputDependency          *Dependency
		expectedOutputDependency *Dependency
		expectError              bool
		name                     string
	}{
		{
			inputDependency: &Dependency{
				Name:   "example",
				Source: "git://example.com/example",
			},
			expectedOutputDependency: &Dependency{
				Name:    "example",
				Source:  "git://example.com/example",
				Enabled: ptr.Bool(true),
			},
			name: "nil enabled input",
		},
		{
			inputDependency: &Dependency{
				Name:    "example",
				Source:  "git://example.com/example",
				Enabled: ptr.Bool(false),
			},
			expectedOutputDependency: &Dependency{
				Name:    "example",
				Source:  "git://example.com/example",
				Enabled: ptr.Bool(false),
			},
			name: "false enabled input",
		},
		{
			inputDependency: &Dependency{
				Name:    "example",
				Source:  "git://example.com/example",
				Enabled: ptr.Bool(true),
			},
			expectedOutputDependency: &Dependency{
				Name:    "example",
				Source:  "git://example.com/example",
				Enabled: ptr.Bool(true),
			},
			name: "false enabled input",
		},
	}

	for _, tc := range testCases {
		err := tc.inputDependency.validate()
		if tc.expectError {
			assert.NotNil(t, err, tc.name)
		} else {
			assert.Nil(t, err, tc.name)
		}
		assert.Equal(t, tc.expectedOutputDependency, tc.inputDependency, tc.name)
	}
}
