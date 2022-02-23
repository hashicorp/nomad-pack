module github.com/hashicorp/nomad-pack

go 1.16

replace github.com/hashicorp/hcl/v2 => github.com/hashicorp/hcl/v2 v2.9.2-0.20210407182552-eb14f8319bdc

require (
	github.com/Masterminds/sprig/v3 v3.2.2
	github.com/bgentry/speakeasy v0.1.0
	github.com/briandowns/spinner v1.11.1
	github.com/containerd/console v1.0.3
	github.com/davecgh/go-spew v1.1.1
	github.com/fatih/color v1.13.0
	github.com/hashicorp/go-getter v1.5.11
	github.com/hashicorp/go-hclog v1.1.0
	github.com/hashicorp/go-multierror v1.1.1
	github.com/hashicorp/hcl/v2 v2.10.1
	github.com/hashicorp/nomad v1.2.7-0.20220223202307-7bcf0afd810b
	github.com/hashicorp/nomad-openapi v0.0.0-20220222223240-27714119c4f0
	github.com/hashicorp/nomad/api v0.0.0-20220223202307-7bcf0afd810b // indirect
	github.com/kr/text v0.2.0
	github.com/lab47/vterm v0.0.0-20201001232628-a9dd795f94c2
	github.com/mattn/go-isatty v0.0.14
	github.com/mitchellh/cli v1.1.2
	github.com/mitchellh/go-glint v0.0.0-20210722152315-6515ceb4a127
	github.com/mitchellh/go-wordwrap v1.0.1
	github.com/morikuni/aec v1.0.0
	github.com/olekukonko/tablewriter v0.0.4
	github.com/posener/complete v1.2.3
	github.com/ryanuber/columnize v2.1.2+incompatible
	github.com/spf13/afero v1.6.0
	github.com/spf13/pflag v1.0.5
	github.com/stretchr/testify v1.7.0
	github.com/zclconf/go-cty v1.9.0
	golang.org/x/crypto v0.0.0-20210921155107-089bfa567519
	google.golang.org/grpc v1.44.0
	gopkg.in/yaml.v3 v3.0.0-20200615113413-eeeca48fe776 // indirect
)
