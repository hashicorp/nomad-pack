# Copyright IBM Corp. 2021, 2025
# SPDX-License-Identifier: MPL-2.0

variable "job_name" {
  description = "job name"
  type        = string
  default     = "child2"
}

variable "complex" {
  description = "complex object for rendering"
  default     = {}
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
