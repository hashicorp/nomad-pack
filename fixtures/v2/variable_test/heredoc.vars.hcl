# Copyright (c) HashiCorp, Inc.
# SPDX-License-Identifier: MPL-2.0

# This file INTENTIONALLY does not end with a newline after the EOF
# and is used to test a parsing condition that occurs when the file
# ends immediately after the end-of-heredoc marker
# https://github.com/hashicorp/nomad-pack/pull/191

variable_test_pack.input = <<EOF
heredoc
EOF