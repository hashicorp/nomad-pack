# Copyright (c) HashiCorp, Inc.
# SPDX-License-Identifier: MPL-2.0

schema = 1
artifacts {
  zip = [
    "nomad-pack_${version}_darwin_amd64.zip",
    "nomad-pack_${version}_darwin_arm64.zip",
    "nomad-pack_${version}_freebsd_amd64.zip",
    "nomad-pack_${version}_freebsd_arm64.zip",
    "nomad-pack_${version}_linux_amd64.zip",
    "nomad-pack_${version}_linux_arm64.zip",
    "nomad-pack_${version}_windows_amd64.zip",
    "nomad-pack_${version}_windows_arm64.zip",
  ]
  rpm = [
    "nomad-pack-${version_linux}-1.aarch64.rpm",
    "nomad-pack-${version_linux}-1.x86_64.rpm",
  ]
  deb = [
    "nomad-pack_${version_linux}-1_amd64.deb",
    "nomad-pack_${version_linux}-1_arm64.deb",
  ]
  container = [
    "nomad-pack_release_linux_amd64_${version}_${commit_sha}.docker.dev.tar",
    "nomad-pack_release_linux_amd64_${version}_${commit_sha}.docker.tar",
    "nomad-pack_release_linux_arm64_${version}_${commit_sha}.docker.dev.tar",
    "nomad-pack_release_linux_arm64_${version}_${commit_sha}.docker.tar",
  ]
}
