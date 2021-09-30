# Detailed Nomad Pack Usage

This guide will go into detail on Nomad Pack usage and command details.

For an overview on basic usage, see the repository [README](../README.md).

For more information on writing custom packs and registries, see the repository [Writing Packs Documentation](./writing-packs.md).

## Init

The `init` command creates a directory at `./.nomad/packs` to store information about availible packs and packs in use.

During initializing, Nomad Pack downloads a default registry of packs from [https://github.com/hashicorp/nomad-pack-registry](https://github.com/hashicorp/nomad-pack-registry).

This can be overridden by using the `--from` flag when running the `init` command. For instance, to use the Community Registry instead of the default, you could run:

```
nomad-pack init --from git@github.com/hashicorp/nomad-pack-community-registry
```

The directory structure is as follows:

```
.nomad
└── packs
    ├── <SOURCE-ORG-REGISTRY>
        ├── <PACK-NAME>
            ├── <PACK-VERSION>
                ├── ...files containing pack contents...
```

The contents of the `.nomad/pack` directory are needed for Nomad Pack to work properly, but users will not have to actively manage or change these files.

## List

The `list` command lists the packs availible to deploy.

```
nomad-pack list
```

This command reads from the `.nomad/packs` directory explained above.

## Render

At times, you may wish to use Nomad Pack to render jobspecs, but you will not want to immediately deploy these to Nomad.

This can be useful when writing a pack, debugging deployments, integrating Nomad Pack into a CI/CD environment, or if you have another mechanism for handlign Nomad deploys.

The `render` command takes the `--var` and `--var-file` flags that `run` takes.

The `--too` flag determines the directory where the rendered templates will be written.

The `--render-output-template` can be passed to additionally render the output template. Some output templates rely on an deployment for information. In these cases, the output template may not be rendered with all necessary information.

```
nomad-pack render hello-world --to ./tmp --var greeting=hola --render-output-template
```

## Run

To deploy all of the resources in a pack to Nomad, use the `run` command.

```
nomad-pack run hello-world
```

By passing a `--name` value into `run`, Nomad Pack deploy each resource in the pack with a metadata value for "pack name". If no name is given, the pack name is used by default.

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
nomad-pack run hello-world --var-file ./my-variables.hcl
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

The `plan` command takes the `--var` and `--var-file` flags like the `run` command.

```
nomad-pack plan hello-world --var greeting=hallo
```

```
nomad-pack plan hello-world --var-file ./my-variables.hcl
```

## Destroy

If you want to remove all of the resources deployed by a pack, run the `destroy` command with the pack name.

```
nomad-pack destroy hello-world
```

If you deployed the pack with a name override, pass in the name you gave the pack. For instance, if you deployed with the command:

```
nomad-pack run hello-world --name hola-mundo
```

You would destroy the contents of that pack with the command;

```
nomad-pack destroy hola-mundo
```
