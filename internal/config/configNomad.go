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
