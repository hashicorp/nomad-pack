# Copyright IBM Corp. 2021, 2025
# SPDX-License-Identifier: MPL-2.0

app {
  url = ""
}
pack {
  name        = "simple_docker"
  description = ""
  url         = ""
  version     = ""
}

# Optional dependency information. This block can be repeated.

# dependency "demo_dep" {
#   alias  = "demo_dep"
#   source = "git://source.git/packs/demo_dep"
# }

# Declared dependencies will be downloaded from their source
# using "nomad-pack vendor deps" and added to ./deps directory.

# Dependencies in active development can by symlinked in
# the ./deps directory

# Example dependency source values:
# - "git::https://github.com/org-name/repo-name.git//packs/demo_dep"
# - "git@github.com:org-name/repo-name.git/packs/demo_dep"
