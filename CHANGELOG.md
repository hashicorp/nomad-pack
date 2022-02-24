## UNRELEASED

BUG FIXES:
* cli: Add missing --name flag for status command [[GH-212](https://github.com/hashicorp/nomad-pack/pull/212)]

IMPROVEMENTS:
* cli: Add flags to configure Nomad API client [[GH-213](https://github.com/hashicorp/nomad-pack/pull/213)]

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
