# Writing Your Own Packs and Pack Repositories

This guide will walk you through the steps involved in writing your own packs and pack registries.

For information on basic use, see the repository [README](../README.md), and for more detailed use, see the [detailed usage documentation](./detailed-usage.md).

## Step One: Create a Custom Registry

First, make a pack registry. This will be a repository that provides the structure, templates, and metadata that define your custom packs.

<!-- TODO: MAKE THE EXAMPLE REPO -->

Cloning [hashicorp/example-pack-repository](https://github.com/hashicorp/example-pack-repository) can give you a head start.

Each registry should have a README.md file that describes thes packs in it, and top-level directories for each pack. Conventionally, the directory name matches the pack name.

The first level of a pack registry looks like the following:

```
.
└── README.md
└── <PACK-NAME-A>
    └── ...pack contents...
└── <PACK-NAME-B>
    └── ...pack contents...
└── ...packs...
```

## Step Two: Add a new Pack

To add a new pack, create a new directory at the top level of the repository.

The directory should have the following contents:

- A `README.md` file containing a human-readable description of the pack, often including any dependency information.
- A `metadata.hcl` file containing information about the pack.
- A `variables.hcl` file that defines the variables in a pack.
- An optional, but _highly encouraged_ `CHANGELOG.md` file that lists changes for each version of the pack.
- An optional `outputs.tpl` file that defines an output to be printed when a pack is deployed.
- A `templates` subdirectory containing the HCL templates used to render the jobspec.

#### metadata.hcl

The `metadata.hcl` file contains important key value information regarding the pack. It contains the following blocks and their associated fields:

- "pack {name}" - The name of the pack.
- "pack {description}" - A small overview of the application that is deployed by the pack.
- "app {url}" - The HTTP(S) url to the homepage of the application to provide a quick reference to the documentation and help pages.
- "pack {type}" - The type of resource that is built by the pack. This currently has only one valid option of "job".
- "app {author}" - An identifier to the author and maintainer of the pack.
- "pack {depedancy: {name, source}}" - The dependencies that the pack has on other packs. Multiple dependencies can be supplied.

An example `metadata.hcl` file:

```
app {
  url = "https://github.com/jrasell//hello-world-app"
  author = "James Rasell"
}

pack {
  name = "hello-world"
  type = "job"
  description = "This pack contains a single job that renders hello world, or a different greeting, to the screen."
}
```

#### variables.hcl

The `variables.hcl` file defines the variables required to fully render and ddeploy all the templates found within the "templates" directory.

These varibles are defined using [HCL](https://github.com/hashicorp/hcl).

An example `variables.hcl` file:

```
variable "datacenters" {
  description = "A list of datacenters in the region which are eligible for task placement."
  type        = list(string)
  default     = ["dc1"]
}

variable "region" {
  description = "The region where the job should be placed."
  type        = string
  default     = "global"
}

variable "app_count" {
  description = "The number of apps to be deployed"
  type        = number
  default     = 3
}

variable "resources" {
  description = "The resource to assign to the application."
  type = object({
    cpu    = number
    memory = number
  })
  default = {
    cpu    = 500,
    memory = 256
  }
}
```

#### outputs.tpl

The `outputs.tpl` file defines an output to be printed when a pack is deployed.

This is an optional file and can also be overridden from the CLI to provide a custom output file.

<!-- TODO: ADD INFORMATION ABOUT HOW THIS ACTUALLY WORKS AND HOW IT GETS NOMAD DATA -->

<!-- TODO: ADD AN EXAMPLE -->

#### README and CHANGELOG

No specific format is required for the `README.md` or `CHANGELOG.md` files.

## Step Three: Write the Templates

Each file at the top level of the `templates` directory that uses the extension ".nomad.tpl" defines a resource (such as a job) that will be applied to Noamd. Thesee files can use any UTF8 encoded prefix as the name.

Helper templates, which can be included within larger templates, have names prefixed with an underscore “\_” and use a ".tpl" extension.

When deploying, Nomad Pack will render each resource template using the variables provided and apply it to Nomad.

#### Template Basics

Templates are written using [Go Template Syntax](https://learn.hashicorp.com/tutorials/nomad/go-template-syntax). This enables templates to have complex logic where necessary.

An example template using variables values from above:

```
job "hello_world" {
  region      = "{{ .hello_world.region }}"
  datacenters = [{{ range $idx, $dc := .hello_world.datacenters }}{{if $idx}},{{end}}{{ $dc | quote }}{{ end }}]
  type = "service"

  group "app" {
    count = {{ .hello_world.count }}

    network {
      port "http" {
        static = 80
      }
    }

    {{/* this is a go template comment */}}

    task "server" {
      driver = "docker"
      config {
        image = "mikenomitch/hello-world"
        network_mode = "host"
        ports = ["http"]
      }

      resources {
        cpu    = {{ .hello_world.resources.cpu }}
        memory = {{ .hello_world.resources.memory }}
      }
    }
  }
}
```

This is a relatively simple template that mostly sets variables.

The `datacenters` value shows slightly more complex Go Template, which allows for [control structures](https://learn.hashicorp.com/tutorials/nomad/go-template-syntax#control-structure-list) like `range` and [pipelines](https://learn.hashicorp.com/tutorials/nomad/go-template-syntax#pipelines).

#### Helper templates

For complex packs, authors may want to reuse template snippets across multiple resources.

For instance, if we had two jobs defined in a pack, and we knew both would re-use the `region` and `datacenters` logic shown above, we could use a helper template to consolidate logic.

Helper template names are prepended with an underscore "\_" and end in ".tpl". So we could define a helper called "\_region_and_dc.tpl":

```
region      = "{{ .hello_world.region }}"
datacenters = [{{ range $idx, $dc := .hello_world.datacenters }}{{if $idx}},{{end}}{{ $dc | quote }}{{ end }}]
```

Then in the parent templates, "job_a.nomad.tpl" and "job_b.nomad.tpl", we would render to the helper template using its name:

```
job "job_a" {

  {{template "region_and_dc"}}
  type = "service"

  ...etc...
}
```

#### Pack Dependencies

Packs can depend on content from other packs.

First, packs must define their dependencies in `metadata.hcl`. A pack stanza with a dependency would look like the the following:

```
app {
  url = "https://github.com/bnorlaug/service"
  author = "Borman Norlaug"
}

pack {
  name = "simple_service"
  type = "job"
  description = "This pack contains a simple service job, and depends on another pack."
}

dependency "demo_dep" {
  name   = “demo_dep”
  source = “git://source.git/packs/demo_dep”
}
```

This would allow templates of "simple_service" to use "demo_dep"'s helper templates in the following way:

```
{{ template "demo_dep.data" . }}
```

## Step Four: Publish and Find your Custom Repository

To use your new pack, you will likely want to publish it to the internet. Push the git repository to a URL accessible by your command line tool. Common version control systems, such as GitHub, GitLab, and Bitbucket work well.

Alternatively, if you copy your pack registry to your local `.nomad/packs` directory. You can deploy it locally without publishing it.

If you wish to share your packs, please consider adding them to the Community Pack Registries in this repo's [README](../README.md).

## Step Five: Deploy your Custom Pack!

Add your custom repository using the `nomad-pack registry add` command.

```
nomad-pack registry add custom-packs git@github.com/<YOUR_ORG>/<YOUR_REPO>
```

Deploy your custom packs.

```
nomad-pack run custom-packs:hello_world --var app_count=1
```

Congrats! You can now write custom packs for Nomad!
