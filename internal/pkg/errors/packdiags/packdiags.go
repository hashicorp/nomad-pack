package packdiags

import (
	"fmt"

	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/nomad-pack/internal/pkg/helper"
)

// DiagFileNotFound is returned when pack parsing encounters a required file
// that is missing.
func DiagFileNotFound(f string) *hcl.Diagnostic {
	return &hcl.Diagnostic{
		Severity: hcl.DiagError,
		Summary:  "Failed to read file",
		Detail:   fmt.Sprintf("The file %q could not be read.", f),
	}
}

// DiagMissingRootVar is returned when a pack consumer passes in a variable that
// is not defined for the pack.
func DiagMissingRootVar(name string, sub *hcl.Range) *hcl.Diagnostic {
	return &hcl.Diagnostic{
		Severity: hcl.DiagError,
		Summary:  "Missing base variable declaration to override",
		Detail:   fmt.Sprintf(`There is no variable named %q. An override file can only override a variable that was already declared in a primary configuration file.`, name),
		Subject:  sub,
	}
}

// DiagInvalidDefaultValue is returned when the default for a variable does not
// match the specified variable type.
func DiagInvalidDefaultValue(detail string, sub *hcl.Range) *hcl.Diagnostic {
	return &hcl.Diagnostic{
		Severity: hcl.DiagError,
		Summary:  "Invalid default value for variable",
		Detail:   detail,
		Subject:  sub,
	}
}

// DiagFailedToConvertCty is an error that can happen late in parsing. It should
// not occur, but is here for coverage.
func DiagFailedToConvertCty(err error, sub *hcl.Range) *hcl.Diagnostic {
	return &hcl.Diagnostic{
		Severity: hcl.DiagError,
		Summary:  "Failed to convert Cty to interface",
		Detail:   helper.Title(err.Error()),
		Subject:  sub,
	}
}

// DiagInvalidValueForType is returned when a pack consumer attempts to set a
// variable to an inappopriate value based on the pack's variable specification
func DiagInvalidValueForType(err error, sub *hcl.Range) *hcl.Diagnostic {
	return &hcl.Diagnostic{
		Severity: hcl.DiagError,
		Summary:  "Invalid value for variable",
		Detail:   fmt.Sprintf("This variable value is not compatible with the variable's type constraint: %s.", err),
		Subject:  sub,
	}
}

// DiagInvalidVariableName is returned when a pack author specifies an invalid
// name for a variable in their varfile
func DiagInvalidVariableName(sub *hcl.Range) *hcl.Diagnostic {
	return &hcl.Diagnostic{
		Severity: hcl.DiagError,
		Summary:  "Invalid variable name",
		Detail:   "Name must start with a letter or underscore and may contain only letters, digits, underscores, and dashes.",
		Subject:  sub,
	}
}

// SafeDiagnosticsAppend prevents a nil Diagnostic from appending to the target
// Diagnostics, since HasError is not nil-safe.
func SafeDiagnosticsAppend(base hcl.Diagnostics, in *hcl.Diagnostic) hcl.Diagnostics {
	if in != nil {
		base = base.Append(in)
	}
	return base
}

// SafeDiagnosticsExtend clean where the input Diagnostics of nils as they are
// appended to the base
func SafeDiagnosticsExtend(base, in hcl.Diagnostics) hcl.Diagnostics {
	for _, diag := range in {
		base = SafeDiagnosticsAppend(base, diag)
	}
	return base
}
