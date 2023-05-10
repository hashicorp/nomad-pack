// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package testui

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"regexp"
	"strings"
	"sync"
	"text/tabwriter"

	"github.com/fatih/color"
	"github.com/hashicorp/nomad-pack/sdk/helper"
	"github.com/hashicorp/nomad-pack/terminal"
	"github.com/olekukonko/tablewriter"
)

type nonInteractiveTestUI struct {
	mu        sync.Mutex
	OutWriter io.Writer
	ErrWriter io.Writer
}

func NonInteractiveTestUI(ctx context.Context, stdout io.Writer, stderr io.Writer) terminal.UI {
	result := &nonInteractiveTestUI{
		OutWriter: stdout,
		ErrWriter: stderr,
	}
	return result
}

func (ui *nonInteractiveTestUI) Input(input *terminal.Input) (string, error) {
	return "", terminal.ErrNonInteractive
}

// Interactive implements UI
func (ui *nonInteractiveTestUI) Interactive() bool {
	return false
}

// Output implements UI
func (ui *nonInteractiveTestUI) Output(msg string, raw ...interface{}) {
	ui.mu.Lock()
	defer ui.mu.Unlock()
	msg, style, _ := terminal.Interpret(msg, raw...)
	w := ui.OutWriter
	switch style {
	case terminal.DebugStyle:
		msg = "debug: " + msg
	case terminal.HeaderStyle:
		msg = "\n» " + msg
	case terminal.ErrorStyle, terminal.ErrorBoldStyle:
		lines := strings.Split(msg, "\n")
		if len(lines) > 0 {
			fmt.Fprintln(w, "! "+lines[0])
			for _, line := range lines[1:] {
				fmt.Fprintln(w, "  "+line)
			}
		}

		return
	case terminal.WarningStyle, terminal.WarningBoldStyle:
		msg = "warning: " + msg
	case terminal.TraceStyle:
		msg = "trace: " + msg
	case terminal.SuccessStyle, terminal.SuccessBoldStyle:

	case terminal.InfoStyle:
		lines := strings.Split(msg, "\n")
		for i, line := range lines {
			lines[i] = colorInfo.Sprintf("  %s", line)
		}

		msg = strings.Join(lines, "\n")
	}

	fmt.Fprintln(w, msg)
}

// TODO: Added purely for compilation purposes. Untested
func (ui *nonInteractiveTestUI) AppendToRow(msg string, raw ...interface{}) {
	ui.mu.Lock()
	defer ui.mu.Unlock()
	msg, style, _ := terminal.Interpret(msg, raw...)
	w := ui.OutWriter
	switch style {
	case terminal.HeaderStyle:
		msg = "\n» " + msg
	case terminal.ErrorStyle, terminal.ErrorBoldStyle:
		lines := strings.Split(msg, "\n")
		if len(lines) > 0 {
			fmt.Fprintln(w, "! "+lines[0])
			for _, line := range lines[1:] {
				fmt.Fprintln(w, "  "+line)
			}
		}

		return

	case terminal.WarningStyle, terminal.WarningBoldStyle:
		msg = "warning: " + msg

	case terminal.SuccessStyle, terminal.SuccessBoldStyle:

	case terminal.InfoStyle:
		lines := strings.Split(msg, "\n")
		for i, line := range lines {
			lines[i] = colorInfo.Sprintf("  %s", line)
		}

		msg = strings.Join(lines, "\n")
	}

	fmt.Fprint(w, msg) // TODO does this work
}

// NamedValues implements UI
func (ui *nonInteractiveTestUI) NamedValues(rows []terminal.NamedValue, opts ...terminal.Option) {
	ui.mu.Lock()
	defer ui.mu.Unlock()

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

	fmt.Fprintln(ui.OutWriter, buf.String())
}

// OutputWriters implements UI
func (ui *nonInteractiveTestUI) OutputWriters() (io.Writer, io.Writer, error) {
	return ui.OutWriter, ui.ErrWriter, nil
}

// Status implements UI
func (ui *nonInteractiveTestUI) Status() terminal.Status {
	return &nonInteractiveStatus{mu: &ui.mu}
}

func (ui *nonInteractiveTestUI) StepGroup() terminal.StepGroup {
	return &nonInteractiveStepGroup{mu: &ui.mu}
}

// Table implements UI
func (ui *nonInteractiveTestUI) Table(tbl *terminal.Table, opts ...terminal.Option) {
	ui.mu.Lock()
	defer ui.mu.Unlock()

	table := tablewriter.NewWriter(ui.OutWriter)
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
}

// Debug implements UI
func (ui *nonInteractiveTestUI) Debug(msg string) {
	ui.Output(msg, terminal.WithDebugStyle())
}

// Error implements UI
func (ui *nonInteractiveTestUI) Error(msg string) {
	ui.Output(msg, terminal.WithErrorStyle())
}

// ErrorWithContext satisfies the ErrorWithContext function on the UI
// interface.
func (ui *nonInteractiveTestUI) ErrorWithContext(err error, sub string, ctx ...string) {
	ui.Error(helper.Title(sub))
	ui.Error("  Error: " + err.Error())
	ui.Error("  Context:")
	max := 0
	for _, entry := range ctx {
		if loc := strings.Index(entry, ":") + 1; loc > max {
			max = loc
		}
	}
	for _, entry := range ctx {
		padding := max - strings.Index(entry, ":") + 1
		ui.Error("  " + strings.Repeat(" ", padding) + entry)
	}
}

// Header implements UI
func (ui *nonInteractiveTestUI) Header(msg string) {
	ui.Output(msg, terminal.WithHeaderStyle())
}

// Info implements UI
func (ui *nonInteractiveTestUI) Info(msg string) {
	ui.Output(msg, terminal.WithInfoStyle())
}

// Success implements UI
func (ui *nonInteractiveTestUI) Success(msg string) {
	ui.Output(msg, terminal.WithSuccessStyle())
}

// Trace implements UI
func (ui *nonInteractiveTestUI) Trace(msg string) {
	ui.Output(msg, terminal.WithTraceStyle())
}

// Warning implements UI
func (ui *nonInteractiveTestUI) Warning(msg string) {
	ui.Output(msg, terminal.WithWarningStyle())
}

// WarningBold implements UI
func (ui *nonInteractiveTestUI) WarningBold(msg string) {
	ui.Output(msg, terminal.WithStyle(terminal.WarningBoldStyle))
}

type nonInteractiveStatus struct {
	mu *sync.Mutex
}

func (s *nonInteractiveStatus) Update(msg string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	fmt.Fprintln(color.Output, msg)
}

func (s *nonInteractiveStatus) Step(status, msg string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	fmt.Fprintf(color.Output, "%s: %s\n", textStatus[status], msg)
}

func (s *nonInteractiveStatus) Close() error {
	return nil
}

type nonInteractiveStepGroup struct {
	mu     *sync.Mutex
	wg     sync.WaitGroup
	closed bool
}

// Start a step in the output
func (f *nonInteractiveStepGroup) Add(str string, args ...interface{}) terminal.Step {
	// Build our step
	step := &nonInteractiveStep{mu: f.mu}

	// Setup initial status
	step.Update(str, args...)

	// Grab the lock now so we can update our fields
	f.mu.Lock()
	defer f.mu.Unlock()

	// If we're closed we don't add this step to our waitgroup or document.
	// We still create a step and return a non-nil step so downstreams don't
	// crash.
	if !f.closed {
		// Add since we have a step
		step.wg = &f.wg
		f.wg.Add(1)
	}

	return step
}

func (f *nonInteractiveStepGroup) Wait() {
	f.mu.Lock()
	f.closed = true
	wg := &f.wg
	f.mu.Unlock()

	wg.Wait()
}

type nonInteractiveStep struct {
	mu   *sync.Mutex
	wg   *sync.WaitGroup
	done bool
}

func (f *nonInteractiveStep) TermOutput() io.Writer {
	return &stripAnsiWriter{Next: color.Output}
}

func (f *nonInteractiveStep) Update(str string, args ...interface{}) {
	f.mu.Lock()
	defer f.mu.Unlock()
	fmt.Fprintln(color.Output, "-> "+fmt.Sprintf(str, args...))
}

func (f *nonInteractiveStep) Status(status string) {}

func (f *nonInteractiveStep) Done() {
	f.mu.Lock()
	defer f.mu.Unlock()

	if f.done {
		return
	}

	// Set done
	f.done = true

	// Unset the waitgroup
	f.wg.Done()
}

func (f *nonInteractiveStep) Abort() {
	f.Done()
}

type stripAnsiWriter struct {
	Next io.Writer
}

func (w *stripAnsiWriter) Write(p []byte) (n int, err error) {
	return w.Next.Write(reAnsi.ReplaceAll(p, []byte{}))
}

var reAnsi = regexp.MustCompile("[\u001B\u009B][[\\]()#;?]*(?:(?:(?:[a-zA-Z\\d]*(?:;[a-zA-Z\\d]*)*)?\u0007)|(?:(?:\\d{1,4}(?:;\\d{0,4})*)?[\\dA-PRZcf-ntqry=><~]))")

var (
	colorInfo = color.New()
)

const (
	Yellow  = terminal.YellowStyle
	Green   = terminal.GreenStyle
	Red     = terminal.RedStyle
	Bold    = terminal.BoldStyle
	Default = terminal.DefaultStyle
)

var colorMapping = map[string]int{
	Green:   tablewriter.FgGreenColor,
	Yellow:  tablewriter.FgYellowColor,
	Red:     tablewriter.FgRedColor,
	Bold:    tablewriter.Bold,
	Default: tablewriter.Normal,
}

var textStatus = map[string]string{
	terminal.StatusOK:      " +",
	terminal.StatusError:   " !",
	terminal.StatusWarn:    " *",
	terminal.StatusTimeout: "<>",
}
