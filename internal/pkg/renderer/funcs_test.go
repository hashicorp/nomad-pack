package renderer

import (
	"bytes"
	"testing"
	"text/template"

	"github.com/davecgh/go-spew/spew"
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

func TestSpewHelpers(t *testing.T) {
	noPointerSpew := func() *spew.ConfigState {
		cs := spew.NewDefaultConfig()
		cs.DisablePointerAddresses = true
		cs.SortKeys = true
		return cs
	}

	testCases := []struct {
		desc      string
		input     func(*spew.ConfigState) interface{}
		spew      func() *spew.ConfigState
		expect    interface{}
		expectErr bool
	}{
		{
			desc:   "noop",
			expect: outB,
			spew:   noPointerSpew,
			input: func(cs *spew.ConfigState) interface{} {
				return cs
			},
		},
		{
			desc:   "indent",
			expect: outI,
			spew:   noPointerSpew,
			input: func(cs *spew.ConfigState) interface{} {
				s, err := withIndent("∫", cs)
				if err != nil {
					return err
				}
				return s
			},
		},
		{
			desc:      "maxdepth-bad",
			expect:    "invalid parameter: expected int or int-like string, received string",
			expectErr: true,
			spew:      noPointerSpew,
			input: func(cs *spew.ConfigState) interface{} {
				s, err := withMaxDepth("BAD", cs)
				if err != nil {
					return err
				}
				return s
			},
		},
		{
			desc:   "maxdepth-string",
			expect: outM,
			spew:   noPointerSpew,
			input: func(cs *spew.ConfigState) interface{} {
				s, err := withMaxDepth("1", cs)
				if err != nil {
					return err
				}
				return s
			},
		},
		{
			desc:   "maxdepth-int",
			expect: outM,
			spew:   noPointerSpew,
			input: func(cs *spew.ConfigState) interface{} {
				s, err := withMaxDepth(1, cs)
				if err != nil {
					return err
				}
				return s
			},
		},
		{
			desc:   "good_bool",
			expect: outB,
			spew:   spew.NewDefaultConfig, // using the default which contains pointer addresses.
			input: func(cs *spew.ConfigState) interface{} {
				s, err := withDisablePointerAddresses("true", cs)
				if err != nil {
					return err
				}
				return s
			},
		},
		{
			desc:      "bad_bool",
			expect:    "invalid parameter: expected bool or bool-like string, received string",
			expectErr: true,
			spew:      spew.NewDefaultConfig,
			input: func(cs *spew.ConfigState) interface{} {
				s, err := withDisablePointerAddresses("BAD", cs)
				if err != nil {
					return err
				}
				return s
			},
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
			c := tC.spew()
			o := tC.input(c)
			switch o := o.(type) {
			case *spew.ConfigState:
				c = o
			case error:
				if tC.expectErr {
					require.Error(t, o)
					require.Equal(t, tC.expect, o.Error())
					return
				} else {
					require.FailNow(t, "unexpected error", "error: %v", o)
				}
			default:
				require.FailNow(t, "o not a *spew.ConfigState, got %T", o)
			}

			out := c.Sdump(s1)
			require.Equal(t, tC.expect, out)
		})
	}
}

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
			input:  "[[ $A := customSpew | withDisablePointerAddresses true ]][[$A.Sdump .]]",
		},
		{
			desc:   "indent",
			expect: outI,
			input:  `[[ $A := customSpew | withDisablePointerAddresses true | withIndent "∫"]][[$A.Sdump .]]`,
		},
		{
			desc:   "maxdepth-int",
			expect: outM,
			input:  "[[ $A := customSpew | withDisablePointerAddresses true | withMaxDepth 1 ]][[$A.Sdump .]]",
		},
		{
			desc:   "maxdepth-string",
			expect: outM,
			input:  `[[ $A := customSpew | withDisablePointerAddresses true | withMaxDepth "1" ]][[$A.Sdump .]]`,
		},
		{
			desc:      "maxdepth-bad",
			expect:    "error calling withMaxDepth: invalid parameter: expected int or int-like string, received string",
			expectErr: true,
			input:     `[[ $A := customSpew | withDisablePointerAddresses true | withMaxDepth "bad" ]][[$A.Sdump .]]`,
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
