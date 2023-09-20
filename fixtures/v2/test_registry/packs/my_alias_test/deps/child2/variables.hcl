# Copyright (c) HashiCorp, Inc.
# SPDX-License-Identifier: MPL-2.0

variable "job_name" {
  description = "job name"
  type        = string
  default     = "child2"
}

variable "complex" {
  description = "complex object for rendering"
  type = object({
    name    = string
    address = string
    ids     = list(string)
    lookup = map(object({
      a = number
      b = string
    }))
  })
}
