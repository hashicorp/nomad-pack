#!/usr/bin/env sh

set -e

if [ "$1" = 'nomad-pack' ]; then
  shift
fi

exec nomad-pack "$@"
