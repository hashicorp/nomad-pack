
## Unreleased

FEATURE:
* **Set Nomad Pack variables using environment variables** - Pack reads the
  environment for variables prefixed with `NOMAD_PACK_VAR_` and supplies them to
  the running pack.

BUG FIXES:
* template: Handle HEREDOC delimiter immediately before EOF [GH-191](https://github.com/hashicorp/nomad-pack/pull/191)

IMPROVEMENTS:

* cache: Moved the pack registry cache to the platform-specific user cache directory [GH-172](https://github.com/hashicorp/nomad-pack/pull/172)
* cli: Don't build pack registry cache during the `version` command [GH-128](https://github.com/hashicorp/nomad-pack/pull/128)
* cli: Support Nomad ACLs and mTLS configuration [GH-177](https://github.com/hashicorp/nomad-pack/pull/177)
* cli/plan: Run template canonicalization before planning to fix diffs [GH-181](https://github.com/hashicorp/nomad-pack/pull/181)
* dependencies: Removed direct import of Nomad code base [GH-157](https://github.com/hashicorp/nomad-pack/pull/157)
* template: Added `toStringList` function [GH-136](https://github.com/hashicorp/nomad-pack/pull/136)

## 0.0.1-techpreview1 (October 19, 2021)

Initial release.
