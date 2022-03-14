## Nomad Pack Registry - {{.RegistryName}}
This repository is meant to be used as a reference when writing custom pack
registries for Nomad Pack.

To get started writing your own pack, make a directory with your pack name.
See the documentation on Writing Packs and Registries for more information.
{{- if .AddExamplePack }}{{println}}
Use the `hello_world` pack as an example for file structure and contents.{{end}}
