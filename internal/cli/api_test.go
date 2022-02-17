package cli

import (
	"fmt"
	"path"
	"testing"
	"time"

	v1 "github.com/hashicorp/nomad-openapi/v1"
	"github.com/hashicorp/nomad/command/agent"
	"github.com/hashicorp/nomad/testutil"
	"github.com/stretchr/testify/require"
)

const (
	globalRegion     = "global"
	defaultNamespace = "default"
)

var (
	queryOpts = v1.DefaultQueryOpts().
			WithAllowStale(true).
			WithWaitIndex(1000).
			WithWaitTime(100 * time.Second)

	writeOpts = v1.DefaultWriteOpts()
)

// makeHTTPServer returns a test server whose logs will be written to
// the passed writer. If the writer is nil, the logs are written to stderr.
func makeHTTPServer(t testing.TB, cb func(c *agent.Config)) *agent.TestAgent {
	return agent.NewTestAgent(t, t.Name(), cb)
}

func httpTest(t testing.TB, cb func(c *agent.Config), f func(srv *agent.TestAgent)) {
	s := makeHTTPServer(t, cb)
	defer s.Shutdown()
	testutil.WaitForLeader(t, s.Agent.RPC)
	//	t.Setenv("NOMAD_ADDR", fmt.Sprintf("http://%s:%d", s.Config.BindAddr, s.Config.Ports.HTTP))
	f(s)
}

func NewTestClient(testAgent *agent.TestAgent) (*v1.Client, error) {
	c, err := v1.NewClient(v1.WithAddress(testAgent.HTTPAddr()))
	if err != nil {
		return nil, err
	}

	return c, nil
}

func TestSetQueryOptions(t *testing.T) {
	httpTest(t, nil, func(s *agent.TestAgent) {

		ctx := queryOpts.Ctx()
		qCtx := ctx.Value("QueryOpts").(*v1.QueryOpts)

		require.Equal(t, qCtx.Region, queryOpts.Region)
		require.Equal(t, qCtx.Namespace, queryOpts.Namespace)
		require.Equal(t, qCtx.AllowStale, queryOpts.AllowStale)
		require.Equal(t, qCtx.WaitIndex, queryOpts.WaitIndex)
		require.Equal(t, qCtx.WaitTime, queryOpts.WaitTime)
		require.Equal(t, qCtx.AuthToken, queryOpts.AuthToken)
		require.Equal(t, qCtx.PerPage, queryOpts.PerPage)
		require.Equal(t, qCtx.NextToken, queryOpts.NextToken)
		require.Equal(t, qCtx.Prefix, queryOpts.Prefix)
	})
}

func TestSetWriteOptions(t *testing.T) {
	httpTest(t, nil, func(s *agent.TestAgent) {
		ctx := writeOpts.Ctx()
		wCtx := ctx.Value("WriteOpts").(*v1.WriteOpts)

		require.Equal(t, wCtx.Region, writeOpts.Region)
		require.Equal(t, wCtx.Namespace, writeOpts.Namespace)
		require.Equal(t, wCtx.AuthToken, writeOpts.AuthToken)
		require.Equal(t, wCtx.IdempotencyToken, writeOpts.IdempotencyToken)
	})
}

func TestACLBootstrap(t *testing.T) {
	enableACL := func(c *agent.Config) {
		c.NomadConfig.ACLEnabled = true
	}
	httpTest(t, enableACL, func(s *agent.TestAgent) {
		client, err := NewTestClient(s)
		require.NoError(t, err)

		q := &v1.QueryOpts{
			Region:    globalRegion,
			Namespace: defaultNamespace,
		}
		token, wMeta, err := client.ACL().Bootstrap(q.Ctx())
		require.NoError(t, err)
		require.NotNil(t, wMeta)
		require.NotNil(t, token)

		// Test query without token now that bootstrapped
		_, qMeta, err := client.Jobs().GetJobs(q.Ctx())
		require.Error(t, err)
		require.Nil(t, qMeta)

		t.Log(err)

		q.WithAuthToken(*token.SecretID)

		// Test query with token now that bootstrapped
		result, qMeta, err := client.Jobs().GetJobs(q.Ctx())
		require.NoError(t, err)
		require.NotNil(t, qMeta)
		require.NotNil(t, result)
	})
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

func TestTLSEnabled(t *testing.T) {
	enableTLS := func(c *agent.Config) {
		tC := c.TLSConfig
		tC.VerifyHTTPSClient = true
		tC.EnableHTTP = true
		tC.CAFile = mTLSFixturePath("server", "cafile")
		tC.CertFile = mTLSFixturePath("server", "certfile")
		tC.KeyFile = mTLSFixturePath("server", "keyfile")
	}
	httpTest(t, enableTLS, func(s *agent.TestAgent) {
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
			Region:    globalRegion,
			Namespace: defaultNamespace,
		}
		result, err := client.Status().Leader(q.Ctx())
		t.Logf("result: %q", *result)
		require.NoError(t, err)
		require.NotNil(t, result)
	})
}

// TestServer configuration helpers

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
