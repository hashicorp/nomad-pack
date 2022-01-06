You deployed a service to Nomad.

There are [[ .simple_service.count ]] instances of your job now running.

The service is using the image: [[.simple_service.image | quote]]

[[ if .simple_service.register_consul_service ]]
You registered an associated Consul service named [[ .simple_service.consul_service_name ]].

[[ if .simple_service.has_health_check ]]
This service has a health check at the path : [[ .simple_service.health_check.path | quote ]]
[[ end ]]
[[ end ]]

