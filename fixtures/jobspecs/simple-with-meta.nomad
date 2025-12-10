# Copyright IBM Corp. 2021, 2025
# SPDX-License-Identifier: MPL-2.0

job "simple_service" {

  datacenters = ["dc1"]

  meta {
    my-key = "my-value"
  }

  type = "service"

  group "app" {
    count = 1

    network {
      port "http" {
        to = 8000
      }
    }

    restart {
      attempts = 2
      interval = "30m"
      delay    = "15s"
      mode     = "fail"
    }

    task "server" {
      driver = "docker"

      config {
        image = "mnomitch/hello_world_server"
        ports = ["http"]
      }

      resources {
        cpu    = 200
        memory = 256
      }
    }
  }
}
