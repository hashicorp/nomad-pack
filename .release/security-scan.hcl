# Copyright (c) HashiCorp, Inc.
# SPDX-License-Identifier: MPL-2.0

container {
  dependencies    = true
  alpine_security = true
  secrets         = true

  # Triage items that are _safe_ to ignore here. Note that this list should be
  # periodically cleaned up to remove items that are no longer found by the
  # scanner.
  triage {
    suppress {
      vulnerabilities = [
        "GO-2022-0635",   // github.com/aws/aws-sdk-go@v1.55.6 TODO(jrasell): remove when dep updated.
        "CVE-2024-58251", // busybox@1.37.0-r12 TODO(dduzgun-security): remove when dep updated.
        "CVE-2025-46394", // busybox@1.37.0-r12 TODO(dduzgun-security): remove when dep updated.
      ]
    }
  }
}

binary {
  secrets    = true
  go_modules = true
  osv        = true
  oss_index  = false
  nvd        = false

  # Triage items that are _safe_ to ignore here. Note that this list should be
  # periodically cleaned up to remove items that are no longer found by the
  # scanner.
  triage {
    suppress {
      vulnerabilities = [
        "GO-2022-0635",   // github.com/aws/aws-sdk-go@v1.55.6 TODO(jrasell): remove when dep updated.
        "CVE-2024-58251", // busybox@1.37.0-r12 TODO(dduzgun-security): remove when dep updated.
        "CVE-2025-46394", // busybox@1.37.0-r12 TODO(dduzgun-security): remove when dep updated.
      ]
    }
  }
}
