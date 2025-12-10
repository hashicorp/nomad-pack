// Copyright IBM Corp. 2021, 2025
// SPDX-License-Identifier: MPL-2.0

package testhelper

import (
	"fmt"
	"os"
	"path"
	"testing"

	"github.com/hashicorp/go-multierror"
	"github.com/hashicorp/nomad/api"
	"github.com/hashicorp/nomad/command/agent"
	"github.com/hashicorp/nomad/testutil"
	"github.com/shoenig/test/must"
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
	testutil.WaitForLeader(t, srv.RPC)

	// Store the bootstrap ACL secret in the TestServer's client meta map
	// so that it is easily accessible from tests.
	srv.Config.Client.Meta = make(map[string]string, 1)
	srv.Config.Client.Meta["token"] = srv.RootToken.SecretID

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
	testutil.WaitForLeader(t, s.RPC)
	f(s)
}

// HTTPTestMultiRegionClusterParallel generates a multi-region two node cluster
// and automatically enables test parallelism. This will panic on test which use
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

	testutil.WaitForLeader(t, s1.RPC)
	testutil.WaitForLeader(t, s2.RPC)

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
	count, err := first.APIClient().Agent().Join(addrs...)
	must.NoError(first.T, err)
	must.Eq(first.T, len(nodes)-1, count)
}

func NewTestClient(testAgent *agent.TestAgent) (*api.Client, error) {
	clientConfig := api.DefaultConfig()
	clientConfig.Address = testAgent.HTTPAddr()
	if token := testAgent.Config.Client.Meta["token"]; token != "" {
		clientConfig.SecretID = token
	}

	return api.NewClient(clientConfig)
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
		c.ACL.Enabled = true
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

func MakeTestNamespaces(t *testing.T, c *api.Client) {
	testNs := &api.Namespace{}
	namespaces := map[string]string{
		"job":  "job namespace",
		"flag": "flag namespace",
		"env":  "env namespace",
	}
	for nsName, nsDesc := range namespaces {
		testNs.Name = nsName
		testNs.Description = nsDesc
		_, err := c.Namespaces().Register(testNs, &api.WriteOptions{})
		must.NoError(t, err)
	}

}

func NomadRun(s *agent.TestAgent, path string) error {
	c, err := NewTestClient(s)
	if err != nil {
		return err
	}

	// Get Job
	jB, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	// Parse into JSON Jobspec
	j, err := c.Jobs().ParseHCLOpts(&api.JobsParseRequest{
		JobHCL:       string(jB),
		Canonicalize: true,
	})
	if err != nil {
		return err
	}

	// Run parsed job
	resp, _, err := c.Jobs().Register(j, &api.WriteOptions{})
	if err != nil {
		return fmt.Errorf("unable to register nomad job: %v", err)
	}
	s.T.Log(FormatRegistrationResponse(resp))
	return nil
}

func NomadJobStatus(s *agent.TestAgent, jobID string) (*api.Job, error) {
	c, err := NewTestClient(s)
	if err != nil {
		return nil, err
	}
	resp, _, err := c.Jobs().Info(jobID, &api.QueryOptions{})
	if err != nil {
		return nil, err
	}
	return resp, nil
}

func NomadStop(s *agent.TestAgent, jobID string) error {
	c, err := NewTestClient(s)
	if err != nil {
		return err
	}

	resp, _, err := c.Jobs().Deregister(jobID, false, &api.WriteOptions{})
	if err != nil {
		return err
	}
	s.T.Log(FormatStopResponse(resp))
	return nil
}

func NomadPurge(s *agent.TestAgent, jobID string) error {
	c, err := NewTestClient(s)
	if err != nil {
		return err
	}

	resp, _, err := c.Jobs().Deregister(jobID, true, &api.WriteOptions{})
	if err != nil {
		return err
	}
	s.T.Log(FormatStopResponse(resp))
	return nil
}

func NomadCleanup(s *agent.TestAgent) (error, error) {
	c, err := NewTestClient(s)
	if err != nil {
		return err, nil
	}
	c.SetNamespace("*")

	jR, _, err := c.Jobs().List(&api.QueryOptions{})
	if err != nil {
		return err, nil
	}

	var mErr *multierror.Error
	for _, job := range jR {
		err := NomadPurge(s, job.ID)
		mErr = multierror.Append(mErr, err)
	}
	return nil, mErr.ErrorOrNil()
}

func FormatRegistrationResponse(resp *api.JobRegisterResponse) string {
	format := `register response. eval_id: %q warnings: %q`
	return fmt.Sprintf(format, resp.EvalID, resp.Warnings)
}

func FormatStopResponse(resp string) string {
	format := `deregister response. eval_id: %q`
	return fmt.Sprintf(format, resp)
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
