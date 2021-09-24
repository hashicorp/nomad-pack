# Nomad Pack

Nomad Pack is a templating and packaging tool used with [HashiCorp Nomad](https://www.nomadproject.io).

Nomad Pack is used to:

- Easily deploy popular applications to Nomad
- Re-use common patterns across internal applications
- Find and share jobspecs with the Nomad community

Nomad Pack is currently in beta.

## Usage

#### Dependencies

Nomad Pack users must have Nomad running and accessible at the address defined in the `NOMAD_ADDR` environment variable.

If Nomad ACLs are enabled, a token with proper permissions must be defined in the `NOMAD_TOKEN` environment variable.

<!-- #### Downloading Nomad Pack -->

<!-- TODO: Add this section once we know how -->

#### Basic Use

In order to use Nomad Pack, first run the `init` command. This add a directory at `./.nomad/packs` to store information about availible packs.

`nomad-pack init`

Next, run the `list` command to see which packs are availible to deploy.

`nomad-pack list`

To deploy one of these packs, run the `run` command to deploy the jobs in the pack to Nomad. For instance, to deploy the `hello-world` pack, you would run the command:

`nomad-pack run hello-world`

Each pack defines a set of variables that can be provided by the user. To get information on the pack and to see what variables can be passed in, run the `info` command.

`nomad-pack info hello-world`

Values for these variables can be passed into the `run` command using the `-var` flag.

`nomad-pack run hello-world -var greeting=hola`

Values can also be provided by passing in a variables file. See the variables section of the [advanced usage guide](/docs/advanced-usage.md) for details.

`nomad-pack run hello-world -var-file ./my-variables.hcl`

If you do not want to immediately deploy the pack, but instead want details on how it will be deployed, run the `plan` command.

`nomad-pack plan hello-world`

If you want to remove all of the resources deployed by a pack, run the `destroy` command with the pack name.

`nomad-pack destroy hello-world`

#### Adding Non-Default Pack Repositories

When initializing Nomad Pack, the default repository for packs is [https://github.com/hashicorp/nomad-pack-registry](https://github.com/hashicorp/nomad-pack-registry).

This can be overridden by using the `--from` flag when running the `init` command. For instance, to use the Community Registry instead of the default, you could run:

`nomad-pack init --from git@github.com/hashicorp/nomad-pack-community-registry`

You can add additional repositories by using the `add` command. For instance, if you wanted to add your own repository that you call `my-packs`, you could run the command:

`nomad-pack add my-packs git@github.com/<YOUR_ORG>/<YOUR_REPO>`

To deploy packs from that repo, you would add `my-packs` before the pack name. For instance:

`nomad-pack run my-packs:grafana`

#### Writing your own Packs

Nomad Pack is valuable when used with official and community packs, but many users will also want to use their own.

Converting your existing Nomad jobspecs into reusable packs is achievable in a few steps:

- Make a Pack Repository. Cloning [hashicorp/example-pack-repository](https://github.com/hashicorp/example-pack-repository) can give you a head start.
- Add a new top-level directory for your pack.
- Add a metadata.hcl file, a variables.hcl file, and an optional output.tpl file.
- Add any jobspecs you wish to deploy into the `templates` subdirectory in your pack.
- Add variables and templating logic to your jobspec files using [Go Template Syntax](https://learn.hashicorp.com/tutorials/nomad/go-template-syntax).
- Push your changes.
- Add your custom repository using the `nomad-pack add` command.
- Deploy your custom packs!

For a detailed description of custom repository structure, see the [custom packs documentation](/docs/custom-packs).

## Pack Repositories

Packs are organized into "repositories" or "repos" which contain multiple packs and shared templates.

#### Official Pack Repository

Nomad Pack is initialized with a default pack repository found at [https://github.com/hashicorp/nomad-pack-registry](https://github.com/hashicorp/nomad-pack-registry).

#### Community Pack Repositories

The Nomad community is encouraged to share their pack repositories. The following is a list of community-managed pack respositories:

- [Nomad Pack Community Registry](https://github.com/hashicorp/nomad-pack-community-registry) a semi-official repository of community-written packs.
- [Your Repository Here] - Pull Requests welcome!
<!-- Dear Community Members, add you Pack Repository above with a name, link, and a brief description. -->

## Upcoming Features and Changes

<!-- TODO: add to this list as we see fit -->

- Integration into the official Nomad CLI
- Support for Volumes and ACLs

## Additional Documentaion

<!-- TODO: Create these -->

- [Writing Custom Packs](/docs/custom-packs)
- [Advanced Usage Guide](/docs/advanced-usage)

## Tutorials

<!-- TODO: add a direct link to the guides when availible -->

Nomad Pack Guides are available on [HashiCorp Learn](https://learn.hashicorp.com/nomad).
