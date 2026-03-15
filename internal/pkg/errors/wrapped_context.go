// Copyright IBM Corp. 2023, 2026
// SPDX-License-Identifier: MPL-2.0

package errors

import (
	"fmt"

	"github.com/hashicorp/hcl/v2"
)

// WrappedUIContext encapsulates an error, subject, and context that can be
// used to provide detail error outputs to the console. It is suggested that
// any function returning an error to the CLI use this instead of a standard
// error.
type WrappedUIContext struct {

	// Err is the full error message to store.
	Err error

	// Subject is a short, high-level summary of the error. It should avoid
	// including complex formatting to include file names for example. These
	// items should be added to the Context instead.
	Subject string

	// Context contains all the context required to fully understand the error
	// and helps troubleshooting.
	Context *UIErrorContext
}

// Error is used to satisfy to builtin.Error interface. This allows us to use
// WrappedUIContext as an error if needed, although we should prefer to return
// the strong type.
func (w *WrappedUIContext) Error() string {
	return fmt.Sprintf("%s: %v: \n%s", w.Subject, w.Err, w.Context.String())
}

// formatHCLRange formats an hcl.Range using the standard HCL v2 format.
// This matches the format used by the HCL library itself and Nomad CLI.
// Format for same line: filename:line,column-column
// Format for different lines: filename:line,column-line,column
// If line/column information is not available, it falls back to just the filename.
func formatHCLRange(r *hcl.Range) string {
	if r == nil {
		return ""
	}

	// Check if we have meaningful line/column information
	// (all zeros means the position wasn't set)
	hasStartPos := r.Start.Line > 0 || r.Start.Column > 0
	hasEndPos := r.End.Line > 0 || r.End.Column > 0

	if !hasStartPos && !hasEndPos {
		// Fall back to filename only if no position info
		return r.Filename
	}

	// Use the standard HCL v2 format: filename:line,column-line,column
	// This matches what hcl.Range.String() returns and what Nomad uses
	if r.Start.Line == r.End.Line {
		// Same line: filename:line,column-column
		return fmt.Sprintf("%s:%d,%d-%d", r.Filename, r.Start.Line, r.Start.Column, r.End.Column)
	} else {
		// Different lines: filename:line,column-line,column
		return fmt.Sprintf("%s:%d,%d-%d,%d", r.Filename, r.Start.Line, r.Start.Column, r.End.Line, r.End.Column)
	}
}

// HCLDiagsToWrappedUIContext converts HCL specific hcl.Diagnostics into an
// array of WrappedUIContext.
func HCLDiagsToWrappedUIContext(diags hcl.Diagnostics) []*WrappedUIContext {
	wrapped := make([]*WrappedUIContext, len(diags))
	for i, diag := range diags {
		wrapped[i] = &WrappedUIContext{
			Err:     newError(diag.Detail),
			Subject: diag.Summary,
			Context: NewUIErrorContext(),
		}

		// Choose the best available range for reporting.
		// Prefer Subject (the location where the error occurred),
		// but fall back to Context if Subject is not available.
		var rangeToUse *hcl.Range
		if diag.Subject != nil {
			rangeToUse = diag.Subject
		} else if diag.Context != nil {
			rangeToUse = diag.Context
		}

		if rangeToUse != nil {
			wrapped[i].Context.Add(UIContextPrefixHCLRange, formatHCLRange(rangeToUse))
		}
	}
	return wrapped
}
