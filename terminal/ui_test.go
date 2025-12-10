// Copyright IBM Corp. 2021, 2025
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
