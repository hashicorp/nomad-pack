// Copyright IBM Corp. 2023, 2026
// SPDX-License-Identifier: MPL-2.0

package cli

import (
	"bytes"
	"context"
	"strings"
	"testing"
	"time"

	"github.com/hashicorp/nomad-pack/internal/testui"
	"github.com/hashicorp/nomad/api"
	"github.com/shoenig/test/must"
)

func TestHasAutoRevert(t *testing.T) {
	testCases := []struct {
		name     string
		deploy   *api.Deployment
		expected bool
	}{
		{
			name: "no task groups",
			deploy: &api.Deployment{
				TaskGroups: map[string]*api.DeploymentState{},
			},
			expected: false,
		},
		{
			name: "no auto revert",
			deploy: &api.Deployment{
				TaskGroups: map[string]*api.DeploymentState{
					"web": {AutoRevert: false},
					"api": {AutoRevert: false},
				},
			},
			expected: false,
		},
		{
			name: "one has auto revert",
			deploy: &api.Deployment{
				TaskGroups: map[string]*api.DeploymentState{
					"web": {AutoRevert: true},
					"api": {AutoRevert: false},
				},
			},
			expected: true,
		},
		{
			name: "all have auto revert",
			deploy: &api.Deployment{
				TaskGroups: map[string]*api.DeploymentState{
					"web": {AutoRevert: true},
					"api": {AutoRevert: true},
				},
			},
			expected: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := hasAutoRevert(tc.deploy)
			must.Eq(t, tc.expected, result)
		})
	}
}

func TestFormatDeploymentGroups(t *testing.T) {
	testCases := []struct {
		name       string
		deploy     *api.Deployment
		uuidLength int
		contains   []string
	}{
		{
			name: "basic deployment",
			deploy: &api.Deployment{
				TaskGroups: map[string]*api.DeploymentState{
					"web": {
						DesiredTotal:    3,
						PlacedAllocs:    3,
						HealthyAllocs:   3,
						UnhealthyAllocs: 0,
					},
				},
			},
			uuidLength: 8,
			contains:   []string{"Task Group", "Desired", "Placed", "Healthy", "Unhealthy", "web", "3"},
		},
		{
			name: "with auto revert",
			deploy: &api.Deployment{
				TaskGroups: map[string]*api.DeploymentState{
					"web": {
						AutoRevert:      true,
						DesiredTotal:    2,
						PlacedAllocs:    2,
						HealthyAllocs:   2,
						UnhealthyAllocs: 0,
					},
				},
			},
			uuidLength: 8,
			contains:   []string{"Auto Revert", "true"},
		},
		{
			name: "with canaries",
			deploy: &api.Deployment{
				TaskGroups: map[string]*api.DeploymentState{
					"web": {
						DesiredCanaries: 1,
						DesiredTotal:    3,
						PlacedAllocs:    1,
						HealthyAllocs:   1,
						UnhealthyAllocs: 0,
						Promoted:        false,
					},
				},
			},
			uuidLength: 8,
			contains:   []string{"Promoted", "Canaries", "false"},
		},
		{
			name: "with progress deadline",
			deploy: &api.Deployment{
				TaskGroups: map[string]*api.DeploymentState{
					"web": {
						DesiredTotal:      2,
						PlacedAllocs:      2,
						HealthyAllocs:     2,
						UnhealthyAllocs:   0,
						ProgressDeadline:  10 * time.Minute,
						RequireProgressBy: time.Date(2026, 2, 25, 12, 0, 0, 0, time.UTC),
					},
				},
			},
			uuidLength: 8,
			contains:   []string{"Progress Deadline", "2026-02-25"},
		},
		{
			name: "multiple task groups sorted",
			deploy: &api.Deployment{
				TaskGroups: map[string]*api.DeploymentState{
					"web": {
						DesiredTotal:  2,
						PlacedAllocs:  2,
						HealthyAllocs: 2,
					},
					"api": {
						DesiredTotal:  3,
						PlacedAllocs:  3,
						HealthyAllocs: 3,
					},
				},
			},
			uuidLength: 8,
			contains:   []string{"api", "web"},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := formatDeploymentGroups(tc.deploy, tc.uuidLength)
			for _, expected := range tc.contains {
				must.True(t, strings.Contains(result, expected),
					must.Sprintf("expected %q to contain %q", result, expected))
			}
		})
	}
}

func TestFormatMultiregionDeployment(t *testing.T) {
	testCases := []struct {
		name       string
		regions    map[string]*api.Deployment
		uuidLength int
		contains   []string
	}{
		{
			name: "single region",
			regions: map[string]*api.Deployment{
				"us-east-1": {
					ID:     "abc12345-1234-1234-1234-123456789abc",
					Status: "running",
				},
			},
			uuidLength: 8,
			contains:   []string{"Region", "ID", "Status", "us-east-1", "abc12345", "running"},
		},
		{
			name: "multiple regions",
			regions: map[string]*api.Deployment{
				"us-east-1": {
					ID:     "abc12345-1234-1234-1234-123456789abc",
					Status: "running",
				},
				"us-west-2": {
					ID:     "def67890-1234-1234-1234-123456789def",
					Status: "successful",
				},
			},
			uuidLength: 8,
			contains:   []string{"us-east-1", "us-west-2", "running", "successful"},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := formatMultiregionDeployment(tc.regions, tc.uuidLength)
			for _, expected := range tc.contains {
				must.True(t, strings.Contains(result, expected),
					must.Sprintf("expected %q to contain %q", result, expected))
			}
		})
	}
}

func TestFormatAllocListStubs(t *testing.T) {
	now := time.Now()
	createTime := now.Add(-5 * time.Minute).UnixNano()
	modifyTime := now.Add(-2 * time.Minute).UnixNano()

	testCases := []struct {
		name       string
		stubs      []*api.AllocationListStub
		verbose    bool
		uuidLength int
		contains   []string
	}{
		{
			name:       "empty allocations",
			stubs:      []*api.AllocationListStub{},
			verbose:    false,
			uuidLength: 8,
			contains:   []string{"No allocations placed"},
		},
		{
			name: "non-verbose format",
			stubs: []*api.AllocationListStub{
				{
					ID:            "alloc123-1234-1234-1234-123456789abc",
					NodeID:        "node1234-1234-1234-1234-123456789abc",
					TaskGroup:     "web",
					JobVersion:    1,
					DesiredStatus: "run",
					ClientStatus:  "running",
					CreateTime:    createTime,
					ModifyTime:    modifyTime,
				},
			},
			verbose:    false,
			uuidLength: 8,
			contains:   []string{"ID", "Node ID", "Task Group", "Version", "Desired", "Status", "alloc123", "web", "run", "running"},
		},
		{
			name: "verbose format",
			stubs: []*api.AllocationListStub{
				{
					ID:            "alloc123-1234-1234-1234-123456789abc",
					EvalID:        "eval1234-1234-1234-1234-123456789abc",
					NodeID:        "node1234-1234-1234-1234-123456789abc",
					NodeName:      "nomad-client-1",
					TaskGroup:     "web",
					JobVersion:    2,
					DesiredStatus: "run",
					ClientStatus:  "running",
					CreateTime:    createTime,
					ModifyTime:    modifyTime,
				},
			},
			verbose:    true,
			uuidLength: 36,
			contains:   []string{"ID", "Eval ID", "Node ID", "Node Name", "Task Group", "alloc123-1234-1234-1234-123456789abc", "eval1234", "nomad-client-1"},
		},
		{
			name: "multiple allocations",
			stubs: []*api.AllocationListStub{
				{
					ID:            "alloc111-1234-1234-1234-123456789abc",
					NodeID:        "node1234-1234-1234-1234-123456789abc",
					TaskGroup:     "web",
					JobVersion:    1,
					DesiredStatus: "run",
					ClientStatus:  "running",
					CreateTime:    createTime,
					ModifyTime:    modifyTime,
				},
				{
					ID:            "alloc222-1234-1234-1234-123456789abc",
					NodeID:        "node5678-1234-1234-1234-123456789abc",
					TaskGroup:     "api",
					JobVersion:    1,
					DesiredStatus: "run",
					ClientStatus:  "pending",
					CreateTime:    createTime,
					ModifyTime:    modifyTime,
				},
			},
			verbose:    false,
			uuidLength: 8,
			contains:   []string{"alloc111", "alloc222", "web", "api", "running", "pending"},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := formatAllocListStubs(tc.stubs, tc.verbose, tc.uuidLength)
			for _, expected := range tc.contains {
				must.True(t, strings.Contains(result, expected),
					must.Sprintf("expected %q to contain %q", result, expected))
			}
		})
	}
}

func TestNewEvalState(t *testing.T) {
	state := newEvalState()
	must.NotNil(t, state)
	must.Eq(t, api.EvalStatusPending, state.status)
	must.NotNil(t, state.allocs)
	must.Eq(t, 0, len(state.allocs))
}

func TestMonitor_Update_Eval(t *testing.T) {
	var stdout, stderr bytes.Buffer
	ui := testui.NonInteractiveTestUI(context.Background(), &stdout, &stderr)
	mon := newMonitor(context.Background(), ui, nil, fullId)

	// Evals triggered by jobs log
	state := &evalState{
		status: api.EvalStatusPending,
		job:    "job1",
	}
	mon.update(state)

	out := stdout.String()
	if !strings.Contains(out, "job1") {
		t.Fatalf("missing job\n\n%s", out)
	}
	stdout.Reset()

	// Evals triggered by nodes log
	state = &evalState{
		status: api.EvalStatusPending,
		node:   "12345678-abcd-efab-cdef-123456789abc",
	}
	mon.update(state)

	out = stdout.String()
	if !strings.Contains(out, "12345678-abcd-efab-cdef-123456789abc") {
		t.Fatalf("missing node\n\n%s", out)
	}

	// Transition to pending should not be logged
	if strings.Contains(out, api.EvalStatusPending) {
		t.Fatalf("should skip status\n\n%s", out)
	}
	stdout.Reset()

	// No logs sent if no update
	mon.update(state)
	if out := stdout.String(); out != "" {
		t.Fatalf("expected no output\n\n%s", out)
	}

	// Status change sends more logs
	state = &evalState{
		status: api.EvalStatusComplete,
		node:   "12345678-abcd-efab-cdef-123456789abc",
	}
	mon.update(state)
	out = stdout.String()
	if !strings.Contains(out, api.EvalStatusComplete) {
		t.Fatalf("missing status\n\n%s", out)
	}
}

func TestMonitor_Update_Allocs(t *testing.T) {
	var stdout, stderr bytes.Buffer
	ui := testui.NonInteractiveTestUI(context.Background(), &stdout, &stderr)
	mon := newMonitor(context.Background(), ui, nil, fullId)

	// New allocations write new logs
	state := &evalState{
		allocs: map[string]*allocState{
			"alloc1": {
				id:      "87654321-abcd-efab-cdef-123456789abc",
				group:   "group1",
				node:    "12345678-abcd-efab-cdef-123456789abc",
				desired: api.AllocDesiredStatusRun,
				client:  api.AllocClientStatusPending,
				index:   1,
			},
		},
	}
	mon.update(state)

	// Logs were output
	out := stdout.String()
	if !strings.Contains(out, "87654321-abcd-efab-cdef-123456789abc") {
		t.Fatalf("missing alloc\n\n%s", out)
	}
	if !strings.Contains(out, "group1") {
		t.Fatalf("missing group\n\n%s", out)
	}
	if !strings.Contains(out, "12345678-abcd-efab-cdef-123456789abc") {
		t.Fatalf("missing node\n\n%s", out)
	}
	if !strings.Contains(out, "created") {
		t.Fatalf("missing created\n\n%s", out)
	}
	stdout.Reset()

	// No change yields no logs
	mon.update(state)
	if out := stdout.String(); out != "" {
		t.Fatalf("expected no output\n\n%s", out)
	}
	stdout.Reset()

	// Alloc updates cause more log lines
	state = &evalState{
		allocs: map[string]*allocState{
			"alloc1": {
				id:      "87654321-abcd-efab-cdef-123456789abc",
				group:   "group1",
				node:    "12345678-abcd-efab-cdef-123456789abc",
				desired: api.AllocDesiredStatusRun,
				client:  api.AllocClientStatusRunning,
				index:   2,
			},
		},
	}
	mon.update(state)

	// Updates were logged
	out = stdout.String()
	if !strings.Contains(out, "87654321-abcd-efab-cdef-123456789abc") {
		t.Fatalf("missing alloc\n\n%s", out)
	}
	if !strings.Contains(out, "pending") {
		t.Fatalf("missing old status\n\n%s", out)
	}
	if !strings.Contains(out, "running") {
		t.Fatalf("missing new status\n\n%s", out)
	}
}

func TestMonitor_Update_AllocModification(t *testing.T) {
	var stdout, stderr bytes.Buffer
	ui := testui.NonInteractiveTestUI(context.Background(), &stdout, &stderr)
	mon := newMonitor(context.Background(), ui, nil, fullId)

	// New allocs with a create index lower than the
	// eval create index are logged as modifications
	state := &evalState{
		index: 2,
		allocs: map[string]*allocState{
			"alloc3": {
				id:    "87654321-abcd-bafe-cdef-123456789abc",
				node:  "12345678-abcd-efab-cdef-123456789abc",
				group: "group2",
				index: 1,
			},
		},
	}
	mon.update(state)

	// Modification was logged
	out := stdout.String()
	if !strings.Contains(out, "87654321-abcd-bafe-cdef-123456789abc") {
		t.Fatalf("missing alloc\n\n%s", out)
	}
	if !strings.Contains(out, "group2") {
		t.Fatalf("missing group\n\n%s", out)
	}
	if !strings.Contains(out, "12345678-abcd-efab-cdef-123456789abc") {
		t.Fatalf("missing node\n\n%s", out)
	}
	if !strings.Contains(out, "modified") {
		t.Fatalf("missing modification\n\n%s", out)
	}
}

func TestFormatAllocMetrics(t *testing.T) {

	testCases := []struct {
		name     string
		metrics  *api.AllocationMetric
		scores   bool
		contains []string
	}{
		{
			name: "no nodes evaluated",
			metrics: &api.AllocationMetric{
				NodesEvaluated: 0,
			},
			scores:   false,
			contains: []string{"No nodes were eligible for evaluation"},
		},
		{
			name: "no nodes available in dc",
			metrics: &api.AllocationMetric{
				NodesEvaluated: 1,
				NodesAvailable: map[string]int{
					"dc1": 0,
					"dc2": 5,
				},
			},
			scores:   false,
			contains: []string{"No nodes are available in datacenter", "dc1"},
		},
		{
			name: "class filtered",
			metrics: &api.AllocationMetric{
				NodesEvaluated: 5,
				ClassFiltered: map[string]int{
					"high-memory": 3,
				},
			},
			scores:   false,
			contains: []string{"Class", "high-memory", "nodes excluded by filter"},
		},
		{
			name: "constraint filtered",
			metrics: &api.AllocationMetric{
				NodesEvaluated: 5,
				ConstraintFiltered: map[string]int{
					"${attr.kernel.name} = linux": 2,
				},
			},
			scores:   false,
			contains: []string{"Constraint", "kernel.name", "nodes excluded by filter"},
		},
		{
			name: "resources exhausted",
			metrics: &api.AllocationMetric{
				NodesEvaluated: 5,
				NodesExhausted: 3,
			},
			scores:   false,
			contains: []string{"Resources exhausted on 3 nodes"},
		},
		{
			name: "class exhausted",
			metrics: &api.AllocationMetric{
				NodesEvaluated: 5,
				ClassExhausted: map[string]int{
					"high-cpu": 2,
				},
			},
			scores:   false,
			contains: []string{"Class", "high-cpu", "exhausted on 2 nodes"},
		},
		{
			name: "dimension exhausted",
			metrics: &api.AllocationMetric{
				NodesEvaluated: 5,
				DimensionExhausted: map[string]int{
					"memory": 4,
				},
			},
			scores:   false,
			contains: []string{"Dimension", "memory", "exhausted on 4 nodes"},
		},
		{
			name: "quota exhausted",
			metrics: &api.AllocationMetric{
				NodesEvaluated: 5,
				QuotaExhausted: []string{"memory"},
			},
			scores:   false,
			contains: []string{"Quota limit hit", "memory"},
		},
		{
			name: "display all possible scores",
			metrics: &api.AllocationMetric{
				NodesEvaluated: 3,
				NodesInPool:    3,
				ScoreMetaData: []*api.NodeScoreMeta{
					{
						NodeID: "node-1",
						Scores: map[string]float64{
							"score-1": 1,
							"score-2": 2,
						},
						NormScore: 1,
					},
					{
						NodeID: "node-2",
						Scores: map[string]float64{
							"score-1": 1,
							"score-3": 3,
						},
						NormScore: 2,
					},
					{
						NodeID: "node-3",
						Scores: map[string]float64{
							"score-4": 4,
						},
						NormScore: 3,
					},
				},
			},
			scores:   true,
			contains: []string{"Node", "score-1", "score-2", "score-3", "score-4", "final score", "node-1", "node-2", "node-3"},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := formatAllocMetrics(tc.metrics, tc.scores, "  ")
			for _, expected := range tc.contains {
				must.True(t, strings.Contains(result, expected),
					must.Sprintf("expected %q to contain %q", result, expected))
			}
		})
	}
}

func TestNewPrefixedUI(t *testing.T) {
	var stdout, stderr bytes.Buffer
	ui := testui.NonInteractiveTestUI(context.Background(), &stdout, &stderr)

	prefixed := ui.WithPrefix("[my-job] ")
	prefixed.Info("test message")

	out := stdout.String()
	must.True(t, strings.Contains(out, "[my-job]"),
		must.Sprintf("expected prefix in output: %q", out))
	must.True(t, strings.Contains(out, "test message"),
		must.Sprintf("expected message in output: %q", out))
}

func TestDeploymentInfo(t *testing.T) {
	info := deploymentInfo{
		jobID:        "test-job",
		deploymentID: "abc123",
		wait:         5 * time.Second,
	}
	must.Eq(t, "test-job", info.jobID)
	must.Eq(t, "abc123", info.deploymentID)
	must.Eq(t, 5*time.Second, info.wait)
}

func TestEvalResult(t *testing.T) {
	result := evalResult{
		jobID:        "test-job",
		evalID:       "eval-123",
		deploymentID: "deploy-456",
		exitCode:     0,
		schedFailure: false,
		wait:         10 * time.Second,
	}
	must.Eq(t, "test-job", result.jobID)
	must.Eq(t, "eval-123", result.evalID)
	must.Eq(t, "deploy-456", result.deploymentID)
	must.Eq(t, 0, result.exitCode)
	must.False(t, result.schedFailure)
	must.Eq(t, 10*time.Second, result.wait)
}
