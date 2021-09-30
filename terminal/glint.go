package terminal

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"strings"
	"text/tabwriter"

	"github.com/mitchellh/go-glint"
	"github.com/olekukonko/tablewriter"
)

type glintUI struct {
	d   *glint.Document
	row []glint.Component
}

func GlintUI(ctx context.Context) UI {
	result := &glintUI{
		d:   glint.New(),
		row: make([]glint.Component, 0),
	}

	go result.d.Render(ctx)

	return result
}

func (ui *glintUI) Close() error {
	return ui.d.Close()
}

func (ui *glintUI) Input(input *Input) (string, error) {
	return "", ErrNonInteractive
}

// Interactive implements UI
func (ui *glintUI) Interactive() bool {
	// TODO(mitchellh): We can make this interactive later but Glint itself
	// doesn't support input yet. We can pause the document, do some input,
	// then resume potentially.
	return false
}

// Output implements UI
func (ui *glintUI) Output(msg string, raw ...interface{}) {
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
func (ui *glintUI) AppendToRow(msg string, raw ...interface{}) {
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
	table := tablewriter.NewWriter(&buf)
	table.SetHeader(tbl.Headers)
	table.SetBorder(false)
	table.SetAutoWrapText(false)

	for _, row := range tbl.Rows {
		colors := make([]tablewriter.Colors, len(row))
		entries := make([]string, len(row))

		for i, ent := range row {
			entries[i] = ent.Value

			color, ok := colorMapping[ent.Color]
			if ok {
				colors[i] = tablewriter.Colors{color}
			}
		}

		table.Rich(entries, colors)
	}

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
			glint.Text(fmt.Sprintf("! %s\n", strings.Title(sub))),
			glint.Color("red"),
		),
	).Row())

	// Add the error string as well as the error type to the output.
	d.Append(glint.Layout(
		glint.Style(glint.Text("\tError:   "), glint.Bold()),
		glint.Text(err.Error()),
	).Row())

	d.Append(glint.Layout(
		glint.Style(glint.Text("\tType:    "), glint.Bold()),
		glint.Text(fmt.Sprintf("%T", err)),
	).Row())

	// We only want this section once per error output, so we cannot perform
	// this within the ctx loop.
	if len(ctx) > 0 {
		d.Append(glint.Layout(
			glint.Style(glint.Text("\tContext: "), glint.Bold()),
		).Row())
	}

	// Iterate the addition context items and append these to the output.
	for _, additionCTX := range ctx {
		d.Append(glint.Layout(
			glint.Style(glint.Text(fmt.Sprintf("\t         - %s", additionCTX))),
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
