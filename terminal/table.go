// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package terminal

import (
	"io"

	"github.com/fatih/color"
	"github.com/olekukonko/tablewriter"
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
		tablewriter.WithBorders(tw.BorderNone),
		tablewriter.WithConfig(
			tablewriter.Config{
				Row: tw.CellConfig{Formatting: tw.CellFormatting{AutoWrap: tw.WrapNone}},
			}))
	table.Header(headers)
	return table
}

// Table implements UI
func (u *basicUI) Table(tbl *Table, opts ...Option) {
	// Build our config and set our options
	cfg := &config{Writer: color.Output}
	for _, opt := range opts {
		opt(cfg)
	}

	table := TableWithSettings(cfg.Writer, tbl.Headers)
	table.Bulk(tbl.Rows)
	table.Render()
}
