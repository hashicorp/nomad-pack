// Copyright IBM Corp. 2021, 2025
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
