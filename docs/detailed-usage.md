# Detailed Nomad Pack Usage

This guide will go into detail on Nomad Pack usage and commands.

For an overview on basic use, see the repository [README](../README.md).

For more information on writing custom packs and registries, see the [Writing Packs Guide](./writing-packs.md)
in th repository or in the [HashiCorp Learn Guides](https://learn.hashicorp.com/nomad).

<!--  TODO: Get the link to the writing own packs guide once it is up  -->

## Initialization

When first using Nomad Pack, a directory at `./.nomad/packs` will be created to store information about available packs and packs in use.

During initializing, Nomad Pack downloads a default registry of packs from [https://github.com/hashicorp/nomad-pack-community- registry](https://github.com/hashicorp/nomad-pack-community-registry).

The directory structure is as follows:

```
.nomad
└── packs
    ├── <REGISTRY>
        ├── <PACK-NAME>
            ├── <PACK-REF>
                ├── ...files containing pack contents...
```

The contents of the `.nomad/pack` directory are needed for Nomad Pack to work properly, 
but users must not manually manage or change these files. Instead, use the `registry`
commands.

## List

The `registry list` command lists the packs available to deploy.

```
nomad-pack registry list
```

This command reads from the `.nomad/packs` directory explained above.

## Adding new Registries and Packs

The `registry` command includes several sub-commands for interacting with registries.

Custom registries can be added using the `registry add` command. Any `git` based
registry supported by [`go-getter`](https://github.com/hashicorp/go-getter) should
work.

For instance, if you wanted to add the entire [Nomad Pack Community Registry](https://github.com/hashicorp/nomad-pack-community-registry),
you would run the following command to download the registry.

```
nomad-pack registry add community github.com/hashicorp/nomad-pack-community-registry
```

To add a single pack from the registry, use the `--target` flag.

```
nomad-pack registry add community github.com/hashicorp/nomad-pack-community-registry --target=nginx
```

To download single pack or an entire registry at a specific version/SHA, use the `--ref` flag.

```
nomad-pack registry add community github.com/hashicorp/nomad-pack-community-registry --ref=v0.0.1
```

To remove a registry or pack from your local cache. Use the `registry delete` command.
This command also supports the `--target` and `--ref` flags.

```
nomad-pack registry delete community
```

## Render

At times, you may wish to use Nomad Pack to render jobspecs, but you will not want to immediately deploy these to Nomad.

This can be useful when writing a pack, debugging deployments, integrating Nomad Pack into a CI/CD environment, or if you have another mechanism for handlign Nomad deploys.

The `render` command takes the `--var` and `--var-file` flags that `run` takes.

The `--too` flag determines the directory where the rendered templates will be written.

The `--render-output-template` can be passed to additionally render the output template. Some output templates rely on a deployment for information. In these cases, the output template may not be rendered with all necessary information.

```
nomad-pack render hello-world --to ./tmp --var greeting=hola --render-output-template
```

## Run

To deploy the resources in a pack to Nomad, use the `run` command.

```
nomad-pack run hello-world
```

By passing a `--name` value into `run`, Nomad Pack deploys each resource in the 
pack with a metadata value for "pack name". If no name is given, the pack name 
is used by default.

This allows Nomad Pack to manage multiple deployments of the same pack.

```
nomad-pack run hello-world --name hola-mundo
```

### Variables

Each pack defines a set of variables that can be provided by the user. Values for variables can be passed into the `run` command using the `--var` flag.

```
nomad-pack run hello-world --var greeting=hola
```

Values can also be provided by passing in a variables file.

```
nomad-pack run hello-world -f ./my-variables.hcl
```

These files can define overrides to the variables defined in the pack.

```
docker_image = "hashicorp/hello-world"

app_count = 3

datacenters = [
  "us-east-1",
  "us-west-2",
]

app_resources = {
  memory = 512
  cpu = 256
}
```

To see the type and description of each variable, run the `info` command.

```
nomad-pack info hello-world
```

## Plan

If you do not want to immediately deploy the pack, but instead want details on how it will be deployed, run the `plan` command.

This invokes Nomad in a dry-run mode using the [Nomad Plan](https://www.nomadproject.io/api-docs/jobs#create-job-plan) API endpoint.

```
nomad-pack plan hello-world
```

By passing a `--name` value into plan, Nomad Pack will look for packs deployed with that name. If no name is provided, Nomad Pack uses the pack name by default.

```
nomad-pack plan hello-world --name hola-mundo
```

The `plan` command takes the `--var` and `-f` flags like the `run` command.

```
nomad-pack plan hello-world --var greeting=hallo
```

```
nomad-pack plan hello-world -f ./my-variables.hcl
```

## Status
If you want to see a list of the packs currently deployed (this may include packs that are stopped but not yet removed), run the `status` command.

```
nomad-pack status
```

To see the status of jobs running in a specific pack, use the `status` command with the pack name.

```
nomad-pack status hello-world
```

## Destroy

If you want to remove the resources deployed by a pack, run the `destroy` command with the pack name.

```
nomad-pack destroy hello-world
```

If you deployed the pack with a `--name` value, pass in the name you gave the pack. For instance, if you deployed with the command:

```
nomad-pack run hello-world --name hola-mundo
```

You would destroy the contents of that pack with the command;

```
nomad-pack destroy hello-world --name hola-mundo
```

If you deployed the pack with variable overrides that override the job name in a pack, pass in those same overrides. For example,
if you deployed with the command:

```
nomad-pack run hello-world --name hola-mundo --var job_name=spanish
```

You would destroy the contents of that pack with the command:

```
nomad-pack destroy hello-world --name hola-mundo --var job_name=spanish
```

It's possible to deploy multiple instances of a pack using the same `--name` value but with different job names using variable
overrides. For example, you can run the following commands, which will create two jobs,
one named "spanish" and one named "hola":

```
nomad-pack run hello-world --name hola-mundo --var job_name=spanish
nomad-pack run hello-world --name hola-mundo --var job_name=hola
```

If you run the destroy command without including the variable overrides, the command will destroy both jobs, since by default
nomad pack will target all jobs belonging to the specified pack and deployment name.
```
# This destroys both jobs: "spanish" and "hola"
nomad-pack destroy hello-world --name hola-mundo
```

If you only want to destroy one of the jobs, you need to include the variable overrides so nomad pack knows which job to target:
```
# This destroys the job named "spanish"
nomad-pack destroy hello-world --name hola-mundo --var job_name=spanish
```

## Stop

To stop the jobs without completely removing them from Nomad completely, use the `stop` command:

```
nomad-pack stop hola-mundo
```

N.B. The `destroy` command is an alias for `stop --purge`.
