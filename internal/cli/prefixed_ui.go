// Copyright IBM Corp. 2021, 2025
// SPDX-License-Identifier: MPL-2.0

package cli

import (
	"fmt"
	"io"

	"github.com/hashicorp/nomad-pack/terminal"
)

// prefixedUI wraps a terminal.UI and adds a prefix to all output messages.
// This is used for parallel job monitoring to distinguish output from different jobs.
type prefixedUI struct {
	ui     terminal.UI
	prefix string
}

// newPrefixedUI creates a new prefixed UI wrapper.
func newPrefixedUI(ui terminal.UI, jobID string) *prefixedUI {
	return &prefixedUI{
		ui:     ui,
		prefix: fmt.Sprintf("[%s] ", jobID),
	}
}

func (p *prefixedUI) Input(input *terminal.Input) (string, error) {
	return p.ui.Input(input)
}

func (p *prefixedUI) Interactive() bool {
	return p.ui.Interactive()
}

func (p *prefixedUI) Output(msg string, raw ...any) {
	p.ui.Output(p.prefix+msg, raw...)
}

func (p *prefixedUI) AppendToRow(msg string, raw ...any) {
	p.ui.AppendToRow(p.prefix+msg, raw...)
}

func (p *prefixedUI) NamedValues(vals []terminal.NamedValue, opts ...terminal.Option) {
	p.ui.NamedValues(vals, opts...)
}

func (p *prefixedUI) OutputWriters() (stdout, stderr io.Writer, err error) {
	return p.ui.OutputWriters()
}

func (p *prefixedUI) Status() terminal.Status {
	return p.ui.Status()
}

func (p *prefixedUI) Table(tbl *terminal.Table, opts ...terminal.Option) {
	p.ui.Table(tbl, opts...)
}

func (p *prefixedUI) StepGroup() terminal.StepGroup {
	return p.ui.StepGroup()
}

func (p *prefixedUI) LiveView() terminal.LiveView {
	return p.ui.LiveView()
}

func (p *prefixedUI) Debug(msg string) {
	p.ui.Debug(p.prefix + msg)
}

func (p *prefixedUI) Error(msg string) {
	p.ui.Error(p.prefix + msg)
}

func (p *prefixedUI) ErrorWithContext(err error, sub string, ctx ...string) {
	p.ui.ErrorWithContext(err, p.prefix+sub, ctx...)
}

func (p *prefixedUI) Header(msg string) {
	p.ui.Header(p.prefix + msg)
}

func (p *prefixedUI) Info(msg string) {
	p.ui.Info(p.prefix + msg)
}

func (p *prefixedUI) Success(msg string) {
	p.ui.Success(p.prefix + msg)
}

func (p *prefixedUI) Trace(msg string) {
	p.ui.Trace(p.prefix + msg)
}

func (p *prefixedUI) Warning(msg string) {
	p.ui.Warning(p.prefix + msg)
}

func (p *prefixedUI) WarningBold(msg string) {
	p.ui.WarningBold(p.prefix + msg)
}
