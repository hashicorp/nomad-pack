// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package testhelper

import (
	"fmt"
	"testing"

	v1 "github.com/hashicorp/nomad-openapi/v1"
	"github.com/hashicorp/nomad/command/agent"
	"github.com/shoenig/test/must"
)

func Test_HTTPTest(t *testing.T) {
	HTTPTest(t, WithDefaultConfig(), func(s *agent.TestAgent) {
		client, err := NewTestClient(s)
		must.NoError(t, err)

		q := &v1.QueryOpts{
			Region:    v1.GlobalRegion,
			Namespace: v1.DefaultNamespace,
		}

		result, err := client.Status().Leader(q.Ctx())
		t.Logf("result: %q", *result)
		must.NoError(t, err)
		must.NotNil(t, result)
	})
}

func TestTestHelper_HTTPTestParallel(t *testing.T) {
	HTTPTestParallel(t, WithDefaultConfig(), func(s *agent.TestAgent) {
		client, err := NewTestClient(s)
		must.NoError(t, err)

		q := &v1.QueryOpts{
			Region:    v1.GlobalRegion,
			Namespace: v1.DefaultNamespace,
		}

		result, err := client.Status().Leader(q.Ctx())
		t.Logf("result: %q", *result)
		must.NoError(t, err)
		must.NotNil(t, result)
	})
}

func TestTestHelper_HTTPTestWithACL(t *testing.T) {
	HTTPTestWithACL(t, WithDefaultConfig(), func(s *agent.TestAgent) {
		client, err := NewTestClient(s)
		must.NoError(t, err)

		q := &v1.QueryOpts{
			Region:    v1.GlobalRegion,
			Namespace: v1.DefaultNamespace,
			AuthToken: s.Config.Client.Meta["token"],
		}

		result, err := client.Status().Leader(q.Ctx())
		t.Logf("result: %q", *result)
		must.NoError(t, err)
		must.NotNil(t, result)
	})
}

func TestTestHelper_HTTPTestWithACLParallel(t *testing.T) {
	HTTPTestWithACLParallel(t, WithDefaultConfig(), func(s *agent.TestAgent) {
		client, err := NewTestClient(s)
		must.NoError(t, err)

		q := &v1.QueryOpts{
			Region:    v1.GlobalRegion,
			Namespace: v1.DefaultNamespace,
			AuthToken: s.Config.Client.Meta["token"],
		}

		result, err := client.Status().Leader(q.Ctx())
		t.Logf("result: %q", *result)
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
			client, err := v1.NewClient(
				v1.WithTLSCerts(
					mTLSFixturePath("client", "cafile"),
					mTLSFixturePath("client", "certfile"),
					mTLSFixturePath("client", "keyfile"),
				),
				v1.WithAddress(s.HTTPAddr()),
			)

			must.NoError(t, err)

			q := &v1.QueryOpts{
				Region:    v1.GlobalRegion,
				Namespace: v1.DefaultNamespace,
			}
			result, err := client.Status().Leader(q.Ctx())
			t.Logf("result: %q", *result)
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
			r, err := c1.Regions().GetRegions(c1.QueryOpts().Ctx())
			must.NoError(t, err)

			must.SliceContainsAll(t, []string{"rA", "rB"}, *r)
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
			r, err := c1.Regions().GetRegions(c1.QueryOpts().Ctx())
			must.NoError(t, err)

			must.SliceContainsAll(t, []string{"rA", "rB"}, *r)
		},
	)
}

func TestTestHelper_NomadRun(t *testing.T) {
	HTTPTestParallel(t, WithDefaultConfig(), func(srv *agent.TestAgent) {
		NomadRun(srv, getTestNomadJobPath("simple_raw_exec"))
		c, err := NewTestClient(srv)
		must.NoError(t, err)
		jR, _, err := c.Jobs().GetJobs(c.QueryOpts().Ctx())
		must.NoError(t, err)
		must.Eq(t, 1, len(*jR))
	})
}

func TestTestHelper_NomadJobStatus(t *testing.T) {
	HTTPTestParallel(t, WithDefaultConfig(), func(srv *agent.TestAgent) {
		NomadRun(srv, getTestNomadJobPath("simple_raw_exec"))
		job, err := NomadJobStatus(srv, "simple_raw_exec")
		must.NoError(t, err)
		must.Eq(t, "simple_raw_exec", job.GetName())
	})
}

func TestTestHelper_NomadStop(t *testing.T) {
	HTTPTestParallel(t, WithDefaultConfig(), func(srv *agent.TestAgent) {
		c, err := NewTestClient(srv)
		must.NoError(t, err)

		NomadRun(srv, getTestNomadJobPath("simple_raw_exec"))
		jR, _, err := c.Jobs().GetJobs(c.QueryOpts().Ctx())
		must.NoError(t, err)
		must.Eq(t, 1, len(*jR))

		NomadStop(srv, "simple_raw_exec")
		job, _, err := c.Jobs().GetJob(c.QueryOpts().Ctx(), "simple_raw_exec")
		must.NoError(t, err)
		must.True(t, job.GetStop())
	})
}

func TestTestHelper_NomadPurge(t *testing.T) {
	HTTPTestParallel(t, WithDefaultConfig(), func(srv *agent.TestAgent) {
		c, err := NewTestClient(srv)
		must.NoError(t, err)

		NomadRun(srv, getTestNomadJobPath("simple_raw_exec"))
		jR, _, err := c.Jobs().GetJobs(c.QueryOpts().Ctx())
		must.NoError(t, err)
		must.Eq(t, 1, len(*jR))

		NomadPurge(srv, "simple_raw_exec")
		jR, _, err = c.Jobs().GetJobs(c.QueryOpts().Ctx())
		must.NoError(t, err)
		must.Eq(t, 0, len(*jR))
	})
}

func TestTestHelper_NomadCleanup(t *testing.T) {
	HTTPTestParallel(t, WithDefaultConfig(), func(srv *agent.TestAgent) {
		c, err := NewTestClient(srv)
		must.NoError(t, err)
		MakeTestNamespaces(t, c)

		qoAllNS := c.QueryOpts().WithNamespace("*")

		err = NomadRun(srv, getTestNomadJobPath("simple_raw_exec"))
		must.NoError(t, err)

		err = NomadRun(srv, getTestNomadJobPath("simple_raw_exec"), v1.WithDefaultNamespace("env"))
		must.NoError(t, err)

		jR, qm, err := c.Jobs().GetJobs(qoAllNS.Ctx())
		fmt.Printf("\n\n-- qm: %+#v\n\n", qm)
		must.NoError(t, err)
		must.Eq(t, 2, len(*jR))

		err, warn := NomadCleanup(srv)
		must.NoError(t, err)

		if warn != nil {
			t.Log("warnings cleaning cluster", "warn", warn.Error())
		}

		jR, _, err = c.Jobs().GetJobs(qoAllNS.Ctx())
		must.NoError(t, err)
		must.Eq(t, 0, len(*jR))
	})
}
