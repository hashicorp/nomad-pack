# Copyright (c) HashiCorp, Inc.
# SPDX-License-Identifier: MPL-2.0

app {
  url = ""
}

pack {
  name        = "child2"
  description = "render-only child dependency"
  url         = "github.com/hashicorp/nomad-pack/fixtures/test_registry/packs/simple-raw-exec"
  version     = "0.0.1"
}
