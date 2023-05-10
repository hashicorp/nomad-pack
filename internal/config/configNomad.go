// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package config

type NomadConfig struct {
	Address       string
	Namespace     string
	Region        string
	Token         string
	TLSSkipVerify bool
	TLSServerName string
	CACert        string
	ClientCert    string
	ClientKey     string
}
