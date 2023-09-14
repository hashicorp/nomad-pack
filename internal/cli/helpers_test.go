package cli

import (
	"testing"

	"github.com/hashicorp/nomad/api"
	"github.com/shoenig/test/must"
)

func TestHelpers_removeBasicAuth(t *testing.T) {
	cases := []struct {
		addr         string
		expectedUser string
		expectedPass string
		expectedAddr string
	}{
		{"addr", "", "", "addr"},
		{"addr:1337", "", "", "addr:1337"},
		{"user:pass@addr", "", "", "user:pass@addr"},
		{"user:pass@addr:1337", "", "", "user:pass@addr:1337"},
		{"scheme://addr", "", "", "scheme://addr"},
		{"scheme://user:pass@addr", "user", "pass", "scheme://addr"},
		{"scheme://user:pass@addr:1337", "user", "pass", "scheme://addr:1337"},
		{"scheme://user:@addr:1337", "user", "", "scheme://addr:1337"},
		{"scheme://:pass@addr:1337", "", "pass", "scheme://addr:1337"},
		{"//user:pass@addr:1337", "user", "pass", "//addr:1337"},
		{"foo@bar", "", "", "foo@bar"},
		{"", "", "", ""},
	}

	for _, c := range cases {
		t.Run(c.addr, func(t *testing.T) {
			user, pass, addr := removeBasicAuth(c.addr)

			must.Eq(t, c.expectedUser, user)
			must.Eq(t, c.expectedPass, pass)
			must.Eq(t, c.expectedAddr, addr)
		})
	}
}

func TestHelpers_clientOptsFromEnvironment_Address(t *testing.T) {
	cases := []struct {
		addr             string
		expectedAddress  string
		expectedHttpAuth *api.HttpBasicAuth
	}{
		{
			addr:             "addr",
			expectedAddress:  "addr",
			expectedHttpAuth: nil,
		},
		{
			addr:             "scheme://user:pass@addr",
			expectedAddress:  "scheme://addr",
			expectedHttpAuth: &api.HttpBasicAuth{Username: "user", Password: "pass"},
		},
		{
			addr:             "scheme://user:@addr",
			expectedAddress:  "scheme://addr",
			expectedHttpAuth: &api.HttpBasicAuth{Username: "user", Password: ""},
		},
		{
			addr:             "scheme://:pass@addr",
			expectedAddress:  "scheme://addr",
			expectedHttpAuth: &api.HttpBasicAuth{Username: "", Password: "pass"},
		},
	}

	for _, c := range cases {
		t.Run(c.addr, func(t *testing.T) {
			t.Setenv("NOMAD_ADDR", c.addr)

			conf := api.Config{HttpAuth: nil}
			clientOptsFromEnvironment(&conf)

			must.Eq(t, c.expectedAddress, conf.Address)
			must.Eq(t, c.expectedHttpAuth, conf.HttpAuth)
		})
	}
}

func TestHelpers_clientOptsFromFlags_Address(t *testing.T) {
	cases := []struct {
		addr             string
		expectedAddress  string
		expectedHttpAuth *api.HttpBasicAuth
	}{
		{
			addr:             "addr",
			expectedAddress:  "addr",
			expectedHttpAuth: nil,
		},
		{
			addr:             "scheme://user:pass@addr",
			expectedAddress:  "scheme://addr",
			expectedHttpAuth: &api.HttpBasicAuth{Username: "user", Password: "pass"},
		},
		{
			addr:             "scheme://user:@addr",
			expectedAddress:  "scheme://addr",
			expectedHttpAuth: &api.HttpBasicAuth{Username: "user", Password: ""},
		},
		{
			addr:             "scheme://:pass@addr",
			expectedAddress:  "scheme://addr",
			expectedHttpAuth: &api.HttpBasicAuth{Username: "", Password: "pass"},
		},
	}

	for _, c := range cases {
		t.Run(c.addr, func(t *testing.T) {
			cmd := baseCommand{nomadConfig: nomadConfig{address: c.addr}}

			conf := api.Config{HttpAuth: nil}
			clientOptsFromFlags(&cmd, &conf)

			must.Eq(t, c.expectedAddress, conf.Address)
			must.Eq(t, c.expectedHttpAuth, conf.HttpAuth)
		})
	}
}
