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

const (
	globalRegion     = "global"
	defaultNamespace = "default"
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
