#!/usr/bin/env sh
# Copyright IBM Corp. 2021, 2025
# SPDX-License-Identifier: MPL-2.0


set -e

if [ "$1" = 'nomad-pack' ]; then
  shift
fi

exec nomad-pack "$@"
