# Simple Service

This pack is a used to deploy a Docker image to as a service job to Nomad.

This is ideal for configuring and deploying a simple web application to Nomad.

## Customizing the Docker Image

The docker image deployed can be replaced with a variable. In the example
below, we will deploy and run `httpd:latest`.

```
nomad-pack run simple_service --var image="httpd:latest"
```

## Customizing Ports

The ports that are exposed via Docker can be customized as well.

In this case, we'll write the port values to a file called `./overrides.hcl`:

```
{
  name = "http"
  port = 8000
},
{
  name = "https"
  port = 8001
}
```

Then pass the file into the run command:

```
nomad-pack run simple_service -f ./overrides.hcl`"
```

## Customizing Resources

The application resource limits can be customized:

```
resources = {
  cpu = 500
  memory = 501
}
```

## Customizing Environment Variables

Environment variables can be added:

```
env_vars = [
  {
    key = "foo"
    value = 1
  }
]
```

## Consul Service and Load Balancer Integration

Optionally, this pack can configure a Consul service.

If the `register_consul_service` is unset or set to true, the Consul service will be registered.

Several load balancers in the [The Nomad Pack Community Registry](../README.md) are configured to connect to
this service with ease.

The [NginX](../nginx/README.md) and [HAProxy](../haproxy/README.md) packs can be configured to balance over the
Consul service deployed by this pack. Just ensure that the "consul_service_name" variable provided to those
packs matches this consul_service_name.

The [Fabio](../fabio/README.md) and [Traefik](../traefik/README.md) packs are configured to search for Consul
services with the specific tags.

To tag this Consul service to work with Fabio, add `"urlprefix-<PATH>"`
to the consul_tags. For instance, to route at the root path, you would add `"urlprefix-/"`. To route at the path `"/api/v1"`, you would add '"urlprefix-/api/v1".

To tag this Consul service to work with Traefik, add "traefik.enable=true" to the consul_tags, also add "traefik.http.routers.http.rule=Path(\`<PATH>\`)". To route at the root path, you would add "traefik.http.routers.http.rule=Path(\`/\`)". To route at the path "/api/v1", you would add "traefik.http.routers.http.rule=Path(\`/api/v1\`)".

```
register_consul_service = true

consul_tags = [
  "urlprefix-/",
  "traefik.enable=true",
  "traefik.http.routers.http.rule=Path(`/`)",
]
```

## Customizing Consul and Upstream Services

Consul configuration can be tweaked and (upstream services)[https://www.nomadproject.io/docs/job-specification/upstreams]
can be added as well.

```
register_consul_service = true
consul_service_name = "app-service-name"
has_health_check = true
health_check = {
  path = "/health"
  interval = "20s"
  timeout  = "3s"
}
upstreams = [
  {
    name = "other-service"
    port = 8001
  }
]
```
