job simple_raw_exec {
  datacenters = ["dc1"]
  type        = "service"

  meta {
    my-key = "my-value"
  }

  group "app" {
    count = 1

    restart {
      attempts = 2
      interval = "30m"
      delay    = "15s"
      mode     = "fail"
    }

    task "server" {
      driver = "raw_exec"

      config {
        command = "/bin/bash"
        args    = ["-c", "echo \"$(date) - Started.\"; while true; do sleep 300; echo -n .; done"]
      }
    }
  }
}
