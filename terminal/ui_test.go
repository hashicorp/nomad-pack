// Copyright IBM Corp. 2023, 2026
// SPDX-License-Identifier: MPL-2.0

package terminal

import (
	"bytes"
	"strings"
	"testing"

	"github.com/shoenig/test/must"
)

func TestNamedValues(t *testing.T) {
	var buf bytes.Buffer
	var ui basicUI
	ui.NamedValues([]NamedValue{
		{"hello", "a"},
		{"this", "is"},
		{"a", "test"},
		{"of", "foo"},
		{"the_key_value", "style"},
	},
		WithWriter(&buf),
	)

	expected := `
          hello: a
           this: is
              a: test
             of: foo
  the_key_value: style

`

	must.Eq(t, strings.TrimLeft(expected, "\n"), buf.String())
}

func TestNamedValues_server(t *testing.T) {
	var buf bytes.Buffer
	var ui basicUI
	ui.Output("Server configuration:", WithHeaderStyle(), WithWriter(&buf))
	ui.NamedValues([]NamedValue{
		{"DB Path", "data.db"},
		{"gRPC Address", "127.0.0.1:1234"},
		{"HTTP Address", "127.0.0.1:1235"},
		{"URL Service", "api.alpha.waypoint.run:443 (account: token)"},
	},
		WithWriter(&buf),
	)

	expected := `
==> Server configuration:
       DB Path: data.db
  gRPC Address: 127.0.0.1:1234
  HTTP Address: 127.0.0.1:1235
   URL Service: api.alpha.waypoint.run:443 (account: token)

`

	must.Eq(t, expected, buf.String())
}

func TestStatusStyle(t *testing.T) {
	var buf bytes.Buffer
	var ui basicUI
	ui.Output(strings.TrimSpace(`
one
two
  three`),
		WithWriter(&buf),
		WithInfoStyle(),
	)

	expected := `    one
    two
      three
`

	must.Eq(t, expected, buf.String())
}

// TestOutput_FormatSpecifiers tests that pre-rendered content containing format
// specifiers like %i, %l is preserved when passed via "%s" wrapping.
// This is a regression test for the issue where rendered templates containing
// format specifiers were incorrectly converted (e.g., %i -> %!i(MISSING)).
func TestOutput_FormatSpecifiers(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "simple format specifier %i",
			input:    "%i",
			expected: "%i\n",
		},
		{
			name:     "format specifier %l",
			input:    "%l",
			expected: "%l\n",
		},
		{
			name:     "format specifier with flags %-4444l",
			input:    "%-4444l",
			expected: "%-4444l\n",
		},
		{
			name:     "multiple format specifiers",
			input:    "%i %l %d %s",
			expected: "%i %l %d %s\n",
		},
		{
			name:     "heredoc with format specifier",
			input:    "data = <<EOT\n%i\nEOT",
			expected: "data = <<EOT\n%i\nEOT\n",
		},
		{
			name:     "double percent signs preserved via %s wrapping",
			input:    "100%% complete, %i done",
			expected: "100%% complete, %i done\n",
		},
		{
			name:     "realistic template content",
			input:    "template {\n  destination = \"config/logback.xml\"\n  data = <<EOT\n%i\nEOT\n}",
			expected: "template {\n  destination = \"config/logback.xml\"\n  data = <<EOT\n%i\nEOT\n}\n",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var buf bytes.Buffer
			var ui basicUI
			// Use "%s" wrapping to pass pre-rendered content safely,
			// just as the render command does.
			ui.Output("%s", tc.input, WithWriter(&buf))
			must.Eq(t, tc.expected, buf.String())
		})
	}
}

// TestOutput_WithArguments tests that format strings still work correctly
// when arguments are provided.
func TestOutput_WithArguments(t *testing.T) {
	tests := []struct {
		name     string
		format   string
		args     []any
		expected string
	}{
		{
			name:     "single %s argument",
			format:   "Hello %s",
			args:     []any{"world"},
			expected: "Hello world\n",
		},
		{
			name:     "multiple arguments",
			format:   "Job '%s' in pack '%s' registered",
			args:     []any{"my-job", "my-pack"},
			expected: "Job 'my-job' in pack 'my-pack' registered\n",
		},
		{
			name:     "integer formatting",
			format:   "Count: %d",
			args:     []any{42},
			expected: "Count: 42\n",
		},
		{
			name:     "mixed arguments",
			format:   "Processing %d of %d files (%s)",
			args:     []any{5, 10, "in progress"},
			expected: "Processing 5 of 10 files (in progress)\n",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var buf bytes.Buffer
			var ui basicUI
			ui.Output(tc.format, append(tc.args, WithWriter(&buf))...)
			must.Eq(t, tc.expected, buf.String())
		})
	}
}
