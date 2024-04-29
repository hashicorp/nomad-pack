## UNRELEASED

## 0.1.1 (April 26,2024)

IMPROVEMENTS:
* build: Improve OS/Architecture detection [[GH-499](https://github.com/hashicorp/nomad-pack/pull/499)]
* build: Update Go version to 1.21.5 [[GH-478](https://github.com/hashicorp/nomad-pack/pull/478)] 
* build: Update Docker build dependencies [[GH-421](https://github.com/hashicorp/nomad-pack/pull/421)]
* release: Add license (MPL) to release artifacts [[GH-498](https://github.com/hashicorp/nomad-pack/pull/498)]

BUG FIXES:
* cli: Update help text for `registry list` [[GH-486](https://github.com/hashicorp/nomad-pack/pull/486)]
* runner: Update nomad/api to fix parsing of consul.service.cluster field [[GH-479](https://github.com/hashicorp/nomad-pack/pull/479)]
* runner: updated to hashicorp/nomad@1.7.2 to support `action` blocks [[GH-476](https://github.com/hashicorp/nomad-pack/pull/476)]
* docs: Update README.md to reflect GA status [[GH-501](https://github.com/hashicorp/nomad-pack/pull/501)]
* docs: Update usage documentation [[GH-484](https://github.com/hashicorp/nomad-pack/pull/484)]

DEPENDENCY CHANGES:
* deps: Bump github.com/docker/docker from 25.0.2+incompatible to 25.0.5+incompatible [[GH-502](https://github.com/hashicorp/nomad-pack/pull/502)]
* deps: Bump hashicorp/nomad/api from v0.0.0-20231219145541-859606a54ade to v0.0.0-20240422165847-3ac3bc1cfede [[GH-500](https://github.com/hashicorp/nomad-pack/pull/500)]
* deps: Bump github.com/go-jose/go-jose/v3 from 3.0.1 to 3.0.3 [[GH-491](https://github.com/hashicorp/nomad-pack/pull/491)]
* deps: Bump golang.org/x/net from 0.19.0 to 0.23.0 [[GH-497](https://github.com/hashicorp/nomad-pack/pull/497)]
* deps: Bump github.com/docker/docker [[GH-485](https://github.com/hashicorp/nomad-pack/pull/485)]
* deps: Bump github.com/opencontainers/runc from 1.1.8 to 1.1.12 [[GH-483](https://github.com/hashicorp/nomad-pack/pull/483)]
* deps: Bump github.com/cloudflare/circl from 1.3.3 to 1.3.7 [[GH-482](https://github.com/hashicorp/nomad-pack/pull/482)]
* deps: Bump github.com/containerd/containerd from 1.6.18 to 1.6.26 [[GH-480](https://github.com/hashicorp/nomad-pack/pull/480)]
* deps: Bump github.com/go-git/go-git/v5 from 5.8.1 to 5.11.0 [[GH-481](https://github.com/hashicorp/nomad-pack/pull/481)]
* deps: Bump golang.org/x/crypto from 0.16.0 to 0.17.0 [[GH-477](https://github.com/hashicorp/nomad-pack/pull/477)]
* deps: Bump github.com/go-jose/go-jose/v3 from 3.0.0 to 3.0.1 [[GH-474](https://github.com/hashicorp/nomad-pack/pull/474)]

## 0.1.0 (October 31, 2023)

* **Generate Variable Override Files for Packs** - With
`nomad-pack generate var-file`, you can create a documented variable override
file as a starting point for customizing a Nomad Pack for deploying in your
environment.

* **Templating Non-Job Files** - Pack authors can now add non-job templates to
their packs. These extra files could be used to provide pre-built configuration
files, or to generate Nomad dependency configurations, like ACL policies and
Volume configurations for operators to load into their clusters before deploying
the pack to their cluster.

* **Vendoring Dependencies** - With `nomad-pack deps vendor`, you can
automatically download all the dependencies listed in the `metadata.hcl` file
into a `deps/` subdirectory.

BUG FIXES:
* cli: `generate registry` command creates registry in properly named folder [[GH-445](https://github.com/hashicorp/nomad-pack/pull/445)]
* cli: `generate pack` validates name argument [[GH-460](https://github.com/hashicorp/nomad-pack/pull/460)]

IMPROVEMENTS:

* cache: Change the way registries are stored and versioned in the cache [[GH-356](https://github.com/hashicorp/nomad-pack/pull/356)]
* cli: Add `generate var-file` command [[GH-333](https://github.com/hashicorp/nomad-pack/pull/333)]
* cli: `registry list` command now shows git refs to repositories present in the cache [[GH-318](https://github.com/hashicorp/nomad-pack/pull/318)]
* cli: `registry list` command now shows only registries, and a new command `list` shows packs [[GH-337](https://github.com/hashicorp/nomad-pack/pull/337)], [[GH-373](https://github.com/hashicorp/nomad-pack/pull/373)]
* cli: `deps vendor` command [[GH-367](https://github.com/hashicorp/nomad-pack/pull/367)]
* cli: `deps vendor` command now allows dependencies to be pinned [[GH-447](https://github.com/hashicorp/nomad-pack/pull/447)]
* cli: `generate pack` command now supports `--overwrite` flag [[GH-380](https://github.com/hashicorp/nomad-pack/pull/380)]
* cli: `registry add` command now uses shallow cloning [[GH-444](https://github.com/hashicorp/nomad-pack/pull/444)]
* deps: Update the Nomad OpenAPI dependency; require Go 1.18 as a build dependency [[GH-288](https://github.com/hashicorp/nomad-pack/pull/288)]
* pack: Author field no longer supported in pack metadata [[GH-317](https://github.com/hashicorp/nomad-pack/pull/317)]
* pack: URL field no longer supported in pack metadata [[GH-343](https://github.com/hashicorp/nomad-pack/pull/343)]
* runner: Submit the job spec to Nomad while running pack [[GH-375](https://github.com/hashicorp/nomad-pack/pull/375)]
* template: Render templates other than Nomad job specifications inside `templates/` [[GH-303](https://github.com/hashicorp/nomad-pack/pull/303)]
* template: Automatically format templates before outputting [[GH-311](https://github.com/hashicorp/nomad-pack/pull/311)]
* template: Skip templates that would render to just whitespace [[GH-313](https://github.com/hashicorp/nomad-pack/pull/313)]
* template: Extract namespace and region from the templates before submitting them to the client [[GH-366](https://github.com/hashicorp/nomad-pack/pull/366)]
* vars: Add flag to ignore variables provided in the given var-files unused by the pack [[GH-315](https://github.com/hashicorp/nomad-pack/pull/315)]

## 0.0.1-techpreview.3 (July 21, 2022)

FEATURES:

* **Generate Sample Pack or Registry** - Using the `nomad-pack generate` command,
  you can get started writing your own pack or building your own pack registry
  using built-in starting artifacts.

BUG FIXES:

* cli: Add missing --name flag for status command [[GH-212](https://github.com/hashicorp/nomad-pack/pull/212)]
* cli: Remove duplicate `this` in some command outputs [[GH-251](https://github.com/hashicorp/nomad-pack/pull/251)]
* cli: Use Pack metadata `Name` in error context once known [[GH-217](https://github.com/hashicorp/nomad-pack/pull/217)]
* cli: Fixed a panic in the `info` command when outputting a variable with a nil type [[GH-254](https://github.com/hashicorp/nomad-pack/pull/254)]
* cli: Fixed a bug that prevented the use of map of maps variables [[GH-272](https://github.com/hashicorp/nomad-pack/pull/272)]
* runner: Update runner to properly handle dependencies [[GH-229](https://github.com/hashicorp/nomad-pack/pull/229)]

IMPROVEMENTS:

* cli: Add flags to configure Nomad API client [[GH-213](https://github.com/hashicorp/nomad-pack/pull/213)]
* template: Add support for custom Spew configurations. [[GH-220](https://github.com/hashicorp/nomad-pack/pull/220)]
* template: Create a `my` alias for the current pack [[GH-221](https://github.com/hashicorp/nomad-pack/pull/221)]
* cli: Add flags to override exit codes on `plan` command [[GH-236](https://github.com/hashicorp/nomad-pack/pull/236)]
* cli: Add environment variables to configure Nomad API client [[GH-230](https://github.com/hashicorp/nomad-pack/pull/230)]
* deps: Update the Nomad OpenAPI dependency [[GH-270](https://github.com/hashicorp/nomad-pack/pull/271)]

## 0.0.1-techpreview2 (February 07, 2022)

FEATURES:

* **Run Pack from Folder** - Nomad Pack can run and render packs stored in the current folder. For example, if the current folder
  contains a pack named `simple-service`, you can run it using `nomad-pack run ./simple-service`.

* **Set Nomad Pack variables using environment variables** - Pack reads the
  environment for variables prefixed with `NOMAD_PACK_VAR_` and supplies them to
  the running pack.

BUG FIXES:

* template: Handle HEREDOC delimiter immediately before EOF [[GH-191](https://github.com/hashicorp/nomad-pack/pull/191)]
* cli: display API client errors in CLI output [[GH-183](https://github.com/hashicorp/nomad-pack/pull/183)]
* cli: add flags to `info` command help output [[GH-200](https://github.com/hashicorp/nomad-pack/pull/200)]
* cli: fix panic from bad registry metadata [[GH-202](https://github.com/hashicorp/nomad-pack/pull/202)]

IMPROVEMENTS:

* cache: Moved the pack registry cache to the platform-specific user cache directory [[GH-172](https://github.com/hashicorp/nomad-pack/pull/172)]
* cli: Don't build pack registry cache during the `version` command [[GH-128](https://github.com/hashicorp/nomad-pack/pull/128)]
* cli: Support Nomad ACLs and mTLS configuration [[GH-177](https://github.com/hashicorp/nomad-pack/pull/177), [GH-205](https://github.com/hashicorp/nomad-pack/pull/205)]
* cli/plan: Run template canonicalization before planning to fix diffs [[GH-181](https://github.com/hashicorp/nomad-pack/pull/181)]
* dependencies: Removed direct import of Nomad code base [[GH-157](https://github.com/hashicorp/nomad-pack/pull/157)]
* template: Added `toStringList` function [[GH-136](https://github.com/hashicorp/nomad-pack/pull/136)]
* template: Update Sprig library to v3 [[GH-197](https://github.com/hashicorp/nomad-pack/pull/197)]


## 0.0.1-techpreview1 (October 19, 2021)

Initial release.
