job [[ coalesce ( var "job_name" .) (meta "pack.name" .) | quote ]] {
  [[- if (var "region" .) ]]
  region = [[.region ]]
  [[- end ]]
  [[- if (var "namespace" .) ]]
  namespace = [[ var "namespace" . | quote ]]
  [[- end ]]
  datacenters = [[ var "datacenters" . | toJson ]]
  type = "service"

  group "app" {
    count = [[ var "count" . ]]

    restart {
      attempts = 2
      interval = "30m"
      delay = "15s"
      mode = "fail"
    }

    task "server" {
      driver = "raw_exec"

      config {
        command = "/bin/bash"
        args = ["-c",[[ var "command" . | quote ]]]
      }
      [[- if (var "env" .) ]]
      [[- print "\n\n      env {\n" -]]
        [[- range $k, $v := var "env" . -]]
        [[- printf "        %s = %q\n" $k $v -]]
        [[- end -]]
      [[- print "      }" -]][[- end -]][[- print "" ]]
    }
  }
}
