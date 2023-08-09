job [[ template "job_name" . ]] {
  [[ template "region" . ]]
  datacenters = [[ .my.datacenters  | toStringList ]]
  type = "service"

  group "app" {
    count = [[ .my.count ]]

    network {
      port "http" {
        to = 8000
      }
    }

    [[ if .my.register_consul_service ]]
    service {
      name = "[[ .my.consul_service_name ]]"
      tags = [[ .my.consul_service_tags | toStringList ]]
      port = "http"
      check {
        name     = "alive"
        type     = "http"
        path     = "/"
        interval = "10s"
        timeout  = "2s"
      }
    }
    [[ end ]]

    restart {
      attempts = 2
      interval = "30m"
      delay = "15s"
      mode = "fail"
    }

    task "server" {
      driver = "docker"

      config {
        image = "mnomitch/hello_world_server"
        ports = ["http"]
      }

      env {
        MESSAGE = [[.my.message | quote]]
      }
    }
  }
}
