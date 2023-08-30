# Copyright (c) HashiCorp, Inc.
# SPDX-License-Identifier: MPL-2.0

variable "job_name" {
  # If "", the pack name will be used
  description = "The name to use as the job name which overrides using the pack name"
  type        = string
  default     = ""
}

variable "region" {
  description = "The region where jobs will be deployed"
  type        = string
  default     = ""
}

variable "datacenters" {
  description = "A list of datacenters in the region which are eligible for task placement"
  type        = list(string)
  default     = ["dc1"]
}

variable "count" {
  description = "The number of app instances to deploy"
  type        = number
  default     = 2
}

variable "message" {
  description = "The message your application will render"
  type        = string
  default     = "Hello World!"
}

variable "register_consul_service" {
  description = "If you want to register a Consul service for the job"
  type        = bool
  default     = true
}

variable "consul_service_name" {
  description = "The Consul service name for the {{.PackName}} application"
  type        = string
  default     = "webapp"
}

variable "consul_service_tags" {
  description = "The Consul service name for the {{.PackName}} application"
  type        = list(string)
  # The default value is shaped to integrate with Fabio or Traefik
  # This routes at the root path "/", to route to this service from
  # another path, change "urlprefix-/" to "urlprefix-/<PATH>" and
  # "traefik.http.routers.http.rule=Path(∫/∫)" to
  # "traefik.http.routers.http.rule=Path(∫/<PATH>∫)"
  default = [
    "urlprefix-/",
    "traefik.enable=true",
    "traefik.http.routers.http.rule=Path(`/`)",
  ]
}
