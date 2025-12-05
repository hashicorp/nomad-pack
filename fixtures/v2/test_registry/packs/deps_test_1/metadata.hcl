# Copyright IBM Corp. 2021, 2025
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
}

dependency "child" {
  alias = "child2"
}
