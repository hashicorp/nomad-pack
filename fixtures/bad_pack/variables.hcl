variable "job_name" {
  description = "The name to use as the job name which overrides using the pack name"
  type        = string
  // If "", the pack name will be used
  default = ""
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

variable "image" {
  description = ""
  type        = string
  default     = "mnomitch/hello_world_server"
}

variable "count" {
  description = "The number of app instances to deploy"
  type        = number
  default     = 1
}

variable "restart_attempts" {
  description = "The number of times the task should restart on updates"
  type        = number
  default     = 2
}

variable "has_health_check" {
  description = "If you want to register a health check in consul"
  type        = bool
  default     = false
}

variable "health_check" {
  description = ""
  type = object({
    path = string
    interval = string
    timeout = string
  })

  default = {
    path = "/"
    interval = "10s"
    timeout  = "2s"
  }
}

variable "upstreams" {
description = ""
type = list(object({
  name   = string
  port = string
  }))
}

variable "register_consul_service" {
  description = "If you want to register a consul service for the job"
  type        = bool
  default     = false
}

variable "ports" {
  description = ""
  type = list(object({
    name = string
    port = number
  }))

  default = [{
    name = "http"
    port = 8000
  }]
}

variable "env_vars" {
  description = ""
  type = list(object({
    key   = string
    value = string
  }))
  default = []
}

variable "consul_service_name" {
  description = "The consul service name for the application"
  type        = string
  default     = "service"
}

variable "consul_service_port" {
  description = "The consul service name for the application"
  type        = string
  default     = "http"
}

variable "consul_tags" {
  description = ""
  type = list(string)
  default = []
}

variable "resources" {
  description = "The resource to assign to the Nginx system task that runs on every client"
  type = object({
    cpu    = number
    memory = number
  })
  default = {
    cpu    = 200,
    memory = 256
  }
}

variable "consul_tags" {
  description = "The consul service name for the hello-world application"
  type        = list(string)
  default = []
}
