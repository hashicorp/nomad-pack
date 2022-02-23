package cli

import (
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

func httpTest(t testing.TB, cb func(c *agent.Config), f func(srv *agent.TestAgent)) {
	s := makeHTTPServer(t, cb)
	defer s.Shutdown()
	testutil.WaitForLeader(t, s.Agent.RPC)
	f(s)
}

func NewTestClient(testAgent *agent.TestAgent) (*v1.Client, error) {
	c, err := v1.NewClient(v1.WithAddress(testAgent.HTTPAddr()))
	if err != nil {
		return nil, err
	}

	return c, nil
}

func Test_TestAgent_Simple(t *testing.T) {
	httpTest(t, nil, func(s *agent.TestAgent) {
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
