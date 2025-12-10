# Copyright IBM Corp. 2021, 2025
# SPDX-License-Identifier: MPL-2.0

app {
  url    = ""
  author = "Nomad Team" # author field deprecated, left here to make sure we don't panic and fail gracefully
}

pack {
  name        = "simple_raw_exec"
  description = "This is a test fixture pack used because all platforms support raw_exec"
  url         = "github.com/hashicorp/nomad-pack/fixtures/test_registry/packs/simple-raw-exec" # url field deprecated, left here to make sure we don't panic and fail gracefully
  version     = "0.0.1"
}
