package testhelper

import (
	"fmt"
	"os"
	"path"
	"testing"

	"github.com/hashicorp/go-multierror"
	client "github.com/hashicorp/nomad-openapi/clients/go/v1"
	v1 "github.com/hashicorp/nomad-openapi/v1"
	"github.com/hashicorp/nomad/command/agent"
	"github.com/hashicorp/nomad/testutil"
	"github.com/stretchr/testify/require"
)

const testLogLevel = "ERROR"

// makeHTTPServer returns a test server whose logs will be written to
// the passed writer. If the writer is nil, the logs are written to stderr.
func makeHTTPServer(t testing.TB, cb func(c *agent.Config)) *agent.TestAgent {
	srv := agent.NewTestAgent(t, t.Name(), cb)
	t.Logf(
		"Started Nomad Test Agent. http: %v, rpc: %v, serf: %v",
		srv.Config.Ports.HTTP,
		srv.Config.Ports.RPC,
		srv.Config.Ports.Serf,
	)
	return srv
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

// HTTPTestWithACLParallel generates a ACL enabled single node cluster and
// automatically enables test parallelism.
func HTTPTestWithACLParallel(t *testing.T, cb func(c *agent.Config), f func(srv *agent.TestAgent)) {
	httpTestWithACL(t, cb, f, true)
}

// HTTPTestWithACL generates an ACL enabled single node cluster, without
// automatically enabling test parallelism. This is necessary for any test that
// uses the environment.
func HTTPTestWithACL(t *testing.T, cb func(c *agent.Config), f func(srv *agent.TestAgent)) {
	httpTestWithACL(t, cb, f, false)
}

// Use httpTestWithACLParallel or httpTestWithACL instead.
func httpTestWithACL(t *testing.T, cb func(c *agent.Config), f func(srv *agent.TestAgent), parallel bool) {
	if parallel {
		t.Parallel()
	}
	s := makeACLEnabledHTTPServer(t, cb)
	defer s.Shutdown()
	// Leadership is waited for in makeACLEnabledHTTPServer
	f(s)
}

// HTTPTest generates a non-ACL enabled single node cluster, without automatically
// enabling test parallelism. This is necessary for any test that uses the
// environment.
func HTTPTest(t *testing.T, cb func(c *agent.Config), f func(srv *agent.TestAgent)) {
	httpTest(t, cb, f, false)
}

// HTTPTestParallel generates a non-ACL enabled single node cluster and automatically
// enables test parallelism.
func HTTPTestParallel(t *testing.T, cb func(c *agent.Config), f func(srv *agent.TestAgent)) {
	// Since any test that uses httpTest has a distinct TestAgent, we can
	// automatically parallelize these tests
	httpTest(t, cb, f, true)
}

// Use httpTestParallel or httpTest instead.
func httpTest(t *testing.T, cb func(c *agent.Config), f func(srv *agent.TestAgent), parallel bool) {
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
func HTTPTestMultiRegionClusterParallel(t *testing.T, cb1, cb2 func(c *agent.Config), f func(s1 *agent.TestAgent, s2 *agent.TestAgent)) {
	httpTestMultiRegionClusters(t, cb1, cb2, f, true)
}

// HTTPTestMultiRegionCluster generates a multi-region two node cluster, without
// automatically enabling test parallelism. This is necessary for any test that
// uses the environment.
func HTTPTestMultiRegionCluster(t *testing.T, cb1, cb2 func(c *agent.Config), f func(s1 *agent.TestAgent, s2 *agent.TestAgent)) {
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
		maybeTokenFn = func() func(*v1.Client) {
			testAgent.T.Logf("building test client with token %q", token)
			return v1.WithToken(token)
		}
	}

	// Push the address and possible token to the end of the options list since
	// they are applied in order to the client config.
	opts = append(opts, v1.WithAddress(testAgent.HTTPAddr()), maybeTokenFn())
	c, err := v1.NewClient(opts...)

	if err != nil {
		return nil, err
	}

	testAgent.T.Log(FormatAPIClientConfig(c))
	return c, nil
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

func MakeTestNamespaces(t *testing.T, c *v1.Client) {
	opts := c.WriteOpts()
	{
		testNs := client.NewNamespace()
		namespaces := map[string]string{
			"job":  "job namespace",
			"flag": "flag namespace",
			"env":  "env namespace",
		}
		for nsName, nsDesc := range namespaces {
			testNs.Name = &nsName
			testNs.Description = &nsDesc
			_, err := c.Namespaces().PostNamespace(opts.Ctx(), testNs)
			require.NoError(t, err)
		}
	}
}

func NomadRun(s *agent.TestAgent, path string, opts ...v1.ClientOption) error {
	c, err := NewTestClient(s, opts...)
	if err != nil {
		return err
	}

	// Apply client options
	for _, opt := range opts {
		opt(c)
	}

	// Get Job
	jB, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	// Parse into JSON Jobspec
	j, err := c.Jobs().Parse(c.WriteOpts().Ctx(), string(jB), true, false)
	if err != nil {
		return err
	}

	// Run parsed job
	resp, _, err := c.Jobs().Register(c.WriteOpts().Ctx(), j, &v1.RegisterOpts{})
	if err != nil {
		return err
	}
	s.T.Log(FormatRegistrationResponse(resp))
	return nil
}

func NomadJobStatus(s *agent.TestAgent, jobname string, opts ...v1.ClientOption) (*client.Job, error) {
	c, err := NewTestClient(s, opts...)
	if err != nil {
		return nil, err
	}
	resp, _, err := c.Jobs().GetJob(c.QueryOpts().Ctx(), jobname)
	if err != nil {
		return nil, err
	}
	return resp, nil
}

func NomadStop(s *agent.TestAgent, jobname string, opts ...v1.ClientOption) error {
	c, err := NewTestClient(s, opts...)

	if err != nil {
		return err
	}

	resp, _, err := c.Jobs().Delete(c.WriteOpts().Ctx(), jobname, false, false)
	if err != nil {
		return err
	}
	s.T.Log(FormatStopResponse(resp))
	return nil
}

func NomadPurge(s *agent.TestAgent, jobname string, opts ...v1.ClientOption) error {
	c, err := NewTestClient(s, opts...)
	for _, opt := range opts {
		opt(c)
	}

	if err != nil {
		return err
	}

	resp, _, err := c.Jobs().Delete(c.WriteOpts().Ctx(), jobname, true, false)
	if err != nil {
		return err
	}
	s.T.Log(FormatStopResponse(resp))
	return nil
}

func NomadCleanup(s *agent.TestAgent, opts ...v1.ClientOption) (error, error) {
	c, err := NewTestClient(s)
	if err != nil {
		return err, nil
	}
	qo := c.QueryOpts()
	qo.Namespace = "*"

	jR, _, err := c.Jobs().GetJobs(qo.Ctx())
	if err != nil {
		return err, nil
	}

	var mErr *multierror.Error
	for _, job := range *jR {
		err := NomadPurge(s, job.GetName(), v1.WithDefaultNamespace(job.GetNamespace()))
		mErr = multierror.Append(mErr, err)
	}
	return nil, mErr.ErrorOrNil()
}

func FormatRegistrationResponse(resp *client.JobRegisterResponse) string {
	format := `register response. eval_id: %q warnings: %q`
	return fmt.Sprintf(format, resp.GetEvalID(), resp.GetWarnings())
}

func FormatStopResponse(resp *client.JobDeregisterResponse) string {
	format := `deregister response. eval_id: %q`
	return fmt.Sprintf(format, resp.GetEvalID())
}

// FormatAPIClientConfig can be used during a test to emit a client's current
// configuration.
func FormatAPIClientConfig(c *v1.Client) string {
	format := `current API client config. region: %q, namespace: %q, token: %q`
	opts := c.QueryOpts()
	return fmt.Sprintf(format, opts.Region, opts.Namespace, opts.AuthToken)
}

// getTestNomadJobPath returns the full path to a pack in the test
// fixtures/jobspecs folder. The `.nomad` extension will be added
// for you.
func getTestNomadJobPath(job string) string {
	return path.Join(testFixturePath(), "jobspecs", job+".nomad")
}

func testFixturePath() string {
	// This is a function to prevent a massive refactor if this ever needs to be
	// dynamically determined.
	return "../../../fixtures/"
}
