// allow nomad-pack to set the job name
[[ define "job_name" ]]
[[- if eq .my.job_name "" -]]
[[- .nomad_pack.pack.name | quote -]]
[[- else -]]
[[- .my.job_name | quote -]]
[[- end ]]
[[- end ]]

// only deploys to a region if specified
[[ define "region" -]]
[[- if not (eq .my.region "") -]]
  region = [[ .my.region | quote]]
[[- end -]]
[[- end -]]

// Generic constraint
[[ define "constraints" -]]
[[ range $idx, $constraint := . ]]
  constraint {
    attribute = [[ $constraint.attribute | quote ]]
    [[ if $constraint.operator -]]
    operator  = [[ $constraint.operator | quote ]]
    [[ end -]]
    value     = [[ $constraint.value | quote ]]
  }
[[ end -]]
[[- end -]]

// Generic "service" block template
[[ define "service" -]]
[[ $service := . ]]
      service {
        name = [[ $service.service_name | quote ]]
        port = [[ $service.service_port_label | quote ]]
        tags = [[ $service.service_tags | toStringList ]]
        provider = [[ $service.service_provider | quote ]]
        [[- if gt (len $service.upstreams) 0 ]]
        connect {
          sidecar_service {
            proxy {
              [[- if gt (len $service.upstreams) 0 ]]
              [[- range $upstream := $service.upstreams ]]
              upstreams {
                destination_name = [[ $upstream.name | quote ]]
                local_bind_port  = [[ $upstream.port ]]
              }
              [[- end ]]
              [[- end ]]
            }
          }
        }
        [[- end ]]
        check {
          type     = [[ $service.check_type | quote ]]
          [[- if $service.check_path]]
          path     = [[ $service.check_path | quote ]]
          [[- end]]
          interval = [[ $service.check_interval | quote ]]
          timeout  = [[ $service.check_timeout | quote ]]
        }
      }
[[- end ]]

// Generic env_vars template
[[ define "env_vars" -]]
        [[- range $idx, $var := . ]]
        [[ $var.key ]] = [[ $var.value | quote ]]
        [[- end ]]
[[- end ]]

// Generic resources template
[[ define "resources" -]]
[[- $resources := . ]]
      resources {
        cpu    = [[ $resources.cpu ]]
        memory = [[ $resources.memory ]]
      }
[[- end ]]
