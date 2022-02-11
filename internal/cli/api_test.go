package cli

import (
	"fmt"
	"os"
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
	f(s)
}

func NewTestClient(testAgent *agent.TestAgent) (*v1.Client, error) {
	os.Setenv("NOMAD_ADDR", fmt.Sprintf("http://%s:%d", testAgent.Config.BindAddr, testAgent.Config.Ports.HTTP))
	defer os.Setenv("NOMAD_ADDR", "http://127.0.0.1:4646")

	return v1.NewClient()
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
		// if os.Getenv("NOMAD_TOKEN") == "" {
		// 	t.Skip()
		// }

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
