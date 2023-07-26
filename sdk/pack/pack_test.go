// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package pack

import (
	"testing"

	"github.com/shoenig/test/must"
)

func TestPack_Name(t *testing.T) {
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
		must.Eq(t, tc.expectedOutput, tc.inputPack.Name(), must.Sprint(tc.name))
	}
}

func TestPack_RootVariableFiles(t *testing.T) {
	testCases := []struct {
		inputPack      *Pack
		expectedOutput map[PackID]*File
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
			expectedOutput: map[PackID]*File{
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
			expectedOutput: map[PackID]*File{
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
		must.Eq(t, tc.expectedOutput, tc.inputPack.RootVariableFiles(), must.Sprint(tc.name))
	}
}
