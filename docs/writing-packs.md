# Writing Your Own Packs and Pack Repositories

This guide will walk you through the steps involved in writing your own packs and pack registries.

For information on basic use, see the repository [README](../README.md), and for more detailed use, see
the [detailed usage documentation](./detailed-usage.md).

## Step One: Create a Custom Registry

First, make a pack registry. This will be a repository that provides the structure, templates, and metadata that define your custom packs.

<!-- TODO: MAKE THE EXAMPLE REPO -->

Cloning [hashicorp/example-nomad-pack-registry](https://github.com/hashicorp/example-nomad-pack-registry) can give you a head start.

Each registry should have a README.md file that describes the packs in it, and 
top-level directory named `packs` that contains a subdirectory for each individual 
pack. Conventionally, the pack subdirectory name matches the pack name.

The top level of a pack registry looks like the following:

```
.
└── README.md
└── packs
    └── <PACK-NAME-A>
        └── ...pack contents...
    └── <PACK-NAME-B>
        └── ...pack contents...
    └── ...packs...
```

## Step Two: Add a new Pack

To add a new pack, create a new directory in the `packs` directory of the repository.

The directory should have the following contents:

- A `README.md` file containing a human-readable description of the pack, often including any dependency information.
- A `metadata.hcl` file containing information about the pack.
- A `variables.hcl` file that defines the variables in a pack.
- An optional, but _highly encouraged_ `CHANGELOG.md` file that lists changes for each version of the pack.
- An optional `outputs.tpl` file that defines an output to be printed when a pack is deployed.
- A `templates` subdirectory containing the HCL templates used to render the jobspec.

#### metadata.hcl

The `metadata.hcl` file contains important key value information regarding the pack. It contains the following blocks and their associated fields:

- "app {url}" - The HTTP(S) url to the homepage of the application to provide a quick reference to the documentation and help pages.
- "app {author}" - An identifier to the author and maintainer of the pack.
- "pack {name}" - The name of the pack.
- "pack {description}" - A small overview of the application that is deployed by the pack.
- "pack {url}" - The source URL for the pack itself.
- "pack {version}" - The version of the pack.
- "dependency {name}" - The dependencies that the pack has on other packs. Multiple dependencies can be supplied.
- "dependency {source}" - The source URL for this dependency.

An example `metadata.hcl` file:

```
app {
  url = "https://github.com/mikenomitch/hello_world_server"
  author = "Mike Nomitch"
}

pack {
  name = "hello_world"
  description = "This pack contains a single job that renders hello world, or a different greeting, to the screen."
  url = "https://github.com/hashicorp/nomad-pack-community-registry/hello_world"
  version = "0.3.2"
}
```

#### variables.hcl

The `variables.hcl` file defines the variables required to fully render and deploy all the templates found within the "templates" directory.

These variables are defined using [HCL](https://github.com/hashicorp/hcl).

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

The `outputs.tpl` is an optional file that defines an output to be printed when a pack is deployed.

Output files have access to pack variables and template helper functions. A simple example:

```
Congrats on deploying [[ .nomad_pack.pack.name ]].

There are [[ .hello_world.app_count ]] instances of your job now running on Nomad.
```

#### README and CHANGELOG

No specific format is required for the `README.md` or `CHANGELOG.md` files.

## Step Three: Write the Templates

Each file at the top level of the `templates` directory that uses the extension ".nomad.tpl" defines a resource (such as a job) that will be applied to Nomad. These files can use any UTF-8 encoded prefix as the name.

Helper templates, which can be included within larger templates, have names prefixed with an underscore “\_” and use a ".tpl" extension.

When deploying, Nomad Pack will render each resource template using the variables provided and apply it to Nomad.

#### Template Basics

Templates are written using [Go Template Syntax](https://learn.hashicorp.com/tutorials/nomad/go-template-syntax). This enables templates to have complex logic where necessary.

Unlike default Go Template syntax, Nomad Pack uses "[[" and "]]" as delimiters.

An example template using variables values from above:

```
job "hello_world" {
  region      = "[[ .hello_world.region ]]"
  datacenters = [ [[ range $idx, $dc := .hello_world.datacenters ]][[if $idx]],[[end]][[ $dc | quote ]][[ end ]] ]
  type = "service"

  group "app" {
    count = [[ .hello_world.count ]]

    network {
      port "http" {
        static = 80
      }
    }

    [[/* this is a go template comment */]]

    task "server" {
      driver = "docker"
      config {
        image = "mikenomitch/hello-world"
        network_mode = "host"
        ports = ["http"]
      }

      resources {
        cpu    = [[ .hello_world.resources.cpu ]]
        memory = [[ .hello_world.resources.memory ]]
      }
    }
  }
}
```

This is a relatively simple template that mostly sets variables.

The `datacenters` value shows slightly more complex Go Template, which allows for [control structures](https://learn.hashicorp.com/tutorials/nomad/go-template-syntax#control-structure-list) like `range` and [pipelines](https://learn.hashicorp.com/tutorials/nomad/go-template-syntax#pipelines).

#### Template Functions

To supplement the standard Go Template set of template functions, the (masterminds/sprig)[https://github.com/Masterminds/sprig] library is used. This adds helpers for various use cases such as string manipulation, cryptographics, and data conversion (for instance to and from JSON).

Custom Nomad-specific and debugging functions are also provided:

- `nomadRegions` returns the API object from `/v1/regions`.
- `nomadNamespaces` returns the API object from `/v1/namespaces`.
- `nomadNamespace` takes a single string parameter of a namespace ID which will be read via `/v1/namespace/:namespace`.
- `spewDump` dumps the entirety of the passed object as a string. The output includes the content types and values. This uses the `spew.SDump` function.
- `spewPrintf` dumps the supplied arguments into a string according to the supplied format. This utilises the `spew.Printf` function.
- `fileContents` takes an argument to a file of the local host, reads its contents and provides this as a string.

A custom function within a template is called like any other:

```
[[ nomadRegions ]]
[[ nomadRegions | spewDump ]]
```

#### Helper templates

For complex packs, authors may want to reuse template snippets across multiple resources.

For instance, if we had two jobs defined in a pack, and we knew both would re-use the same `region`
logic for both, we could use a helper template to consolidate logic.

Helper template names are prepended with an underscore "\_" and end in ".tpl". So we could define a helper called "\_region.tpl":

```
[[- define "region" -]]
[[- if not (eq .hello_world.region "") -]]
region = [[ .hello_world.region | quote]]
[[- end -]]
[[- end -]]
```

Then in the parent templates, "job_a.nomad.tpl" and "job_b.nomad.tpl", we would render to the helper template using its name:

```
job "job_a" {
  type = "service"

  [[template "region"]]

  ...etc...
}
```

#### Pack Dependencies

Packs can depend on content from other packs.

First, packs must define their dependencies in `metadata.hcl`. A pack stanza with a dependency would look like the following:

```
app {
  url = "https://some-url-for-the-application.dev"
  author = "Borman Norlaug"
}

pack {
  name = "simple_service"
  description = "This pack contains a simple service job, and depends on another pack."
  url = "https://github.com/hashicorp/nomad-pack-community-registry/simple_service"
  version = "0.2.1"
}

dependency "demo_dep" {
  name   = “demo_dep”
  source = “git://source.git/packs/demo_dep”
}
```

This would allow templates of "simple_service" to use "demo_dep"'s helper templates in the following way:

```
[[ template "demo_dep.data" . ]]
```

## Step Four: Testing your Pack

As you write your pack, you will probably want to test it. To do this, pass the
directory path as the name of the pack to the `run`, `plan`, `render`, `info`, 
`stop`, or `destroy` commands. Relative paths are supported.

For instance, if your current working directory is the directory where you are
developing your pack, and `nomad-pack` is on your path, you can run this command.

```
nomad-pack run .
```

Packs added this way will show up in output with a `dev` registry and `dev` ref.

## Step Five: Publish and Find your Custom Repository

To use your new pack, you will likely want to publish it to the internet. Push the git repository to a URL
accessible by your command line tool.

If you wish to share your packs, please consider adding them to the
[Nomad Pack Community Registry](https://github.com/hashicorp/nomad-pack-community-registry)

If you don't want to host a pack registry in version control, you can work locally
using the filesystem method.

## Step Six: Deploy your Custom Pack!

Add your custom repository using the `nomad-pack registry add` command.

```
nomad-pack registry add my_packs github.com/<YOUR_ORG>/<YOUR_REPO>
```

Deploy your custom packs.

```
nomad-pack run hello_world --var app_count=1 --registry=my_packs
```

Congrats! You can now write custom packs for Nomad!
