job [[ coalesce .simple_raw_exec.job_name .nomad_pack.pack.name ]] {
  [[- if empty .simple_raw_exec.region | not ]]
  region = [[quote .simple_raw_exec.region ]]
  [[- end ]]
  [[- if empty .simple_raw_exec.namespace | not ]]
  namespace = [[quote .simple_raw_exec.namespace ]]
  [[- end ]]
  datacenters = [[ .simple_raw_exec.datacenters | toJson ]]
  type = "service"

  group "app" {
    count = [[ .simple_raw_exec.count ]]

    task "server" {
      driver = "raw_exec"

      config {
        command = "/bin/bash"
        args = ["-c",[[ quote .simple_raw_exec.command ]]]
      }
      [[- if (not (empty .simple_raw_exec.env) ) ]]
      [[- print "\n\n      env {\n" -]]
        [[- range $k, $v := .simple_raw_exec.env -]]
        [[- printf "        %s = %q\n" $k $v -]]
        [[- end -]]
      [[- print "      }" -]][[- end -]][[- print "" ]]
    }
  }
}
