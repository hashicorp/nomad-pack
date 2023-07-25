// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package variable

import (
	"github.com/hashicorp/hcl/v2"
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
