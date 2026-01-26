package cli

import (
	"net/url"
	"testing"

	"github.com/hashicorp/nomad/api"
	"github.com/shoenig/test/must"
)

func TestHelpers_removeBasicAuth(t *testing.T) {
	cases := []struct {
		addr         string
		expectedAddr string
		expectedUser *url.Userinfo
	}{
		{"addr", "addr", nil},
		{"addr:1337", "addr:1337", nil},
		{"user:pass@addr", "user:pass@addr", nil},
		{"user:pass@addr:1337", "user:pass@addr:1337", nil},
		{"scheme://addr", "scheme://addr", nil},
		{"foo@bar", "foo@bar", nil},
		{"", "", nil},
		{"scheme://user:pass@addr", "scheme://addr", url.UserPassword("user", "pass")},
		{"scheme://user:pass@addr:1337", "scheme://addr:1337", url.UserPassword("user", "pass")},
		{"scheme://user:@addr:1337", "scheme://addr:1337", url.UserPassword("user", "")},
		{"scheme://:pass@addr:1337", "scheme://addr:1337", url.UserPassword("", "pass")},
		{"//user:pass@addr:1337", "//addr:1337", url.UserPassword("user", "pass")},
	}

	for _, c := range cases {
		t.Run(c.addr, func(t *testing.T) {
			addr, userinfo := removeBasicAuth(c.addr)

			must.Eq(t, c.expectedAddr, addr)
			must.Eq(t, c.expectedUser, userinfo)
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
