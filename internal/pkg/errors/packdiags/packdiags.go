package packdiags

import (
	"fmt"

	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/nomad-pack/internal/pkg/helper"
)

func DiagFileNotFound(f string) *hcl.Diagnostic {
	return &hcl.Diagnostic{
		Severity: hcl.DiagError,
		Summary:  "Failed to read file",
		Detail:   fmt.Sprintf("The file %q could not be read.", f),
	}
}

func DiagsFileNotFound(f string) hcl.Diagnostics {
	return hcl.Diagnostics{DiagFileNotFound(f)}
}

func DiagMissingRootVar(name string, sub *hcl.Range) *hcl.Diagnostic {
	return &hcl.Diagnostic{
		Severity: hcl.DiagError,
		Summary:  "Missing base variable declaration to override",
		Detail:   fmt.Sprintf(`There is no variable named %q. An override file can only override a variable that was already declared in a primary configuration file.`, name),
		Subject:  sub,
	}
}

func DiagInvalidDefaultValue(detail string, sub *hcl.Range) *hcl.Diagnostic {
	return &hcl.Diagnostic{
		Severity: hcl.DiagError,
		Summary:  "Invalid default value for variable",
		Detail:   detail,
		Subject:  sub,
	}
}

func DiagFailedToConvertCty(err error, sub *hcl.Range) *hcl.Diagnostic {
	return &hcl.Diagnostic{
		Severity: hcl.DiagError,
		Summary:  "Failed to convert Cty to interface",
		Detail:   helper.Title(err.Error()),
		Subject:  sub,
	}
}
func DiagInvalidValueForType(err error, sub *hcl.Range) *hcl.Diagnostic {
	return &hcl.Diagnostic{
		Severity: hcl.DiagError,
		Summary:  "Invalid value for variable",
		Detail:   fmt.Sprintf("This variable value is not compatible with the variable's type constraint: %s.", err),
		Subject:  sub,
	}
}

func DiagInvalidVariableName(sub *hcl.Range) *hcl.Diagnostic {
	return &hcl.Diagnostic{
		Severity: hcl.DiagError,
		Summary:  "Invalid variable name",
		Detail:   "Name must start with a letter or underscore and may contain only letters, digits, underscores, and dashes.",
		Subject:  sub,
	}
}
