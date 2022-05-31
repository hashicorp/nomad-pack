#!/usr/bin/env bash

version_file=$1
version_metadata_file=$2
version=$(awk '$1 == "Version" && $2 == "=" { gsub(/"/, "", $3); print $3 }' <"${version_file}")
prerelease=$(awk '$1 == "Prerelease" && $2 == "=" { gsub(/"/, "", $3); print $3 }' <"${version_file}")
metadata=$(awk '$1 == "Metadata" && $2 == "=" { gsub(/"/, "", $3); print $3 }' <"${version_metadata_file}")

if [ -n "$metadata" ] && [ -n "$prerelease" ]; then
    echo "${version}-${prerelease}+${metadata}"
elif [ -n "$metadata" ]; then
    echo "${version}+${metadata}"
elif [ -n "$prerelease" ]; then
    echo "${version}-${prerelease}"
else
    echo "${version}"
fi
