job [[ template "job_name" . ]] {
  [[ template "region" . ]]
  datacenters = [[ .simple_service.datacenters | toPrettyJson ]]
  type = "service"

  # parse error
  config {}

  group "app" {
    count = [[ .simple_service.count ]]

    network {
      [[- range $port := .simple_service.ports ]]
      port [[ $port.name | quote ]] {
        to = [[ $port.port ]]
      }
      [[- end ]]
    }

    [[- if .simple_service.register_consul_service ]]
    service {
      name = "[[ .simple_service.consul_service_name ]]"
      port = "[[ .simple_service.consul_service_port ]]"
      tags = [[ .simple_service.consul_tags | toPrettyJson ]]

      connect {
        sidecar_service {
          proxy {
            [[- range $upstream := .simple_service.upstreams ]]
            upstreams {
              destination_name = [[ $upstream.name | quote ]]
              local_bind_port  = [[ $upstream.port ]]
            }
            [[- end ]]
          }
        }
      }

      [[- if .simple_service.has_health_check ]]
      check {
        name     = "alive"
        type     = "http"
        path     = [[ .simple_service.health_check.path | quote ]]
        interval = [[ .simple_service.health_check.interval | quote ]]
        timeout  = [[ .simple_service.health_check.timeout | quote ]]
      }
      [[- end ]]
    }
    [[- end ]]

    restart {
      attempts = [[ .simple_service.restart_attempts ]]
      interval = "30m"
      delay = "15s"
      mode = "fail"
    }

    task "server" {
      driver = "docker"

      config {
        image = [[.simple_service.image | quote]]
        ports = ["http"]
      }

      [[- $env_vars_length := len .simple_service.env_vars ]]
      [[- if not (eq $env_vars_length 0) ]]
      env {
        [[- range $var := .simple_service.env_vars ]]
        [[ $var.key ]] = [[ $var.value ]]
        [[- end ]]
      }
      [[- end ]]

      resources {
        cpu    = [[ .simple_service.resources.cpu ]]
        memory = [[ .simple_service.resources.memory ]]
      }
    }
  }
}
