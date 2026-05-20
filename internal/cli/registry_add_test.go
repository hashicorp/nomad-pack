// Copyright IBM Corp. 2023, 2026
// SPDX-License-Identifier: MPL-2.0

package cli

import (
	"testing"

	"github.com/shoenig/test/must"
)

func TestValidateBranchName(t *testing.T) {
	tests := []struct {
		name      string
		branch    string
		wantError bool
	}{
		{
			name:      "valid simple branch",
			branch:    "main",
			wantError: false,
		},
		{
			name:      "valid feature branch",
			branch:    "feature/add-templates",
			wantError: false,
		},
		{
			name:      "valid branch with hyphens",
			branch:    "fix-bug-123",
			wantError: false,
		},
		{
			name:      "empty branch name",
			branch:    "",
			wantError: true,
		},
		{
			name:      "branch name too long",
			branch:    string(make([]byte, 256)),
			wantError: true,
		},
		{
			name:      "branch with double dots",
			branch:    "feature..branch",
			wantError: true,
		},
		{
			name:      "branch starting with slash",
			branch:    "/feature",
			wantError: true,
		},
		{
			name:      "branch ending with slash",
			branch:    "feature/",
			wantError: true,
		},
		{
			name:      "branch with consecutive slashes",
			branch:    "feature//branch",
			wantError: true,
		},
		{
			name:      "branch with null byte",
			branch:    "feature\x00branch",
			wantError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateBranchName(tt.branch)
			if tt.wantError {
				must.Error(t, err)
			} else {
				must.NoError(t, err)
			}
		})
	}
}

func TestLooksLikeSHA(t *testing.T) {
	tests := []struct {
		name   string
		input  string
		expect bool
	}{
		{
			name:   "short SHA",
			input:  "abc123d",
			expect: true,
		},
		{
			name:   "full SHA",
			input:  "abc123def456789012345678901234567890abcd",
			expect: true,
		},
		{
			name:   "branch name",
			input:  "feature/branch",
			expect: false,
		},
		{
			name:   "version tag",
			input:  "v1.0.0",
			expect: false,
		},
		{
			name:   "mixed case",
			input:  "ABC123",
			expect: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := looksLikeSHA(tt.input)
			must.Eq(t, tt.expect, result)
		})
	}
}
