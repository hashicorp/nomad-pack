// allow nomad-pack to set the job name

[[- define "job_name" -]]
[[- if eq .simple_service.job_name "" -]]
[[- .nomad_pack.pack.name | quote -]]
[[- else -]]
[[- .simple_service.job_name | quote -]]
[[- end -]]
[[- end -]]

// only deploys to a region if specified

[[- define "region" -]]
[[- if not (eq .simple_service.region "") -]]
region = [[ .simple_service.region | quote]]
[[- end -]]
[[- end -]]
