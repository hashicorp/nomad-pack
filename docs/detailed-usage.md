# Detailed Nomad Pack Usage

This guide goes more deeply into detail on Nomad Pack usage and commands.

For an overview on basic use, see the repository [README](../README.md).

For more information on writing custom packs and registries, see the [Writing Packs Guide](./writing-packs.md)
in the repository or the [Writing Custom Packs tutorial][writing-packs-tut] at developer.hashicorp.com.

## Initialization

The first time you run registry list, Nomad Pack will add a `nomad/packs`
directory to your desktop user's cache directory—`$XDG_CACHE_DIR` on Linux,
`~/Library/Caches` on macOS, `%AppData%` on Windows, etc. This folder stores
information about cloned registries and their available packs.

During initializing, Nomad Pack downloads a default registry of packs from the
[Nomad Pack community registry][].

The directory structure is as follows:
<!-- TODO: this will need to be trued up based on PR -->
```plaintext
nomad
└── packs
    ├── <REGISTRY>
        ├── <PACK-NAME>
            ├── <PACK-REF>
                ├── ...files containing pack contents...
```

The `nomad/packs` directory's contents are managed by Nomad Pack. Users should
not manually manage or change these files. Instead, use the `registry` commands.

## List

The `registry list` command lists the packs available to deploy.

```shell
nomad-pack registry list
```

This command reads from the `nomad/packs` directory explained earlier.

## Add new registries and packs

The `registry` command includes several sub-commands for interacting with registries.

Custom registries can be added using the `registry add` command. Any `git` based
registry supported by [`go-getter`][] should work.

For instance, if you wanted to add the entire [Nomad Pack community registry][],
you would run the following command to download the registry.

```shell
nomad-pack registry add community github.com/hashicorp/nomad-pack-community-registry
```

To add a single pack from the registry, use the `--target` flag.

```shell
nomad-pack registry add community github.com/hashicorp/nomad-pack-community-registry --target=nginx
```

To download single pack or an entire registry at a specific version/SHA, use the `--ref` flag.

```shell
nomad-pack registry add community github.com/hashicorp/nomad-pack-community-registry --ref=v0.0.1
```

To remove a registry or pack from your local cache. Use the `registry delete` command.
This command also supports the `--target` and `--ref` flags.

```shell
nomad-pack registry delete community
```

## Render a pack

At times, you may wish to use Nomad Pack to render job specifications, but don't
want to immediately deploy these to Nomad.

This can be useful when writing a pack, debugging deployments, integrating Nomad
Pack into a CI/CD environment, or if you have another mechanism for handling
deployments to Nomad.

Like the `run` command, the `render` command takes the `--var` and `--var-file`
flags to provide variable values and overrides.

The `--to-dir` flag determines the directory where the rendered templates will
be written.

Packs can create multiple output files. When running the `render` command
without the `--to-dir` flag the `render` command outputs all the rendered pack
templates to the terminal underneath their file names. When rendering a pack's
output to disk, you should use the `--to-dir` flag.

The `--render-output-template` can be passed to additionally render the output
template. Some output templates rely on a deployment for information. In these
cases, the output template may not be rendered with all necessary information.

```shell
nomad-pack render hello-world --to-dir ./tmp --var greeting=hola --render-output-template
```

## Run a pack

To deploy the resources in a pack to Nomad, use the `run` command.

```shell
nomad-pack run hello-world
```

<!-- TODO: awkward -->
By passing a `--name` value into `run`, Nomad Pack deploys each resource in the
pack with a metadata value for "pack name". If no name is given, the pack name
is used by default.

This allows Nomad Pack to manage multiple deployments of the same pack.

```shell
nomad-pack run hello-world --name hola-mundo
```

It's also possible to run a local pack directly from the pack directory by passing in the directory instead of the pack name.

```shell
nomad pack run .
```

### Provide values for pack variables

Each pack defines a set of variables that can be provided by the user. Values for variables can be passed into the `run` command using the `--var` flag.

```shell
nomad-pack run hello-world --var greeting=hola
```

Values can also be provided by passing in a variables file.

```shell
nomad-pack run hello-world -f ./my-variables.hcl
```

These files can define overrides to the variables defined in the pack.

```hcl
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

```shell
nomad-pack info hello-world
```

## Plan a pack

If you do not want to immediately deploy the pack, but instead want details on how it will be deployed, run the `plan` command.

This invokes Nomad in a dry-run mode using the Nomad's [Create Job Plan][nomad-plan] API endpoint.

```shell
nomad-pack plan hello-world
```

By passing a `--name` value into plan, Nomad Pack will look for packs deployed with that name. If no name is provided, Nomad Pack uses the pack name by default.

```shell
nomad-pack plan hello-world --name hola-mundo
```

The `plan` command takes the `--var` and `-f` flags like the `run` command.

```shell
nomad-pack plan hello-world --var greeting=hallo
```

```shell
nomad-pack plan hello-world -f ./my-variables.hcl
```

## Get the status of running packs

If you want to see a list of the packs currently deployed (this may include packs that are stopped but not yet removed), run the `status` command.

```shell
nomad-pack status
```

To see the status of jobs running in a specific pack, use the `status` command with the pack name.

```shell
nomad-pack status hello-world
```

## Stop a running pack

To stop the jobs without completely removing them from Nomad completely, use the `stop` command:

```shell
nomad-pack stop hola-mundo
```

If you deployed the pack with a `--name` value, you must also provide the name
you gave the pack. For instance, to stop a `hello-world` pack having the name
`hola-mundo`, you would run the following command.

```shell
nomad-pack stop hello-world --name hola-mundo
```

When you have deployed a pack with variable overrides that override the job name,
you must pass in those same overrides when stopping the pack. The `hello-world`
pack allows you to override the Nomad job name and can be used to demonstrate
this. If you deploy it with the following command.

```shell
nomad-pack run hello-world --name hola-mundo --var job_name=spanish
```

Running the following command stops the job instance named `spanish`.

```shell
nomad-pack stop hello-world --name hola-mundo --var job_name=spanish
```

It's also possible to deploy multiple instances of a pack using the same
`--name` value but with different job names using variable overrides.
For example, you can run the following commands, which will create two different
jobs, one named "spanish" and one named "hola".

```shell
nomad-pack run hello-world --name hola-mundo --var job_name=spanish
nomad-pack run hello-world --name hola-mundo --var job_name=hola
```

If you run the destroy command without including the variable overrides, Nomad
Pack destroys both jobs, since by default it targets all jobs belonging to the
specified pack and deployment name.

```shell
# This stops both jobs: "spanish" and "hola"
nomad-pack stop hello-world --name hola-mundo
```

In this case, you must also include the variable overrides so that Nomad Pack
targets the correct instance.

```shell
# This stops the job named "spanish"
nomad-pack stop hello-world --name hola-mundo --var job_name=spanish
```

## Destroy a running pack

If you want to purge the resources deployed by a pack, run the `destroy` command with the pack name.

```shell
nomad-pack destroy hello-world
```

If you deployed the pack with a `--name` value, pass in the name you gave the pack. For instance, if you deployed with the command:

```shell
nomad-pack run hello-world --name hola-mundo
```

You would destroy the contents of that pack with the command;

```shell
nomad-pack destroy hello-world --name hola-mundo
```

When you deploy a pack with variable overrides that override the job name, pass
in those same overrides. The `hello-world` job is one such job. If you deploy
it with the following command.

```shell
nomad-pack run hello-world --name hola-mundo --var job_name=spanish
```

You would destroy a running instance of that pack with the following command.

```shell
nomad-pack destroy hello-world --name hola-mundo --var job_name=spanish
```

It's also possible to deploy multiple instances of a pack using the same
`--name` value but with different job names using variable overrides.
For example, you can run the following commands, which will create two different
jobs, one named "spanish" and one named "hola".

```shell
nomad-pack run hello-world --name hola-mundo --var job_name=spanish
nomad-pack run hello-world --name hola-mundo --var job_name=hola
```

If you run the destroy command without including the variable overrides, Nomad
Pack destroys both jobs, since by default it targets all jobs belonging to the
specified pack and deployment name.

```shell
# This destroys both jobs: "spanish" and "hola"
nomad-pack destroy hello-world --name hola-mundo
```

In this case, you must also include the variable overrides so that Nomad Pack
targets the correct instance.

```shell
# This destroys the job named "spanish"
nomad-pack destroy hello-world --name hola-mundo --var job_name=spanish
```

N.B. The `destroy` command is an alias for `stop --purge`.

[Nomad Pack community registry]: https://github.com/hashicorp/nomad-pack-community-registry
[`go-getter`]: https://github.com/hashicorp/go-getter
[nomad-plan]: https://www.nomadproject.io/api-docs/jobs#create-job-plan
[writing-packs-tut]: https://developer.hashicorp.com/nomad/tutorials/job-specifications/nomad-pack-writing-packs
