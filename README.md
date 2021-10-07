# Nomad Pack

Nomad Pack is currently a Tech Preview.

Nomad Pack is a templating and packaging tool used with [HashiCorp Nomad](https://www.nomadproject.io).

Nomad Pack is used to:

- Easily deploy popular applications to Nomad
- Re-use common patterns across internal applications
- Find and share job specifications with the Nomad community

## Usage

### Dependencies

Nomad Pack users must have Nomad running and accessible at the address defined in the `NOMAD_ADDR`
environment variable.

If Nomad ACLs are enabled, a token with proper permissions must be defined in the `NOMAD_TOKEN`
environment variable.

<!-- TODO: Add this section once we know how to download it -->
<!-- ### Downloading Nomad Pack -->

### Basic Use

In order to use Nomad Pack, first run the `init` command. This add a directory at `./.nomad/packs`
to store information about available packs.

```
nomad-pack init
```

Next, run the `list` command to see which packs are available to deploy. This will show packs
that are available to deploy.

```
nomad-pack list
```

To deploy one of these packs, run the `run` command to deploy the jobs in the pack to Nomad. For
instance, to deploy the `hello_world` pack, you would run the command:

```
nomad-pack run hello_world
```

Each pack defines a set of variables that can be provided by the user. To get information on the pack
and to see what variables can be passed in, run the `info` command.

```
nomad-pack info hello_world
```

Values for these variables can be passed into the `run` command using the `--var` flag.

```
nomad-pack run hello_world --var greeting=hola
```

Values can also be provided by passing in a variables file. See the variables section of the
[Detailed usage guide](/docs/detailed-usage.md) for details.

```
nomad-pack run hello_world --var-file ./my-variables.hcl
```

If you want to remove all of the resources deployed by a pack, run the `destroy` command with the
pack name.

```
nomad-pack destroy hello_world
```

### Adding Non-Default Pack Registries

When using Nomad Pack, the default registry for packs is
[https://github.com/hashicorp/nomad-pack-registry](https://github.com/hashicorp/nomad-pack-registry).
Packs from this registry will be made automatically availible.

You can add additional registries by using the `registry add` command. For instance, if you wanted
to add the [Nomad Pack Community Registry](https://github.com/hashicorp/nomad-pack-community-registry),
you would run the following command to download the registry.

```
nomad-pack registry add --from=github.com/hashicorp/nomad-pack-community-registry
```

To view the packs you can now deploy, run `list` or `registry list` command.

```
nomad-pack registry list
```

Packs from this registry can now be deployed using the `run` command.

### Writing your own Packs

Nomad Pack is valuable when used with official and community packs, but many users will also want to
use their own.

Converting your existing Nomad job specifications into reusable packs is achievable in a few steps,
see the [Writing Packs documentation](/docs/writing-packs.md) for more details.

## Pack Registries

Packs are organized into "registries" which contain multiple packs and shared templates.

### Pack Registries

Nomad Pack is initialized with a default pack registries found at
[https://github.com/hashicorp/nomad-pack-registry](https://github.com/hashicorp/nomad-pack-registry).
Packs in this repository are vetted and maintained by the Nomad team at HashiCorp.

The [Nomad Pack Community Registry](https://github.com/hashicorp/nomad-pack-community-registry) is
an alternative registry for community-maintained packs. Nomad community members are
encouraged to share their packs and collaborate with one anothr in this repo.

Pull Requests and feedback on both repositories are welcome!

## Upcoming Features and Changes

- Integration into the official Nomad CLI
- Support for Volumes and ACLs
- Support for other Vesion Control Systems

## Additional Documentaion

- [How to Write Your Own Pack](/docs/writing-packs.md)
- [Detailed Usage Guide](/docs/detailed-usage.md)

## Tutorials

<!-- TODO: add a direct link to the guides when availible -->

Nomad Pack Guides are available on [HashiCorp Learn](https://learn.hashicorp.com/nomad).
