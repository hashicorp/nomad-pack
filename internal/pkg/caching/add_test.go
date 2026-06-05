// Copyright IBM Corp. 2023, 2026
// SPDX-License-Identifier: MPL-2.0

package caching

import (
	"testing"

	"github.com/shoenig/test/must"

	"github.com/hashicorp/nomad/ci"
)

func TestBuildGoGetterGitURL(t *testing.T) {
	ci.Parallel(t)

	testCases := []struct {
		name     string
		opts     *AddOpts
		expected string
	}{
		{
			name: "latest registry does not force depth",
			opts: &AddOpts{
				Source: "ssh://gitea@gitea.internal/nomad_pack_templates.git",
				Ref:    "latest",
			},
			expected: "ssh://gitea@gitea.internal/nomad_pack_templates.git",
		},
		{
			name: "empty ref treated as latest",
			opts: &AddOpts{
				Source: "https://github.com/hashicorp/nomad-pack-community-registry",
				Ref:    "",
			},
			expected: "https://github.com/hashicorp/nomad-pack-community-registry",
		},
		{
			name: "specific ref appends query",
			opts: &AddOpts{
				Source: "https://github.com/hashicorp/nomad-pack-community-registry",
				Ref:    "v0.1.0",
			},
			expected: "https://github.com/hashicorp/nomad-pack-community-registry?ref=v0.1.0",
		},
		{
			name: "target pack appends packs path",
			opts: &AddOpts{
				Source:   "ssh://gitea@gitea.internal/nomad_pack_templates.git",
				PackName: "simple",
				Ref:      "latest",
			},
			expected: "ssh://gitea@gitea.internal/nomad_pack_templates.git//packs/simple",
		},
		{
			name: "target pack with non latest ref appends ref query",
			opts: &AddOpts{
				Source:   "ssh://gitea@gitea.internal/nomad_pack_templates",
				PackName: "simple",
				Ref:      "main",
			},
			expected: "ssh://gitea@gitea.internal/nomad_pack_templates.git//packs/simple?ref=main",
		},
		{
			name: "ref with slash gets URL encoded",
			opts: &AddOpts{
				Source: "https://github.com/hashicorp/nomad-pack-community-registry",
				Ref:    "feature/add-templates",
			},
			expected: "https://github.com/hashicorp/nomad-pack-community-registry?ref=feature%2Fadd-templates",
		},
		{
			name: "ref with multiple slashes gets URL encoded",
			opts: &AddOpts{
				Source: "https://github.com/hashicorp/nomad-pack-community-registry",
				Ref:    "compliance/update-headers/batch-1",
			},
			expected: "https://github.com/hashicorp/nomad-pack-community-registry?ref=compliance%2Fupdate-headers%2Fbatch-1",
		},
		{
			name: "ref with slash and pack name",
			opts: &AddOpts{
				Source:   "ssh://gitea@gitea.internal/nomad_pack_templates",
				PackName: "simple",
				Ref:      "feature/new-pack",
			},
			expected: "ssh://gitea@gitea.internal/nomad_pack_templates.git//packs/simple?ref=feature%2Fnew-pack",
		},
		{
			name: "SHA ref not encoded",
			opts: &AddOpts{
				Source: "https://github.com/hashicorp/nomad-pack-community-registry",
				Ref:    "5d96571d5600366597a44cf86f1a2f8f7e2959d9",
			},
			expected: "https://github.com/hashicorp/nomad-pack-community-registry?ref=5d96571d5600366597a44cf86f1a2f8f7e2959d9",
		},
		{
			name: "ref with underscore and dash not encoded",
			opts: &AddOpts{
				Source: "https://github.com/hashicorp/nomad-pack-community-registry",
				Ref:    "feature_branch-v2",
			},
			expected: "https://github.com/hashicorp/nomad-pack-community-registry?ref=feature_branch-v2",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ci.Parallel(t)
			actual := buildGoGetterGitURL(tc.opts)
			must.Eq(t, tc.expected, actual)
		})
	}
}
