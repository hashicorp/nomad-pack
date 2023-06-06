// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package variable

import (
	"fmt"

	"github.com/hashicorp/hcl/v2"

	"github.com/hashicorp/nomad-pack/internal/pkg/helper"
)

func safeDiagnosticsAppend(base hcl.Diagnostics, new *hcl.Diagnostic) hcl.Diagnostics {
	if new != nil {
		base = base.Append(new)
	}
	return base
}

func safeDiagnosticsExtend(base, new hcl.Diagnostics) hcl.Diagnostics {
	if new != nil && new.HasErrors() {
		base = base.Extend(new)
	}
	return base
}

func diagnosticMissingRootVar(name string, sub *hcl.Range) *hcl.Diagnostic {
	return &hcl.Diagnostic{
		Severity: hcl.DiagError,
		Summary:  "Missing base variable declaration to override",
		Detail:   fmt.Sprintf(`There is no variable named %q. An override file can only override a variable that was already declared in a primary configuration file.`, name),
		Subject:  sub,
	}
}

func diagnosticInvalidDefaultValue(detail string, sub *hcl.Range) *hcl.Diagnostic {
	return &hcl.Diagnostic{
		Severity: hcl.DiagError,
		Summary:  "Invalid default value for variable",
		Detail:   detail,
		Subject:  sub,
	}
}

func diagnosticFailedToConvertCty(err error, sub *hcl.Range) *hcl.Diagnostic {
	return &hcl.Diagnostic{
		Severity: hcl.DiagError,
		Summary:  "Failed to convert Cty to interface",
		Detail:   helper.Title(err.Error()),
		Subject:  sub,
	}
}
func diagnosticInvalidValueForType(err error, sub *hcl.Range) *hcl.Diagnostic {
	return &hcl.Diagnostic{
		Severity: hcl.DiagError,
		Summary:  "Invalid value for variable",
		Detail:   fmt.Sprintf("This variable value is not compatible with the variable's type constraint: %s.", err),
		Subject:  sub,
	}
}

func diagnosticInvalidVariableName(sub *hcl.Range) *hcl.Diagnostic {
	return &hcl.Diagnostic{
		Severity: hcl.DiagError,
		Summary:  "Invalid variable name",
		Detail:   "Name must start with a letter or underscore and may contain only letters, digits, underscores, and dashes.",
		Subject:  sub,
	}
}
