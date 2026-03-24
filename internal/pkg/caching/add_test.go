// Copyright IBM Corp. 2021, 2025
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
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ci.Parallel(t)
			actual := buildGoGetterGitURL(tc.opts)
			must.Eq(t, tc.expected, actual)
		})
	}
}
