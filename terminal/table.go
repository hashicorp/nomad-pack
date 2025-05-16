// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package terminal

import (
	"io"

	"github.com/olekukonko/tablewriter"
	"github.com/olekukonko/tablewriter/renderer"
	"github.com/olekukonko/tablewriter/tw"
)

// Passed to UI.Table to provide a nicely formatted table.
type Table struct {
	Headers []string
	Rows    [][]string
}

// Table creates a new Table structure that can be used with UI.Table.
func NewTable(headers ...string) *Table {
	return &Table{
		Headers: headers,
	}
}

func TableWithSettings(writer io.Writer, headers []string) *tablewriter.Table {
	table := tablewriter.NewTable(writer,
		tablewriter.WithRenderer(renderer.NewBlueprint(tw.Rendition{
			Borders: tw.BorderNone,
		})),
	)
	table.Configure(func(config *tablewriter.Config) {
		config.Row.Formatting.AutoWrap = tw.WrapNone
	})
	table.Header(headers)
	return table
}
