package cli

import (
	"fmt"
	"path"
	"testing"

	v1 "github.com/hashicorp/nomad-openapi/v1"
	"github.com/hashicorp/nomad/command/agent"
	"github.com/hashicorp/nomad/testutil"
	"github.com/stretchr/testify/require"
)

// makeHTTPServer returns a test server whose logs will be written to
// the passed writer. If the writer is nil, the logs are written to stderr.
func makeHTTPServer(t testing.TB, cb func(c *agent.Config)) *agent.TestAgent {
	return agent.NewTestAgent(t, t.Name(), cb)
}

// makeHTTPServer returns a test server whose logs will be written to
// the passed writer. If the writer is nil, the logs are written to stderr.
func makeACLEnabledHTTPServer(t testing.TB, cb func(c *agent.Config)) *agent.TestAgent {
	aclCB := func(c *agent.Config) {
		cb(c)
		ACLEnabled()(c)
	}
	srv := agent.NewTestAgent(t, t.Name(), aclCB)
	testutil.WaitForLeader(t, srv.Agent.RPC)
	c, err := NewTestClient(srv)
	require.NoError(t, err)
	tResp, _, err := c.ACL().Bootstrap(c.WriteOpts().Ctx())
	require.NoError(t, err)

	// Store the bootstrap ACL secret in the TestServer's client meta map
	// so that it is easily accessible from tests.
	srv.Config.Client.Meta = make(map[string]string, 1)
	srv.Config.Client.Meta["token"] = tResp.GetSecretID()

	return srv
}

// httpTestWithACLParallel generates a ACL enabled single node cluster and
// automatically enables test parallelism.
func httpTestWithACLParallel(t *testing.T, cb func(c *agent.Config), f func(srv *agent.TestAgent)) {
	httpTestWithACLs(t, cb, f, true)
}

// httpTestWithACL generates an ACL enabled single node cluster, without
// automatically enabling test parallelism. This is necessary for any test that
// uses the environment.
func httpTestWithACL(t *testing.T, cb func(c *agent.Config), f func(srv *agent.TestAgent)) {
	httpTestWithACLs(t, cb, f, false)
}

// Use httpTestWithACLParallel or httpTestWithACL instead.
func httpTestWithACLs(t *testing.T, cb func(c *agent.Config), f func(srv *agent.TestAgent), parallel bool) {
	if parallel {
		t.Parallel()
	}
	s := makeACLEnabledHTTPServer(t, cb)
	defer s.Shutdown()
	// Leadership is waited for in makeACLEnabledHTTPServer
	f(s)
}

// httpTest generates a non-ACL enabled single node cluster, without automatically
// enabling test parallelism. This is necessary for any test that uses the
// environment.
func httpTest(t *testing.T, cb func(c *agent.Config), f func(srv *agent.TestAgent)) {
	httpTests(t, cb, f, false)
}

// httpTest generates a non-ACL enabled single node cluster and automatically
// enables test parallelism.
func httpTestParallel(t *testing.T, cb func(c *agent.Config), f func(srv *agent.TestAgent)) {
	// Since any test that uses httpTest has a distinct TestAgent, we can
	// automatically parallelize these tests
	httpTests(t, cb, f, true)
}

// Use httpTestParallel or httpTest instead.
func httpTests(t *testing.T, cb func(c *agent.Config), f func(srv *agent.TestAgent), parallel bool) {
	if parallel {
		t.Parallel()
	}
	s := makeHTTPServer(t, cb)
	defer s.Shutdown()
	testutil.WaitForLeader(t, s.Agent.RPC)
	f(s)
}

// httpTestMultiRegionCluster generates a multi-region two node cluster and
// automatically enables test parallelism. This will panic on test which use
// the environment. For those, use httpTestMultiRegionCluster instead.
func httpTestMultiRegionClusterParallel(t *testing.T, cb1, cb2 func(c *agent.Config), f func(s1 *agent.TestAgent, s2 *agent.TestAgent)) {
	httpTestMultiRegionClusters(t, cb1, cb2, f, true)
}

// httpTestMultiRegionCluster generates a multi-region two node cluster, without
// automatically enabling test parallelism. This is necessary for any test that
// uses the environment.
func httpTestMultiRegionCluster(t *testing.T, cb1, cb2 func(c *agent.Config), f func(s1 *agent.TestAgent, s2 *agent.TestAgent)) {
	httpTestMultiRegionClusters(t, cb1, cb2, f, false)
}

// Use httpTestMultiRegionClusterParallel or httpTestMultiRegionCluster instead.
func httpTestMultiRegionClusters(t *testing.T, cb1, cb2 func(c *agent.Config), f func(s1 *agent.TestAgent, s2 *agent.TestAgent), parallel bool) {
	if parallel {
		t.Parallel()
	}
	s1, s2 := makeMultiRegionCluster(t, cb1, cb2)
	defer func(s1, s2 *agent.TestAgent) {
		s1.Shutdown()
		s2.Shutdown()
	}(s1, s2)

	testutil.WaitForLeader(t, s1.Agent.RPC)
	testutil.WaitForLeader(t, s2.Agent.RPC)

	f(s1, s2)
}

func makeMultiRegionCluster(t testing.TB, cb1, cb2 func(c *agent.Config)) (s1, s2 *agent.TestAgent) {
	s1 = agent.NewTestAgent(t, fmt.Sprintf("%s-%s", t.Name(), "-s1"), cb1)
	s2 = agent.NewTestAgent(t, fmt.Sprintf("%s-%s", t.Name(), "-s2"), cb2)

	join(s1, s2)

	return s1, s2
}

// join takes the first node and joins all of the remaining nodes provided to
// it. When no nodes or 1 node is passed in, this function returns immediately.
func join(nodes ...*agent.TestAgent) {
	if len(nodes) == 0 || len(nodes) == 1 {
		return
	}
	first := nodes[0]
	var addrs = make([]string, len(nodes)-1)
	for i, node := range nodes[1:] {
		member := node.Agent.Server().LocalMember()
		addrs[i] = fmt.Sprintf("%s:%d", member.Addr, member.Port)
	}
	count, err := first.Client().Agent().Join(addrs...)
	require.NoError(first.T, err)
	require.Equal(first.T, len(nodes)-1, count)
}

func NewTestClient(testAgent *agent.TestAgent, opts ...v1.ClientOption) (*v1.Client, error) {
	maybeTokenFn := func() func(*v1.Client) { return func(*v1.Client) {} }

	if token := testAgent.Config.Client.Meta["token"]; token != "" {
		testAgent.T.Logf("building test client with token %q", token)
		maybeTokenFn = func() func(*v1.Client) {
			fmt.Printf("applying token %q\n", token)
			return v1.WithToken(token)
		}
	}

	// Push the address and possible token to the end of the options list since
	// they are applied in order to the client config.
	cOpts := append(opts, v1.WithAddress(testAgent.HTTPAddr()), maybeTokenFn())
	c, err := v1.NewClient(cOpts...)

	if err != nil {
		return nil, err
	}

	testAgent.T.Log(FormatAPIClientConfig(c))
	return c, nil
}

func Test_TestAgent_Simple(t *testing.T) {
	httpTestParallel(t, WithDefaultConfig(), func(s *agent.TestAgent) {
		client, err := NewTestClient(s)
		require.NoError(t, err)

		q := &v1.QueryOpts{
			Region:    v1.GlobalRegion,
			Namespace: v1.DefaultNamespace,
		}

		result, err := client.Status().Leader(q.Ctx())
		t.Logf("result: %q", *result)
		require.NoError(t, err)
		require.NotNil(t, result)
	})
}

func Test_TestAgent_TLSEnabled(t *testing.T) {
	httpTestParallel(t,
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

			require.NoError(t, err)

			q := &v1.QueryOpts{
				Region:    v1.GlobalRegion,
				Namespace: v1.DefaultNamespace,
			}
			result, err := client.Status().Leader(q.Ctx())
			t.Logf("result: %q", *result)
			require.NoError(t, err)
			require.NotNil(t, result)
		},
	)
}

func Test_MultiRegionCluster(t *testing.T) {
	httpTestMultiRegionCluster(t,
		WithAgentConfig(LogLevel(testLogLevel), Region("rA")),
		WithAgentConfig(LogLevel(testLogLevel), Region("rB")),
		func(s1, s2 *agent.TestAgent) {
			c1, err := NewTestClient(s1)
			require.NoError(t, err)
			r, err := c1.Regions().GetRegions(c1.QueryOpts().Ctx())
			require.NoError(t, err)

			require.ElementsMatch(t, []string{"rA", "rB"}, *r)
		},
	)
}

// TestAgent configuration helpers

// AgentOption is a functional option used as an argument to `WithAgentConfig`
type AgentOption func(*agent.Config)

// WithDefaultConfig provides an agent.Config callback that generally applies
// across all tests
func WithDefaultConfig() func(c *agent.Config) {
	return WithAgentConfig(LogLevel(testLogLevel))
}

// WithAgentConfig creates a callback function that applies all of the provided
// AgentOptions to the agent.Config.
func WithAgentConfig(opts ...AgentOption) func(*agent.Config) {
	return func(c *agent.Config) {
		for _, opt := range opts {
			opt(c)
		}
	}
}

// Region is an AgentOption used to control the log level of the TestServer
func Region(name string) AgentOption {
	return func(c *agent.Config) {
		c.Region = name
	}
}

// LogLevel is an AgentOption used to control the log level of the TestServer
func LogLevel(level string) AgentOption {
	return func(c *agent.Config) {
		c.LogLevel = level
	}
}

// ACLEnabled is an AgentOption used to configure the TestServer with ACLs
// enabled. Once started, this agent will need to have the ACLs bootstrapped.
func ACLEnabled() AgentOption {
	return func(c *agent.Config) {
		c.NomadConfig.ACLEnabled = true
	}
}

// TLSEnabled is an AgentOption used to configure the TestServer with a set of
// test mTLS certificates.
func TLSEnabled() AgentOption {
	return func(c *agent.Config) {
		tC := c.TLSConfig
		tC.VerifyHTTPSClient = true
		tC.EnableHTTP = true
		tC.CAFile = mTLSFixturePath("server", "cafile")
		tC.CertFile = mTLSFixturePath("server", "certfile")
		tC.KeyFile = mTLSFixturePath("server", "keyfile")
	}
}

func mTLSFixturePath(nodeType, pemType string) string {
	var filename string
	switch pemType {
	case "cafile":
		filename = "nomad-agent-ca.pem"
	case "certfile":
		filename = fmt.Sprintf("global-%s-nomad-0.pem", nodeType)
	case "keyfile":
		filename = fmt.Sprintf("global-%s-nomad-0-key.pem", nodeType)
	}

	return path.Join(testFixturePath(), "mtls", filename)
}
