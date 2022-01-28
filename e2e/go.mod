module github.com/hashicorp/nomad-pack/e2e

go 1.16

replace (
	github.com/hashicorp/nomad-pack => ../
)

require (
	github.com/hashicorp/nomad v1.1.6
	github.com/hashicorp/nomad-pack v0.0.0-00010101000000-000000000000
	github.com/hashicorp/nomad/api v0.0.0-20210816133024-fdb868400424
)
