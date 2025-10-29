// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package packdiags

import (
	"errors"
	"testing"

	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/nomad/ci"
	"github.com/shoenig/test/must"
)

var testRange = hcl.Range{
	Filename: "«filename»",
	Start: hcl.Pos{
		Line:   1,
		Column: 1,
		Byte:   0,
	},
	End: hcl.Pos{
		Line:   1,
		Column: 6,
		Byte:   5,
	},
}

func TestPackDiag_DiagFileNotFound(t *testing.T) {
	ci.Parallel(t)
	diag := DiagFileNotFound("test.txt")
	must.Eq(t, diag.Severity, hcl.DiagError)
	must.Eq(t, "Failed to read file", diag.Summary)
	must.Eq(t, `The file "test.txt" could not be read.`, diag.Detail)

	diags := make(hcl.Diagnostics, 0, 1)
	diags = SafeDiagnosticsAppend(diags, diag)
	must.True(t, diags.HasErrors())
}

func TestPackDiag_DiagMissingRootVar(t *testing.T) {
	ci.Parallel(t)
	diag := DiagMissingRootVar("myVar", &testRange)

	must.Eq(t, diag.Severity, hcl.DiagError)
	must.Eq(t, "Missing base variable declaration to override", diag.Summary)
	must.Eq(t, `There is no variable named "myVar". An override file can only override a variable that was already declared in a primary configuration file.`, diag.Detail)
	must.Eq(t, testRange, *diag.Subject)
}

func TestPackDiag_DiagInvalidDefaultValue(t *testing.T) {
	ci.Parallel(t)
	diag := DiagInvalidDefaultValue("test detail", &testRange)

	must.Eq(t, diag.Severity, hcl.DiagError)
	must.Eq(t, "Invalid default value for variable", diag.Summary)
	must.Eq(t, `test detail`, diag.Detail)
	must.Eq(t, testRange, *diag.Subject)
}

func TestPackDiag_DiagFailedToConvertCty(t *testing.T) {
	ci.Parallel(t)
	diag := DiagFailedToConvertCty(errors.New("test error"), &testRange)

	must.Eq(t, diag.Severity, hcl.DiagError)
	must.Eq(t, "Failed to convert Cty to interface", diag.Summary)
	must.Eq(t, `Test Error`, diag.Detail)
	must.Eq(t, testRange, *diag.Subject)
}
func TestPackDiag_DiagInvalidValueForType(t *testing.T) {
	ci.Parallel(t)
	diag := DiagInvalidValueForType(errors.New("test error"), &testRange)

	must.Eq(t, diag.Severity, hcl.DiagError)
	must.Eq(t, "Invalid value for variable", diag.Summary)
	must.StrContains(t, diag.Detail, "This variable value is not compatible with the variable's type constraint")
	must.StrContains(t, diag.Detail, "test error")
	must.Eq(t, testRange, *diag.Subject)
}

func TestPackDiag_DiagInvalidVariableName(t *testing.T) {
	ci.Parallel(t)
	diag := DiagInvalidVariableName(&testRange)

	must.Eq(t, diag.Severity, hcl.DiagError)
	must.Eq(t, "Invalid variable name", diag.Summary)
	must.Eq(t, "Name must start with a letter or underscore and may contain only letters, digits, underscores, and dashes.", diag.Detail)
	must.Eq(t, testRange, *diag.Subject)
}

func TestPackDiag_SafeDiagnosticsAppend(t *testing.T) {
	diags := hcl.Diagnostics{}
	var diag *hcl.Diagnostic

	// HasErrors on hcl.Diagnostics is not nil-safe
	must.Panic(t, func() {
		d1 := diags.Append(diag)
		d1.HasErrors()
	}, must.Sprint("should have panicked"))

	// Using SafeDiagnosticsAppend should prevent the panic
	must.NotPanic(t, func() {
		d2 := SafeDiagnosticsAppend(diags, diag)
		d2.HasErrors()
	}, must.Sprint("should not have panicked"))

	// Verify that the original case still panics
	must.Panic(t, func() {
		d1 := diags.Append(diag)
		d1.HasErrors()
	}, must.Sprint("should have panicked"))
}

func TestPackDiag_SafeDiagnosticsExtend(t *testing.T) {
	var diag *hcl.Diagnostic
	diags := hcl.Diagnostics{}
	diags2 := hcl.Diagnostics{diag}
	must.Panic(t, func() {
		d := diags.Extend(diags2)
		d.HasErrors()
	}, must.Sprint("should have panicked"))

	must.NotPanic(t, func() {
		d := SafeDiagnosticsExtend(diags, diags2)
		d.HasErrors()
	}, must.Sprint("should not have panicked"))

	must.Panic(t, func() {
		d := diags.Extend(diags2)
		d.HasErrors()
	}, must.Sprint("should have still panicked"))
}
