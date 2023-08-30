# Copyright (c) HashiCorp, Inc.
# SPDX-License-Identifier: MPL-2.0

app {
  url = "https://learn.hashicorp.com/tutorials/nomad/get-started-run?in=nomad/get-started"
}

pack {
  name        = "simple_service"
  description = "This deploys a simple service job to Nomad that runs a docker container."
  version     = "0.0.1"
}