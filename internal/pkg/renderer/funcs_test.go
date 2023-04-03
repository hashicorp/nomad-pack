// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package renderer

import (
	"bytes"
	"testing"
	"text/template"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func Test_toStringList(t *testing.T) {
	testCases := []struct {
		input          []interface{}
		expectedOutput string
	}{
		{
			input:          []interface{}{"dc1", "dc2", "dc3", "dc4"},
			expectedOutput: `["dc1", "dc2", "dc3", "dc4"]`,
		},
		{
			input:          []interface{}{"dc1"},
			expectedOutput: `["dc1"]`,
		},
		{
			input:          []interface{}{},
			expectedOutput: `[]`,
		},
	}

	for _, tc := range testCases {
		actualOutput, _ := toStringList(tc.input)
		assert.Equal(t, tc.expectedOutput, actualOutput)
	}
}

const (
	// Baseline spew output
	outB = "(renderer.Foo) {\n unexportedField: (renderer.Bar) {\n  data: (*uint)(100)\n },\n ExportedField: (map[interface {}]interface {}) (len=1) {\n  (string) (len=3) \"one\": (bool) true\n }\n}\n"
	// Indent test output with indent set to ∫
	outI = "(renderer.Foo) {\n∫unexportedField: (renderer.Bar) {\n∫∫data: (*uint)(100)\n∫},\n∫ExportedField: (map[interface {}]interface {}) (len=1) {\n∫∫(string) (len=3) \"one\": (bool) true\n∫}\n}\n"
	// MaxDepth output for MaxDepth = 1
	outM = "(renderer.Foo) {\n unexportedField: (renderer.Bar) {\n  <max depth reached>\n },\n ExportedField: (map[interface {}]interface {}) (len=1) {\n  <max depth reached>\n }\n}\n"
)

func TestSpewHelpersInTemplate(t *testing.T) {
	testCases := []struct {
		desc      string
		input     string
		expect    interface{}
		expectErr bool
	}{
		{
			desc:   "baseline",
			expect: outB,
			input:  "[[ $A := customSpew | withDisablePointerAddresses ]][[$A.Sdump .]]",
		},
		{
			desc:   "indent",
			expect: outI,
			input:  `[[ $A := customSpew | withDisablePointerAddresses | withIndent "∫"]][[$A.Sdump .]]`,
		},
		{
			desc:   "maxdepth-int",
			expect: outM,
			input:  "[[ $A := customSpew | withDisablePointerAddresses | withMaxDepth 1 ]][[$A.Sdump .]]",
		},
		{
			desc:      "maxdepth-string",
			expect:    `expected integer; found "1"`,
			expectErr: true,
			input:     `[[ $A := customSpew | withDisablePointerAddresses | withMaxDepth "1" ]][[$A.Sdump .]]`,
		},
		{
			desc:      "maxdepth-bad",
			expect:    `wrong type for value; expected int; got string`,
			expectErr: true,
			input:     `[[$I := "1"]][[ $A := customSpew | withDisablePointerAddresses | withMaxDepth $I ]][[$A.Sdump .]]`,
		},
	}
	type Bar struct {
		data *uint
	}

	type Foo struct {
		unexportedField Bar
		ExportedField   map[interface{}]interface{}
	}
	var a uint = 100
	bar := Bar{&a}
	s1 := Foo{bar, map[interface{}]interface{}{"one": true}}

	for _, tC := range testCases {
		t.Run(tC.desc, func(t *testing.T) {
			var b bytes.Buffer
			tpl := template.Must(template.New("test").Funcs(funcMap(nil)).Delims("[[", "]]").Parse(tC.input))
			err := tpl.Execute(&b, s1)
			if tC.expectErr {
				require.Error(t, err)
				require.Contains(t, err.Error(), tC.expect)
				return
			}
			require.NoError(t, err)
			require.Equal(t, tC.expect, b.String())
		})
	}
}
