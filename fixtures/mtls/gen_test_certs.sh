#!/usr/bin/env bash
# Copyright IBM Corp. 2021, 2025
# SPDX-License-Identifier: MPL-2.0


GOOS=`go env GOOS`
GOARCH=`go env GOARCH`

wget --no-verbose https://releases.hashicorp.com/consul/1.11.3/consul_1.11.3_${GOOS}_${GOARCH}.zip

unzip consul_1.11.3_${GOOS}_${GOARCH}.zip
rm -f consul_1.11.3_${GOOS}_${GOARCH}.zip

./consul tls ca create -domain="nomad"
./consul tls cert create -dc="global" -domain="nomad" -server
./consul tls cert create -dc="global" -domain="nomad" -client

rm -f consul
