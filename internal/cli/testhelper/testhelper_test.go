// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package testhelper

import (
	"fmt"
	"testing"

	"github.com/hashicorp/nomad/api"
	"github.com/hashicorp/nomad/command/agent"
	"github.com/shoenig/test/must"
)

func Test_HTTPTest(t *testing.T) {
	HTTPTest(t, WithDefaultConfig(), func(s *agent.TestAgent) {
		client, err := NewTestClient(s)
		must.NoError(t, err)

		result, err := client.Status().Leader()
		t.Logf("result: %q", result)
		must.NoError(t, err)
		must.NotNil(t, result)
	})
}

func TestTestHelper_HTTPTestParallel(t *testing.T) {
	HTTPTestParallel(t, WithDefaultConfig(), func(s *agent.TestAgent) {
		client, err := NewTestClient(s)
		must.NoError(t, err)

		result, err := client.Status().Leader()
		t.Logf("result: %q", result)
		must.NoError(t, err)
		must.NotNil(t, result)
	})
}

func TestTestHelper_HTTPTestWithACL(t *testing.T) {
	HTTPTestWithACL(t, WithDefaultConfig(), func(s *agent.TestAgent) {
		client, err := NewTestClient(s)
		must.NoError(t, err)

		client.SetSecretID(s.Config.Client.Meta["token"])

		result, err := client.Status().Leader()
		t.Logf("result: %q", result)
		must.NoError(t, err)
		must.NotNil(t, result)
	})
}

func TestTestHelper_HTTPTestWithACLParallel(t *testing.T) {
	HTTPTestWithACLParallel(t, WithDefaultConfig(), func(s *agent.TestAgent) {
		client, err := NewTestClient(s)
		must.NoError(t, err)

		client.SetSecretID(s.Config.Client.Meta["token"])

		result, err := client.Status().Leader()
		t.Logf("result: %q", result)
		must.NoError(t, err)
		must.NotNil(t, result)
	})
}

func TestTestHelper_HTTPTestParallel_TLSEnabled(t *testing.T) {
	HTTPTestParallel(t,
		WithAgentConfig(
			TLSEnabled(),
			LogLevel(testLogLevel),
		),
		func(s *agent.TestAgent) {
			clientConfig := api.DefaultConfig()
			clientConfig.Address = s.HTTPAddr()
			clientConfig.TLSConfig.ClientCert = mTLSFixturePath("client", "certfile")
			clientConfig.TLSConfig.CACert = mTLSFixturePath("client", "cafile")
			clientConfig.TLSConfig.ClientKey = mTLSFixturePath("client", "keyfile")

			client, err := api.NewClient(api.DefaultConfig())
			must.NoError(t, err)

			result, err := client.Status().Leader()
			t.Logf("result: %q", result)
			must.NoError(t, err)
			must.NotNil(t, result)
		},
	)
}

func TestTestHelper_MultiRegionCluster(t *testing.T) {
	HTTPTestMultiRegionCluster(t,
		WithAgentConfig(LogLevel(testLogLevel), Region("rA")),
		WithAgentConfig(LogLevel(testLogLevel), Region("rB")),
		func(s1, s2 *agent.TestAgent) {
			c1, err := NewTestClient(s1)
			must.NoError(t, err)
			r, err := c1.Regions().List()
			must.NoError(t, err)

			must.SliceContainsAll(t, []string{"rA", "rB"}, r)
		},
	)
}

func TestTestHelper_MultiRegionClusterParallel(t *testing.T) {
	HTTPTestMultiRegionClusterParallel(t,
		WithAgentConfig(LogLevel(testLogLevel), Region("rA")),
		WithAgentConfig(LogLevel(testLogLevel), Region("rB")),
		func(s1, s2 *agent.TestAgent) {
			c1, err := NewTestClient(s1)
			must.NoError(t, err)
			r, err := c1.Regions().List()
			must.NoError(t, err)

			must.SliceContainsAll(t, []string{"rA", "rB"}, r)
		},
	)
}

func TestTestHelper_NomadRun(t *testing.T) {
	HTTPTestParallel(t, WithDefaultConfig(), func(srv *agent.TestAgent) {
		NomadRun(srv, getTestNomadJobPath("simple_raw_exec"))
		c, err := NewTestClient(srv)
		must.NoError(t, err)
		jR, _, err := c.Jobs().List(nil)
		must.NoError(t, err)
		must.Eq(t, 1, len(jR))
	})
}

func TestTestHelper_NomadJobStatus(t *testing.T) {
	HTTPTestParallel(t, WithDefaultConfig(), func(srv *agent.TestAgent) {
		NomadRun(srv, getTestNomadJobPath("simple_raw_exec"))
		job, err := NomadJobStatus(srv, "simple_raw_exec")
		must.NoError(t, err)
		must.Eq(t, "simple_raw_exec", *job.Name)
	})
}

func TestTestHelper_NomadStop(t *testing.T) {
	HTTPTestParallel(t, WithDefaultConfig(), func(srv *agent.TestAgent) {
		c, err := NewTestClient(srv)
		must.NoError(t, err)

		NomadRun(srv, getTestNomadJobPath("simple_raw_exec"))
		jR, _, err := c.Jobs().List(nil)
		must.NoError(t, err)
		must.Eq(t, 1, len(jR))

		NomadStop(srv, "simple_raw_exec")
		job, _, err := c.Jobs().Info("simple_raw_exec", nil)
		must.NoError(t, err)
		must.True(t, *job.Stop)
	})
}

func TestTestHelper_NomadPurge(t *testing.T) {
	HTTPTestParallel(t, WithDefaultConfig(), func(srv *agent.TestAgent) {
		c, err := NewTestClient(srv)
		must.NoError(t, err)

		NomadRun(srv, getTestNomadJobPath("simple_raw_exec"))
		jR, _, err := c.Jobs().List(nil)
		must.NoError(t, err)
		must.Eq(t, 1, len(jR))

		NomadPurge(srv, "simple_raw_exec")
		jR, _, err = c.Jobs().List(nil)
		must.NoError(t, err)
		must.Eq(t, 0, len(jR))
	})
}

func TestTestHelper_NomadCleanup(t *testing.T) {
	HTTPTestParallel(t, WithDefaultConfig(), func(srv *agent.TestAgent) {
		c, err := NewTestClient(srv)
		must.NoError(t, err)
		MakeTestNamespaces(t, c)

		err = NomadRun(srv, getTestNomadJobPath("simple_raw_exec"))
		must.NoError(t, err)

		qoAllNS := &api.QueryOptions{Namespace: "*"}

		jR, qm, err := c.Jobs().List(qoAllNS)
		fmt.Printf("\n\n-- qm: %+#v\n\n", qm)
		must.NoError(t, err)
		must.Eq(t, 2, len(jR))

		err, warn := NomadCleanup(srv)
		must.NoError(t, err)

		if warn != nil {
			t.Log("warnings cleaning cluster", "warn", warn.Error())
		}

		jR, _, err = c.Jobs().List(qoAllNS)
		must.NoError(t, err)
		must.Eq(t, 0, len(jR))
	})
}
