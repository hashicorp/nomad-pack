job [[ template "job_name" . ]] {
  [[ template "region" . ]]
  datacenters = [[ .{{.PackName}}.datacenters  | toJson ]]
  type = "service"

  group "app" {
    count = [[ .{{.PackName}}.count ]]

    network {
      port "http" {
        to = 8000
      }
    }

    [[ if .{{.PackName}}.register_consul_service ]]
    service {
      name = "[[ .{{.PackName}}.consul_service_name ]]"
      tags = [[ .{{.PackName}}.consul_service_tags | toJson ]]
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
        MESSAGE = [[.{{.PackName}}.message | quote]]
      }
    }
  }
}
