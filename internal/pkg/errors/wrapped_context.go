// Copyright IBM Corp. 2021, 2025
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
		if diag.Subject != nil {
			wrapped[i].Context.Add(UIContextPrefixHCLRange, diag.Subject.String())
		}
	}
	return wrapped
}
