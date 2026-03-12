// Copyright IBM Corp. 2021, 2025
// SPDX-License-Identifier: MPL-2.0

package renderer

import (
	"bytes"
	"strings"
	"testing"
	"text/template"

	"github.com/hashicorp/nomad-pack/internal/pkg/variable/parser"
	"github.com/hashicorp/nomad-pack/sdk/pack"
	"github.com/hashicorp/nomad-pack/sdk/pack/variables"
	nomadapi "github.com/hashicorp/nomad/api"
	"github.com/shoenig/test/must"
	"github.com/zclconf/go-cty/cty"
)

func Test_toStringList(t *testing.T) {
	testCases := []struct {
		input          []any
		expectedOutput string
	}{
		{
			input:          []any{"dc1", "dc2", "dc3", "dc4"},
			expectedOutput: `["dc1", "dc2", "dc3", "dc4"]`,
		},
		{
			input:          []any{"dc1"},
			expectedOutput: `["dc1"]`,
		},
		{
			input:          []any{},
			expectedOutput: `[]`,
		},
	}

	for _, tc := range testCases {
		actualOutput, _ := toStringList(tc.input)
		must.Eq(t, tc.expectedOutput, actualOutput)
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
		expect    string
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
		ExportedField   map[any]any
	}
	var a uint = 100
	bar := Bar{&a}
	s1 := Foo{bar, map[any]any{"one": true}}

	for _, tC := range testCases {
		t.Run(tC.desc, func(t *testing.T) {
			var b bytes.Buffer
			tpl := template.Must(template.New("test").Funcs(funcMap(nil)).Delims("[[", "]]").Parse(tC.input))
			err := tpl.Execute(&b, s1)
			if tC.expectErr {
				must.Error(t, err)
				must.StrContains(t, err.Error(), tC.expect)
				return
			}
			must.NoError(t, err)
			must.Eq(t, tC.expect, b.String())
		})
	}
}

func Test_tplFunc(t *testing.T) {
	testCases := []struct {
		desc      string
		input     string
		data      any
		expect    string
		expectErr bool
		errMsg    string
		strict    bool
	}{
		{
			desc:   "basic string interpolation",
			input:  `[[ tpl "[[ .name ]]" . ]]`,
			data:   map[string]any{"name": "test-job"},
			expect: "test-job",
		},
		{
			desc:   "multiple fields",
			input:  `[[ tpl "[[ .region ]]-[[ .app ]]" . ]]`,
			data:   map[string]any{"region": "us-east-1", "app": "worker"},
			expect: "us-east-1-worker",
		},
		{
			desc:   "using sprig function inside tpl",
			input:  `[[ tpl "[[ upper .name ]]" . ]]`,
			data:   map[string]any{"name": "hello"},
			expect: "HELLO",
		},
		{
			desc:   "nested object access",
			input:  `[[ tpl "[[ .job.name ]]-[[ .job.count ]]" . ]]`,
			data:   map[string]any{"job": map[string]any{"name": "api", "count": 3}},
			expect: "api-3",
		},
		{
			desc:   "passing subset of context",
			input:  `[[ tpl "[[ .name ]]" .job ]]`,
			data:   map[string]any{"job": map[string]any{"name": "worker"}},
			expect: "worker",
		},
		{
			desc:   "empty tpl string",
			input:  `[[ tpl "" . ]]`,
			data:   map[string]any{"name": "test"},
			expect: "",
		},
		{
			desc:   "static string in tpl",
			input:  `[[ tpl "static-value" . ]]`,
			data:   map[string]any{},
			expect: "static-value",
		},
		{
			desc:   "tpl with dict function",
			input:  `[[ tpl "[[ .msg ]]" (dict "msg" "hello-world") ]]`,
			data:   map[string]any{},
			expect: "hello-world",
		},
		{
			desc:   "chained sprig functions in tpl",
			input:  `[[ tpl "[[ .name | upper | replace \"HELLO\" \"HI\" ]]" . ]]`,
			data:   map[string]any{"name": "hello"},
			expect: "HI",
		},
		{
			desc:   "missing key with strict=false returns empty",
			input:  `[[ tpl "value:[[ .missing ]]" . ]]`,
			data:   map[string]any{"name": "test"},
			expect: "value:",
			strict: false,
		},
		{
			desc:      "missing key with strict=true returns error",
			input:     `[[ tpl "value:[[ .missing ]]" . ]]`,
			data:      map[string]any{"name": "test"},
			expectErr: true,
			errMsg:    "missing",
			strict:    true,
		},
		{
			desc:      "invalid template syntax",
			input:     `[[ tpl "[[ .name " . ]]`,
			data:      map[string]any{"name": "test"},
			expectErr: true,
			errMsg:    "cannot parse template",
		},
		{
			desc:   "conditional in tpl",
			input:  `[[ tpl "[[ if .enabled ]]yes[[ else ]]no[[ end ]]" . ]]`,
			data:   map[string]any{"enabled": true},
			expect: "yes",
		},
		{
			desc:   "range in tpl",
			input:  `[[ tpl "[[ range .items ]][[ . ]][[ end ]]" . ]]`,
			data:   map[string]any{"items": []string{"a", "b", "c"}},
			expect: "abc",
		},
		{
			desc:   "toStringList function in tpl",
			input:  `[[ tpl "[[ toStringList .dcs ]]" . ]]`,
			data:   map[string]any{"dcs": []any{"dc1", "dc2"}},
			expect: `["dc1", "dc2"]`,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.desc, func(t *testing.T) {
			// Create a renderer with the strict setting
			r := &Renderer{Strict: tc.strict}

			// Create the template with funcMap and delimiters
			tpl := template.New("test").Funcs(funcMap(r)).Delims("[[", "]]")

			// Parse the test input - this sets up the parent template
			tpl, err := tpl.Parse(tc.input)
			must.NoError(t, err)

			// Set r.tpl so tplFunc can clone it
			r.tpl = tpl

			// Execute the template
			var buf bytes.Buffer
			err = tpl.Execute(&buf, tc.data)

			if tc.expectErr {
				must.Error(t, err)
				must.StrContains(t, err.Error(), tc.errMsg)
				return
			}

			must.NoError(t, err)
			must.Eq(t, tc.expect, buf.String())
		})
	}
}

func Test_tplFunc_NestedCalls(t *testing.T) {
	// Test nested tpl calls simulating: tpl(var("outer")) -> tpl(var("inner")) -> value
	// This mimics a user storing template strings in variables that reference other template variables
	r := &Renderer{Strict: false}

	// Simulate variables containing template strings:
	// outer_tpl contains a template that calls tpl on inner_tpl
	// inner_tpl contains a template that accesses the final value
	data := map[string]any{
		"outer_tpl": `[[ tpl .inner_tpl . ]]`,
		"inner_tpl": `[[ .value ]]`,
		"value":     "final-result",
	}

	// This is equivalent to: tpl(var("outer_tpl")) where outer_tpl calls tpl(var("inner_tpl"))
	input := `[[ tpl .outer_tpl . ]]`

	tpl := template.New("test").Funcs(funcMap(r)).Delims("[[", "]]")
	tpl, err := tpl.Parse(input)
	must.NoError(t, err)

	r.tpl = tpl

	var buf bytes.Buffer
	err = tpl.Execute(&buf, data)
	must.NoError(t, err)
	must.Eq(t, "final-result", buf.String())
}

func Test_tplFunc_WithVarFunction(t *testing.T) {
	// Test that tpl works with V2-style var function
	// This requires proper ParsedVariables and PackTemplateContext setup

	// Create a minimal pack with required metadata fields
	testPack := &pack.Pack{
		Metadata: &pack.Metadata{
			App: &pack.MetadataApp{},
			Pack: &pack.MetadataPack{
				Name: "test-pack",
			},
		},
	}

	// Create ParsedVariables with V2 data using cty.Value
	pv := &parser.ParsedVariables{}
	err := pv.LoadV2Result(map[pack.ID]map[variables.ID]*variables.Variable{
		"test-pack": {
			"job_name":   {Name: "job_name", Value: cty.StringVal("my-service")},
			"region":     {Name: "region", Value: cty.StringVal("us-west-2")},
			"tpl_string": {Name: "tpl_string", Value: cty.StringVal(`[[ var "job_name" . ]]-[[ var "region" . ]]`)},
		},
	})
	must.NoError(t, err)

	// Create PackTemplateContext using the proper method
	ctx, diags := pv.ToPackTemplateContext(testPack)
	must.False(t, diags.HasErrors())

	// Create renderer with pv set so var function is available
	r := &Renderer{
		Strict: false,
		pv:     pv,
	}

	// Template that uses tpl with var inside
	input := `[[ tpl (var "tpl_string" .) . ]]`

	tpl := template.New("test").Funcs(funcMap(r)).Delims("[[", "]]")
	tpl, err = tpl.Parse(input)
	must.NoError(t, err)

	r.tpl = tpl

	var buf bytes.Buffer
	err = tpl.Execute(&buf, ctx)
	must.NoError(t, err)
	must.Eq(t, "my-service-us-west-2", buf.String())
}

func Test_tplFunc_NestedTplWithVar(t *testing.T) {
	// Test nested tpl: tpl(var("outer")) where outer contains tpl(var("inner"))

	testPack := &pack.Pack{
		Metadata: &pack.Metadata{
			App: &pack.MetadataApp{},
			Pack: &pack.MetadataPack{
				Name: "test-pack",
			},
		},
	}

	pv := &parser.ParsedVariables{}
	err := pv.LoadV2Result(map[pack.ID]map[variables.ID]*variables.Variable{
		"test-pack": {
			"outer_tpl": {Name: "outer_tpl", Value: cty.StringVal(`[[ tpl (var "inner_tpl" .) . ]]`)},
			"inner_tpl": {Name: "inner_tpl", Value: cty.StringVal(`[[ var "value" . ]]`)},
			"value":     {Name: "value", Value: cty.StringVal("nested-result")},
		},
	})
	must.NoError(t, err)

	ctx, diags := pv.ToPackTemplateContext(testPack)
	must.False(t, diags.HasErrors())

	r := &Renderer{
		Strict: false,
		pv:     pv,
	}

	// tpl(var("outer_tpl")) -> evaluates to tpl(var("inner_tpl")) -> evaluates to var("value") -> "nested-result"
	input := `[[ tpl (var "outer_tpl" .) . ]]`

	tpl := template.New("test").Funcs(funcMap(r)).Delims("[[", "]]")
	tpl, err = tpl.Parse(input)
	must.NoError(t, err)

	r.tpl = tpl

	var buf bytes.Buffer
	err = tpl.Execute(&buf, ctx)
	must.NoError(t, err)
	must.Eq(t, "nested-result", buf.String())
}

func TestNomadVariables(t *testing.T) {
	client, err := nomadapi.NewClient(nomadapi.DefaultConfig())
	must.NoError(t, err)

	// Create a test variable
	testPath := "test/nomad-pack/test-var"
	testVar := &nomadapi.Variable{
		Namespace: "default",
		Path:      testPath,
		Items:     map[string]string{"test_key": "test_value"},
	}

	_, _, err = client.Variables().Create(testVar, nil)
	if err != nil {
		t.Skipf("Skipping test - Nomad not available: %v", err)
		return
	}
	defer client.Variables().Delete(testPath, nil)

	fn := nomadVariables(client)
	must.NotNil(t, fn)

	result, err := fn("default")
	must.NoError(t, err)
	must.NotNil(t, result)

	if len(result) == 0 {
		t.Fatal("Expected at least one variable")
	}

	// Verify our test variable is in the results
	found := false
	for _, v := range result {
		if v.Path == testPath {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("Expected to find test variable %s in results", testPath)
	}
}

func TestNomadVariablesWithPrefix(t *testing.T) {
	client, err := nomadapi.NewClient(nomadapi.DefaultConfig())
	must.NoError(t, err)

	// Create test variables with different prefixes
	testVars := []*nomadapi.Variable{
		{
			Namespace: "default",
			Path:      "test/nomad-pack/secret/db",
			Items:     map[string]string{"key": "value1"},
		},
		{
			Namespace: "default",
			Path:      "test/nomad-pack/secret/api",
			Items:     map[string]string{"key": "value2"},
		},
		{
			Namespace: "default",
			Path:      "test/nomad-pack/config/app",
			Items:     map[string]string{"key": "value3"},
		},
	}

	// Create all test variables
	for _, v := range testVars {
		_, _, err := client.Variables().Create(v, nil)
		if err != nil {
			t.Skipf("Skipping test - Nomad not available: %v", err)
			return
		}
		defer client.Variables().Delete(v.Path, nil)
	}

	fn := nomadVariables(client)
	must.NotNil(t, fn)

	// Test with "test/nomad-pack/secret/" prefix
	result, err := fn("default", "test/nomad-pack/secret/")
	must.NoError(t, err)
	must.NotNil(t, result)

	if len(result) < 2 {
		t.Fatalf("Expected at least 2 variables with secret/ prefix, got %d", len(result))
	}

	// Verify all returned paths start with the prefix
	for _, v := range result {
		if !strings.HasPrefix(v.Path, "test/nomad-pack/secret/") {
			t.Errorf("Expected path %s to start with 'test/nomad-pack/secret/'", v.Path)
		}
	}

	// Test without prefix (should return more)
	resultAll, err := fn("default")
	must.NoError(t, err)

	if len(resultAll) < len(result) {
		t.Errorf("Expected all results (%d) >= filtered results (%d)", len(resultAll), len(result))
	}
}

func TestNomadVariable(t *testing.T) {
	client, err := nomadapi.NewClient(nomadapi.DefaultConfig())
	must.NoError(t, err)

	// Create a test variable
	testPath := "test/nomad-pack/test-variable"
	testVar := &nomadapi.Variable{
		Namespace: "default",
		Path:      testPath,
		Items: map[string]string{
			"password": "secret123",
			"host":     "localhost",
		},
	}

	_, _, err = client.Variables().Create(testVar, nil)
	if err != nil {
		t.Skipf("Skipping test - Nomad not available: %v", err)
		return
	}
	defer client.Variables().Delete(testPath, nil)

	fn := nomadVariable(client)
	must.NotNil(t, fn)

	// Test reading the variable we just created
	result, err := fn(testPath, "default")
	must.NoError(t, err)
	must.NotNil(t, result)
	must.Eq(t, testPath, result.Path)
	must.Eq(t, "default", result.Namespace)
	must.Eq(t, "secret123", result.Items["password"])
	must.Eq(t, "localhost", result.Items["host"])

	// Test reading non-existent variable
	resultNone, err := fn("nonexistent/path", "default")
	if err == nil {
		t.Error("Expected error for non-existent variable")
	}
	must.Nil(t, resultNone)
}
