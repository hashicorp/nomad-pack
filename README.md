# Nomad Pack

Nomad Pack is currently a Tech Preview.

Nomad Pack is a templating and packaging tool used with [HashiCorp Nomad](https://www.nomadproject.io).

Nomad Pack is used to:

- Easily deploy popular applications to Nomad
- Re-use common patterns across internal applications
- Find and share job specifications with the Nomad community

## Installation

To simplify the getting started experience, you can download a [precompiled binary][hashicorp_releases]
and run it on your machine locally. After downloading Nomad Pack, unzip the package. Make sure that
the nomad binary is available on your PATH. You can inspect the locations available on your path by
running this command:
```
$ echo $PATH
/usr/local/bin:/usr/bin:/bin:/usr/sbin:/sbin
```

The output is a list of locations separated by colons. You can make Nomad Pack available by moving
the binary to one of the listed locations, or by adding Nomad Pack's location to your PATH.

Nomad Pack is also available as a [Docker image][docker_hub]. With Docker installed on your local
machine, you can pull the latest image by running the following command:
```
$ docker pull hashicorp/nomad-pack:0.0.1-techpreview1
```

## Usage

### Dependencies

Nomad Pack users must have Nomad running and accessible at the address defined in the `NOMAD_ADDR`
environment variable.

If Nomad ACLs are enabled, a token with proper permissions must be defined in the `NOMAD_TOKEN`
environment variable.

### Basic Use


First, run the `registry list` command to see which packs are available to deploy.

```
nomad-pack registry list
```

To deploy one of these packs, use the `run` command. This deploys each jobs defined in the pack to Nomad.
To deploy the `hello_world` pack, you would run the following command:

```
nomad-pack run hello_world
```

Each pack defines a set of variables that can be provided by the user. To get information on the pack
and to see which variables can be passed in, run the `info` command.

```
nomad-pack info hello_world
```

Values for these variables are provided using the `--var` flag.

```
nomad-pack run hello_world --var message=hola
```

Values can also be provided by passing in a variables file. See the variables section of the
[Detailed usage guide](/docs/detailed-usage.md) for details.

```
tee -a ./my-variables.hcl << END
message=bonjour
END

nomad-pack run hello_world -f ./my-variables.hcl
```

If you want to remove all of the resources deployed by a pack, run the `destroy` command with the
pack name.

```
nomad-pack destroy hello_world
```

### Adding Non-Default Pack Registries

When using Nomad Pack, the default registry for packs is
[the Nomad Pack Community Registry](https://github.com/hashicorp/nomad-pack-community-registry).
Packs from this registry will be made automatically availible.

You can add additional registries by using the `registry add` command. For instance, if you wanted
to add [an example Gitlab registry](https://gitlab.com/mikenomitch/pack-registry),
you would run the following command to download the registry.

```
nomad-pack registry add example gitlab.com/mikenomitch/pack-registry
```

To view the packs you can now deploy, run the `registry list` command.

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

The [Nomad Pack Community Registry](https://github.com/hashicorp/nomad-pack-community-registry) is
a public registry for community-maintained packs. Nomad community members are encouraged to share
their packs and collaborate with one anothr in this repo.

Pull Requests and feedback on both repositories are welcome!

## Upcoming Features and Changes

- Support for Volumes and ACLs
- Support for other Vesion Control Systems
- Pack search command
- Integration into the official Nomad CLI

## Additional Documentaion

- [Detailed Usage Guide](/docs/detailed-usage.md)
- [How to Write Your Own Pack](/docs/writing-packs.md)
- [Contributing](/docs/contributing.md)

<!-- TODO: add a direct link to the guides when availible -->

<!-- ## Tutorials

Nomad Pack Guides are available on [HashiCorp Learn](https://learn.hashicorp.com/nomad). -->

[hashicorp_releases]: (https://releases.hashicorp.com/nomad-pack/)
[docker_hub]: (https://hub.docker.com/r/hashicorp/nomad-pack)
