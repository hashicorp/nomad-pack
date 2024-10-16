// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package pack

import (
	"testing"

	"github.com/hashicorp/nomad/ci"
	"github.com/shoenig/test/must"
)

func TestPack_Name(t *testing.T) {
	ci.Parallel(t)
	testCases := []struct {
		inputPack      *Pack
		expectedOutput string
		name           string
	}{
		{
			inputPack:      &Pack{Metadata: &Metadata{Pack: &MetadataPack{Name: "generic1"}}},
			expectedOutput: "generic1",
			name:           "generic test 1",
		},
		{
			inputPack:      &Pack{Metadata: &Metadata{Pack: &MetadataPack{Name: "generic2"}}},
			expectedOutput: "generic2",
			name:           "generic test 2",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ci.Parallel(t) // Parallel has to be called in the subtest too
			must.Eq(t, tc.expectedOutput, tc.inputPack.Name())
		})
	}
}

func TestPack_RootVariableFiles(t *testing.T) {
	ci.Parallel(t)
	testCases := []struct {
		inputPack      *Pack
		expectedOutput map[ID]*File
		name           string
	}{
		{
			inputPack: &Pack{
				Metadata: &Metadata{
					Pack: &MetadataPack{
						Name: "example",
					},
				},
				RootVariableFile: &File{
					Name:    "variables.hcl",
					Path:    "/opt/packs/example/variables.hcl",
					Content: []byte(`variable "foo" {default = "bar"}`),
				},
			},
			expectedOutput: map[ID]*File{
				"example": {
					Name:    "variables.hcl",
					Path:    "/opt/packs/example/variables.hcl",
					Content: []byte(`variable "foo" {default = "bar"}`),
				},
			},
			name: "zero dependency pack",
		},
		{
			inputPack: &Pack{
				Metadata: &Metadata{
					Pack: &MetadataPack{
						Name: "example",
					},
				},
				RootVariableFile: &File{
					Name:    "variables.hcl",
					Path:    "/opt/packs/example/variables.hcl",
					Content: []byte(`variable "foo" {default = "bar"}`),
				},
				dependencies: []*Pack{
					{
						Metadata: &Metadata{
							Pack: &MetadataPack{
								Name: "dep1",
							},
						},
						RootVariableFile: &File{
							Name:    "variables.hcl",
							Path:    "/opt/packs/dep1/variables.hcl",
							Content: []byte(`variable "hoo" {default = "har"}`),
						},
					},
					{
						Metadata: &Metadata{
							Pack: &MetadataPack{
								Name: "dep2",
							},
						},
						RootVariableFile: &File{
							Name:    "variables.hcl",
							Path:    "/opt/packs/dep2/variables.hcl",
							Content: []byte(`variable "sun" {default = "start"}`),
						},
					},
				},
			},
			expectedOutput: map[ID]*File{
				"example": {
					Name:    "variables.hcl",
					Path:    "/opt/packs/example/variables.hcl",
					Content: []byte(`variable "foo" {default = "bar"}`),
				},
				"example.dep1": {
					Name:    "variables.hcl",
					Path:    "/opt/packs/dep1/variables.hcl",
					Content: []byte(`variable "hoo" {default = "har"}`),
				},
				"example.dep2": {
					Name:    "variables.hcl",
					Path:    "/opt/packs/dep2/variables.hcl",
					Content: []byte(`variable "sun" {default = "start"}`),
				},
			},
			name: "multiple dependencies pack",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ci.Parallel(t) // Parallel has to be called in the subtest too
			must.Eq(t, tc.expectedOutput, tc.inputPack.RootVariableFiles())
		})
	}
}

func TestPack_IsValidName(t *testing.T) {
	testCases := []struct {
		name  string
		input string
		valid bool
	}{
		{name: "empty", input: "", valid: false},
		{name: "slashes", input: "foo/bar", valid: false},
		{name: "dashes", input: "foo-bar", valid: false},
		{name: "dots", input: "foo.bar", valid: false},
		{name: "underscore", input: "foo_bar", valid: true},
		{name: "alphanum", input: "f00bar", valid: true},
		{name: "hiragana", input: "チャーリー", valid: true},
		{name: "emoji", input: "🗯️", valid: false},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			must.Eq(t, IsValidName(tc.input), tc.valid)
		})
	}
}
