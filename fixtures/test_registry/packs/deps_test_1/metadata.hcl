# Copyright (c) HashiCorp, Inc.
# SPDX-License-Identifier: MPL-2.0

app {
  author = "Nomad Team"
  url    = ""
}

pack {
  name        = "deps_test_1"
  description = "This pack tests repeated dependencies"
  version     = "0.0.1"
}

dependency "child" {
  alias = "child1"
  source = "./child"
}

dependency "child" {
  alias = "child2"
  source = "./child"
}
