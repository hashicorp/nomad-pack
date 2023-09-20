# variable "simple_raw_exec.command"
#   description: bash command to run
#   type: string
#   default: "echo \"$(date) - Started.\"; while true; do sleep 300; echo -n .; done"
#
# simple_raw_exec.command="echo \"$(date) - Started.\"; while true; do sleep 300; echo -n .; done"


# variable "simple_raw_exec.count"
#   description: The number of app instances to deploy
#   type: number
#   default: 1
#
# simple_raw_exec.count=1


# variable "simple_raw_exec.datacenters"
#   description: A list of datacenters in the region which are eligible for task
#   placement
#   type: list(string)
#   default: ["dc1"]
#
# simple_raw_exec.datacenters=["dc1"]


# variable "simple_raw_exec.env"
#   description: environment variable collection
#   type: map(string)
#   default: {}
#
simple_raw_exec.env = {
  "NOMAD_TOKEN" = "some awesome token"
  "NOMAD_ADDR"  = "http://127.0.0.1:4646"
}


# variable "simple_raw_exec.job_name"
#   description: The name to use as the job name which overrides using the pack name
#   type: string
#   default: ""
#
simple_raw_exec.job_name = "sre"


# variable "simple_raw_exec.namespace"
#   description: namespace to run the job in
#   type: string
#   default: ""
#
# simple_raw_exec.namespace=""


# variable "simple_raw_exec.region"
#   description: The region where jobs will be deployed
#   type: string
#   default: ""
#
# simple_raw_exec.region=""
