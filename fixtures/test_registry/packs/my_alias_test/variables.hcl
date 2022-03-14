variable "job_name" {
  description = "The name to use as the job name which overrides using the pack name"
  type        = string
  default     = "deps_test"
}

variable "test_name" {
  description = "This variable allows for configurable test output"
  type        = string
  default     = "test"
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
  default     = 1
}

variable "command" {
	type = string
	description = "bash command to run"
	default     = "echo \"$(date) - Started.\"; while true; do sleep 300; echo -n .; done"
}

variable "env" {
  type = map(string)
  description = "environment variable collection"
  default = {}
}

variable "test_name" {
  type = string
  description = "behavior modifying constant"
  default = ""
}
