// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package terminal

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"slices"
	"strings"
	"text/tabwriter"

	"github.com/bgentry/speakeasy"
	"github.com/fatih/color"
	"github.com/mattn/go-isatty"
	"github.com/mitchellh/go-glint"

	"github.com/hashicorp/nomad-pack/internal/pkg/errors"
	"github.com/hashicorp/nomad-pack/internal/pkg/helper"
)

type glintUI struct {
	ctx context.Context
	d   *glint.Document
	row []glint.Component
}

func GlintUI(ctx context.Context) UI {
	result := &glintUI{
		d:   glint.New(),
		row: make([]glint.Component, 0),
		ctx: ctx,
	}

	go result.d.Render(ctx)

	return result
}

func (ui *glintUI) Close() error {
	return ui.d.Close()
}

func (ui *glintUI) Input(input *Input) (string, error) {
	var buf bytes.Buffer

	// Write the prompt, add a space.
	ui.Output(input.Prompt, WithStyle(input.Style), WithWriter(&buf))
	fmt.Fprint(color.Output, strings.TrimRight(buf.String(), "\r\n"))
	fmt.Fprint(color.Output, " ")

	// Ask for input in a go-routine so that we can ignore it.
	errCh := make(chan error, 1)
	lineCh := make(chan string, 1)
	go func() {
		var line string
		var err error
		if input.Secret && isatty.IsTerminal(os.Stdin.Fd()) {
			line, err = speakeasy.Ask("")
		} else {
			r := bufio.NewReader(os.Stdin)
			line, err = r.ReadString('\n')
		}
		if err != nil {
			errCh <- err
			return
		}

		lineCh <- strings.TrimRight(line, "\r\n")
	}()

	select {
	case err := <-errCh:
		return "", err
	case line := <-lineCh:
		return line, nil
	case <-ui.ctx.Done():
		// Print newline so that any further output starts properly
		fmt.Fprintln(color.Output)
		return "", ui.ctx.Err()
	}
}

// Interactive implements UI
func (ui *glintUI) Interactive() bool {
	return isatty.IsTerminal(os.Stdin.Fd())
}

// Output implements UI
func (ui *glintUI) Output(msg string, raw ...any) {
	// Render row and reset
	// This will still respect new lines (i.e. it won't turn several lines of text
	// into one massively long single line of text)
	if len(ui.row) > 0 {
		ui.d.Append(glint.Finalize(
			glint.Layout(ui.row...).Row(),
		))
	}
	ui.row = make([]glint.Component, 0)

	msg, style, _ := Interpret(msg, raw...)

	var cs []glint.StyleOption
	switch style {
	case HeaderStyle:
		cs = append(cs, glint.Bold())
		msg = "\n» " + msg
	case ErrorStyle, ErrorBoldStyle:
		cs = append(cs, glint.Color("lightRed"))
		if style == ErrorBoldStyle {
			cs = append(cs, glint.Bold())
		}

		lines := strings.Split(msg, "\n")
		if len(lines) > 0 {
			ui.d.Append(glint.Finalize(
				glint.Style(
					glint.Text("! "+lines[0]),
					cs...,
				),
			))

			for _, line := range lines[1:] {
				ui.d.Append(glint.Finalize(
					glint.Text("  " + line),
				))
			}
		}

		return

	case WarningStyle, WarningBoldStyle:
		cs = append(cs, glint.Color("lightYellow"))
		if style == WarningBoldStyle {
			cs = append(cs, glint.Bold())
		}

	case SuccessStyle, SuccessBoldStyle:
		cs = append(cs, glint.Color("lightGreen"))
		if style == SuccessBoldStyle {
			cs = append(cs, glint.Bold())
		}

		msg = colorSuccess.Sprint(msg)

	case InfoStyle:
		lines := strings.Split(msg, "\n")
		for i, line := range lines {
			lines[i] = colorInfo.Sprintf("  %s", line)
		}

		msg = strings.Join(lines, "\n")

	case BoldStyle:
		cs = append(cs, glint.Bold())
	case BlueStyle:
		cs = append(cs, glint.Color("blue"))
	case CyanStyle:
		cs = append(cs, glint.Color("cyan"))
	case GreenStyle:
		cs = append(cs, glint.Color("green"))
	case RedStyle:
		cs = append(cs, glint.Color("lightRed"))
	case YellowStyle:
		cs = append(cs, glint.Color("yellow"))
	case LightYellowStyle:
		cs = append(cs, glint.Color("lightYellow"))
	}

	ui.d.Append(glint.Finalize(
		glint.Style(
			glint.Text(msg),
			cs...,
		),
	))
}

// Used to have multiple colors/styles in a single line of output
// Because there's no way to know if the row is complete within this
// method, this relies on Output being called after the final call
// to AppendToRow to force rendering of the row.
func (ui *glintUI) AppendToRow(msg string, raw ...any) {
	msg, style, _ := Interpret(msg, raw...)

	var cs []glint.StyleOption
	switch style {
	case HeaderStyle:
		cs = append(cs, glint.Bold())
		msg = "\n» " + msg
	case ErrorStyle, ErrorBoldStyle:
		cs = append(cs, glint.Color("lightRed"))
		if style == ErrorBoldStyle {
			cs = append(cs, glint.Bold())
		}

		lines := strings.Split(msg, "\n")
		if len(lines) > 0 {
			if len(lines) > 0 {
				for i, line := range lines {
					if i == 0 {
						lines[i] = fmt.Sprintf("! %s", line)
					} else {
						lines[i] = fmt.Sprintf("  %s", line)
					}
				}
				msg = strings.Join(lines, "\n")
			}
		}

	case WarningStyle, WarningBoldStyle:
		cs = append(cs, glint.Color("lightYellow"))
		if style == WarningBoldStyle {
			cs = append(cs, glint.Bold())
		}

	case SuccessStyle, SuccessBoldStyle:
		cs = append(cs, glint.Color("lightGreen"))
		if style == SuccessBoldStyle {
			cs = append(cs, glint.Bold())
		}

		msg = colorSuccess.Sprint(msg)

	case InfoStyle:
		lines := strings.Split(msg, "\n")
		for i, line := range lines {
			lines[i] = colorInfo.Sprintf("  %s", line)
		}

		msg = strings.Join(lines, "\n")

	case BoldStyle:
		cs = append(cs, glint.Bold())
	case BlueStyle:
		cs = append(cs, glint.Color("blue"))
	case CyanStyle:
		cs = append(cs, glint.Color("cyan"))
	case GreenStyle:
		cs = append(cs, glint.Color("green"))
	case RedStyle:
		cs = append(cs, glint.Color("lightRed"))
	case YellowStyle:
		cs = append(cs, glint.Color("yellow"))
	case LightYellowStyle:
		cs = append(cs, glint.Color("lightYellow"))
	}

	ui.row = append(ui.row, glint.Style(
		glint.Text(msg),
		cs...,
	))
}

// NamedValues implements UI
func (ui *glintUI) NamedValues(rows []NamedValue, opts ...Option) {
	cfg := &config{}
	for _, opt := range opts {
		opt(cfg)
	}

	var buf bytes.Buffer
	tr := tabwriter.NewWriter(&buf, 1, 8, 0, ' ', tabwriter.AlignRight)
	for _, row := range rows {
		switch v := row.Value.(type) {
		case int, uint, int8, uint8, int16, uint16, int32, uint32, int64, uint64:
			fmt.Fprintf(tr, "  %s: \t%d\n", row.Name, row.Value)
		case float32, float64:
			fmt.Fprintf(tr, "  %s: \t%f\n", row.Name, row.Value)
		case bool:
			fmt.Fprintf(tr, "  %s: \t%v\n", row.Name, row.Value)
		case string:
			if v == "" {
				continue
			}
			fmt.Fprintf(tr, "  %s: \t%s\n", row.Name, row.Value)
		default:
			fmt.Fprintf(tr, "  %s: \t%s\n", row.Name, row.Value)
		}
	}
	tr.Flush()

	// We want to trim the trailing newline
	text := buf.String()
	if len(text) > 0 && text[len(text)-1] == '\n' {
		text = text[:len(text)-1]
	}

	ui.d.Append(glint.Finalize(glint.Text(text)))
}

// OutputWriters implements UI
func (ui *glintUI) OutputWriters() (io.Writer, io.Writer, error) {
	return os.Stdout, os.Stderr, nil
}

// Status implements UI
func (ui *glintUI) Status() Status {
	st := newGlintStatus()
	ui.d.Append(st)
	return st
}

func (ui *glintUI) StepGroup() StepGroup {
	ctx, cancel := context.WithCancel(context.Background())
	sg := &glintStepGroup{ctx: ctx, cancel: cancel}
	ui.d.Append(sg)
	return sg
}

// Table implements UI
func (ui *glintUI) Table(tbl *Table, opts ...Option) {
	var buf bytes.Buffer
	table := TableWithSettings(&buf, tbl.Headers)
	table.Bulk(tbl.Rows)
	table.Render()
	ui.d.Append(glint.Finalize(glint.Text(buf.String())))
}

// Debug implements UI
func (ui *glintUI) Debug(msg string) {
	ui.Output(msg, WithDebugStyle())
}

// Error implements UI
func (ui *glintUI) Error(msg string) {
	ui.Output(msg, WithErrorStyle())
}

// ErrorWithContext satisfies the ErrorWithContext function on the UI
// interface.
func (ui *glintUI) ErrorWithContext(err error, sub string, ctx ...string) {
	// Grab the glint document.
	d := ui.d

	// The rest of this is copy pasted straight from the ErrorWithContext
	// function in ui.go
	// Title the error output in red with the subject.
	d.Append(glint.Layout(
		glint.Style(
			glint.Text(fmt.Sprintf("! %s\n", helper.Title(sub))),
			glint.Color("red"),
		),
	).Row())

	// Add the error string as well as the error type to the output.
	d.Append(glint.Layout(
		glint.Style(glint.Text("    Error:   "), glint.Bold()),
		glint.Text(err.Error()),
	).Row())

	// Selectively promote Details and Suggestion from the context.
	var extractItem = func(ctx []string, key string) ([]string, string, bool) {
		for i, v := range ctx {
			if strings.HasPrefix(v, key) {
				outStr := v
				outCtx := slices.Delete(ctx, i, i+1)
				return outCtx, outStr, true
			}
		}
		return ctx, "", false
	}
	var promote = func(key string) {
		if oc, item, found := extractItem(ctx, key); found {
			ctx = oc
			splits := strings.Split(item, ": ")

			switch len(splits) {
			case 0:
				// no-op
			case 1:
				// There is something odd going on if we don't get a 2 split
				// if we get 1, print the whole thing out.
				d.Append(glint.Layout(
					glint.Style(glint.Text("    " + splits[0])),
				).Row())
			default:
				d.Append(glint.Layout(
					glint.Style(glint.Text("    "+splits[0]+":   "), glint.Bold()),
					glint.Text(strings.Join(splits[1:], ": "))).Row())
			}
		}
	}

	promote(errors.UIContextErrorDetail)
	promote(errors.UIContextErrorSuggestion)

	// We only want this section once per error output, so we cannot perform
	// this within the ctx loop.
	if len(ctx) > 0 {
		d.Append(glint.Layout(
			glint.Style(glint.Text("    Context: "), glint.Bold()),
		).Row())
	}

	// Iterate the addition context items and append these to the output.
	for _, additionCTX := range ctx {
		d.Append(glint.Layout(
			glint.Style(glint.Text(fmt.Sprintf("        - %s", additionCTX))),
		).Row())
	}
	// Add a new line
	d.Append(glint.Layout(glint.Text("")).Row())
}

// Header implements UI
func (ui *glintUI) Header(msg string) {
	ui.Output(msg, WithHeaderStyle())
}

// Info implements UI
func (ui *glintUI) Info(msg string) {
	ui.Output(msg, WithInfoStyle())
}

// Success implements UI
func (ui *glintUI) Success(msg string) {
	ui.Output(msg, WithSuccessStyle())
}

// Trace implements UI
func (ui *glintUI) Trace(msg string) {
	ui.Output(msg, WithTraceStyle())
}

// Warning implements UI
func (ui *glintUI) Warning(msg string) {
	ui.Output(msg, WithWarningStyle())
}

// WarningBold implements UI
func (ui *glintUI) WarningBold(msg string) {
	ui.Output(msg, WithStyle(WarningBoldStyle))
}
