// Copyright IBM Corp. 2023, 2026
// SPDX-License-Identifier: MPL-2.0

package cli

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/fatih/color"
	"github.com/hashicorp/nomad-pack/terminal"
	"github.com/hashicorp/nomad/api"
	"github.com/mitchellh/go-glint"
)

// bold is a color printer for bold text
var bold = color.New(color.Bold)

const (
	// updateWait is the amount of time to wait between status
	// updates. Because the monitor is poll-based, we use this
	// delay to avoid overwhelming the API server.
	updateWait = time.Second

	// shortId and fullId determine how IDs are displayed in the UI
	shortId = 8
	fullId  = 36
)

// evalState is used to store the current "state of the world"
// in the context of monitoring an evaluation.
type evalState struct {
	status     string
	desc       string
	node       string
	deployment string
	job        string
	allocs     map[string]*allocState
	wait       time.Duration
	index      uint64
}

// newEvalState creates and initializes a new monitorState
func newEvalState() *evalState {
	return &evalState{
		status: api.EvalStatusPending,
		allocs: make(map[string]*allocState),
	}
}

// allocState is used to track the state of an allocation
type allocState struct {
	id          string
	group       string
	node        string
	desired     string
	desiredDesc string
	client      string
	clientDesc  string
	index       uint64
}

// monitor wraps an evaluation monitor and holds metadata and
// state information.
type monitor struct {
	ctx    context.Context
	ui     terminal.UI
	client *api.Client
	state  *evalState

	// length determines the number of characters for identifiers in the ui.
	length int

	sync.Mutex
}

// newMonitor returns a new monitor. The returned monitor will
// write output information to the provided ui. The length parameter determines
// the number of characters for identifiers in the ui.
func newMonitor(ctx context.Context, ui terminal.UI, client *api.Client, length int) *monitor {
	mon := &monitor{
		ctx:    ctx,
		ui:     ui,
		client: client,
		state:  newEvalState(),
		length: length,
	}
	return mon
}

// update is used to update our monitor with new state. It can be
// called whether the passed information is new or not, and will
// only dump update messages when state changes.
func (m *monitor) update(update *evalState) {
	m.Lock()
	defer m.Unlock()

	existing := m.state

	// Swap in the new state at the end
	defer func() {
		m.state = update
	}()

	// Check if the evaluation was triggered by a node
	if existing.node == "" && update.node != "" {
		m.ui.Info(fmt.Sprintf("%s: Evaluation triggered by node %q",
			formatTime(time.Now()), limit(update.node, m.length)))
	}

	// Check if the evaluation was triggered by a job
	if existing.job == "" && update.job != "" {
		m.ui.Info(fmt.Sprintf("%s: Evaluation triggered by job %q",
			formatTime(time.Now()), update.job))
	}

	// Check if the evaluation was triggered by a deployment
	if existing.deployment == "" && update.deployment != "" {
		m.ui.Info(fmt.Sprintf("%s: Evaluation within deployment: %q",
			formatTime(time.Now()), limit(update.deployment, m.length)))
	}

	// Check the allocations
	for allocID, alloc := range update.allocs {
		if existing, ok := existing.allocs[allocID]; !ok {
			switch {
			case alloc.index < update.index:
				// New alloc with create index lower than the eval
				// create index indicates modification
				m.ui.Info(fmt.Sprintf(
					"%s: Allocation %q modified: node %q, group %q",
					formatTime(time.Now()), limit(alloc.id, m.length),
					limit(alloc.node, m.length), alloc.group))

			case alloc.desired == api.AllocDesiredStatusRun:
				// New allocation with desired status running
				m.ui.Info(fmt.Sprintf(
					"%s: Allocation %q created: node %q, group %q",
					formatTime(time.Now()), limit(alloc.id, m.length),
					limit(alloc.node, m.length), alloc.group))
			}
		} else if existing.client != alloc.client {
			description := ""
			if alloc.clientDesc != "" {
				description = fmt.Sprintf(" (%s)", alloc.clientDesc)
			}
			// Allocation status has changed
			m.ui.Info(fmt.Sprintf(
				"%s: Allocation %q status changed: %q -> %q%s",
				formatTime(time.Now()), limit(alloc.id, m.length),
				existing.client, alloc.client, description))
		}
	}

	// Check if the status changed. We skip any transitions to pending status.
	if existing.status != "" &&
		update.status != api.AllocClientStatusPending &&
		existing.status != update.status {
		m.ui.Info(fmt.Sprintf("%s: Evaluation status changed: %q -> %q",
			formatTime(time.Now()), existing.status, update.status))
	}
}

// monitor is used to start monitoring the given evaluation ID. It
// writes output directly to the monitor's ui, and returns the
// exit code for the command.
//
// The return code will be 0 on successful evaluation. If there are
// problems scheduling the job (impossible constraints, resources
// exhausted, etc), then the return code will be 2. For any other
// failures (API connectivity, internal errors, etc), the return code
// will be 1.

func (m *monitor) monitor(evalIDs []string) int {

	verbose := m.length == fullId

	// Phase 1: Monitor all evaluations in parallel
	deployments, evalExitCode := monitorAllEvaluations(m.ctx, m.ui, m.client, evalIDs, m.length)
	if evalExitCode != 0 && len(deployments) == 0 {
		return evalExitCode
	}

	// Phase 2: Monitor all deployments in parallel
	if len(deployments) > 0 {
		deployExitCode := monitorAllDeployments(m.ctx, m.ui, m.client, deployments, verbose)
		if deployExitCode != 0 {
			return deployExitCode
		}
	}

	return evalExitCode
}

// evalResult holds the result of monitoring a single evaluation
type evalResult struct {
	jobID        string
	evalID       string
	deploymentID string
	exitCode     int
	schedFailure bool
	wait         time.Duration
}

// monitorEval monitors just the evaluation, without monitoring the deployment.
// It returns the result including the deployment ID for later parallel deployment monitoring.
func (m *monitor) monitorEval(evalID string) evalResult {
	result := evalResult{evalID: evalID}

	// Add the initial pending state
	m.update(newEvalState())

	m.ui.Info(fmt.Sprintf("%s: Monitoring evaluation %q",
		formatTime(time.Now()), limit(evalID, m.length)))

	for {
		// Check for context cancellation
		select {
		case <-m.ctx.Done():
			result.exitCode = 1
			return result
		default:
		}

		// Query the evaluation
		eval, _, err := m.client.Evaluations().Info(evalID, nil)
		if err != nil {
			if m.ctx.Err() != nil {
				m.ui.Warning(fmt.Sprintf("%s: Monitoring cancelled", formatTime(time.Now())))
				result.exitCode = 1
				return result
			}
			m.ui.Error(fmt.Sprintf("No evaluation with id %q found", evalID))
			result.exitCode = 1
			return result
		}

		result.jobID = eval.JobID

		// Create the new eval state.
		state := newEvalState()
		state.status = eval.Status
		state.desc = eval.StatusDescription
		state.node = eval.NodeID
		state.job = eval.JobID
		state.deployment = eval.DeploymentID
		state.wait = eval.Wait
		state.index = eval.CreateIndex

		// Query the allocations associated with the evaluation
		allocs, _, err := m.client.Evaluations().Allocations(eval.ID, nil)
		if err != nil {
			if m.ctx.Err() != nil {
				m.ui.Warning(fmt.Sprintf("%s: Monitoring cancelled", formatTime(time.Now())))
				result.exitCode = 1
				return result
			}
			m.ui.Error(fmt.Sprintf("%s: Error reading allocations: %s", formatTime(time.Now()), err))
			result.exitCode = 1
			return result
		}

		// Add the allocs to the state
		for _, alloc := range allocs {
			state.allocs[alloc.ID] = &allocState{
				id:          alloc.ID,
				group:       alloc.TaskGroup,
				node:        alloc.NodeID,
				desired:     alloc.DesiredStatus,
				desiredDesc: alloc.DesiredDescription,
				client:      alloc.ClientStatus,
				clientDesc:  alloc.ClientDescription,
				index:       alloc.CreateIndex,
			}
		}

		// Update the state
		m.update(state)

		switch eval.Status {
		case api.EvalStatusComplete, api.EvalStatusFailed, api.EvalStatusCancelled:
			if len(eval.FailedTGAllocs) == 0 {
				m.ui.Info(fmt.Sprintf("%s: Evaluation %q finished with status %q",
					formatTime(time.Now()), limit(eval.ID, m.length), eval.Status))
			} else {
				// There were failures making the allocations
				result.schedFailure = true
				m.ui.Info(fmt.Sprintf("%s: Evaluation %q finished with status %q but failed to place all allocations:",
					formatTime(time.Now()), limit(eval.ID, m.length), eval.Status))

				// Print the failures per task group
				for tg, metrics := range eval.FailedTGAllocs {
					noun := "allocation"
					if metrics.CoalescedFailures > 0 {
						noun += "s"
					}
					m.ui.Info(fmt.Sprintf("%s: Task Group %q (failed to place %d %s):",
						formatTime(time.Now()), tg, metrics.CoalescedFailures+1, noun))
					metrics := formatAllocMetrics(metrics, false, "  ")
					for _, line := range strings.Split(metrics, "\n") {
						m.ui.Info(line)
					}
				}

				if eval.BlockedEval != "" {
					m.ui.Info(fmt.Sprintf("%s: Evaluation %q waiting for additional capacity to place remainder",
						formatTime(time.Now()), limit(eval.BlockedEval, m.length)))
				}
			}
		default:
			// Wait for the next update
			select {
			case <-m.ctx.Done():
				result.exitCode = 1
				return result
			case <-time.After(updateWait):
			}
			continue
		}

		// Monitor the next eval in the chain, if present
		if eval.NextEval != "" {
			if eval.Wait.Nanoseconds() != 0 {
				m.ui.Info(fmt.Sprintf(
					"%s: Monitoring next evaluation %q in %s",
					formatTime(time.Now()), limit(eval.NextEval, m.length), eval.Wait))

				// Skip some unnecessary polling
				select {
				case <-m.ctx.Done():
					result.exitCode = 1
					return result
				case <-time.After(eval.Wait):
				}
			}

			// Reset the state and monitor the new eval
			m.state = newEvalState()
			return m.monitorEval(eval.NextEval)
		}
		break
	}

	// Capture deployment info for later parallel monitoring
	result.deploymentID = m.state.deployment
	result.wait = m.state.wait
	return result
}

// deploymentInfo holds information needed to monitor a deployment
type deploymentInfo struct {
	jobID        string
	deploymentID string
	wait         time.Duration
}

// monitorAllEvaluations monitors all evaluations in parallel using prefixed output.
// It returns the deployment info for all jobs that have deployments.
func monitorAllEvaluations(ctx context.Context, ui terminal.UI, client *api.Client, evalIDs []string, length int) ([]deploymentInfo, int) {
	if len(evalIDs) == 0 {
		return nil, 0
	}

	// For a single job, just monitor directly
	if len(evalIDs) == 1 {
		mon := newMonitor(ctx, ui, client, length)
		result := mon.monitorEval(evalIDs[0])
		var deployments []deploymentInfo
		if result.deploymentID != "" {
			deployments = append(deployments, deploymentInfo{
				jobID:        result.jobID,
				deploymentID: result.deploymentID,
				wait:         result.wait,
			})
		}
		if result.schedFailure {
			return deployments, 2
		}
		return deployments, result.exitCode
	}

	// Multiple jobs - monitor in parallel with prefixed output
	results := make(chan evalResult, len(evalIDs))
	var wg sync.WaitGroup

	for _, evalID := range evalIDs {
		wg.Add(1)
		go func(eID string) {
			defer wg.Done()
			// Use a temporary non-prefixed monitor to get the job ID first
			tempMon := newMonitor(ctx, ui, client, length)
			eval, _, err := tempMon.client.Evaluations().Info(eID, nil)
			if err != nil {
				if ctx.Err() != nil {
					return
				}
				ui.Error(fmt.Sprintf("No evaluation with id %q found", eID))
				results <- evalResult{evalID: eID, exitCode: 1}
				return
			}

			// Now create a prefixed UI with the job ID
			prefixedUI := ui.WithPrefix(fmt.Sprintf("[%s] ", eval.JobID))
			mon := newMonitor(ctx, prefixedUI, client, length)
			result := mon.monitorEval(eID)
			results <- result
		}(evalID)
	}

	// Wait for all monitors to complete
	go func() {
		wg.Wait()
		close(results)
	}()

	// Collect results
	var deployments []deploymentInfo
	var finalCode int
	var hasSchedFailure bool

	for result := range results {
		if result.exitCode > finalCode {
			finalCode = result.exitCode
		}
		if result.schedFailure {
			hasSchedFailure = true
		}
		if result.deploymentID != "" {
			deployments = append(deployments, deploymentInfo{
				jobID:        result.jobID,
				deploymentID: result.deploymentID,
				wait:         result.wait,
			})
		}
	}

	if hasSchedFailure && finalCode == 0 {
		finalCode = 2
	}

	return deployments, finalCode
}

// monitorDeployment monitors the deployment and returns the final status.
// It uses the UI's Status interface if available for in-place rendering,
// otherwise falls back to basic text output.
func monitorDeployment(ctx context.Context, ui terminal.UI, client *api.Client, deployID string, index uint64, wait time.Duration, verbose bool) (string, error) {
	// Check if UI is interactive (supports Status spinner)
	if ui.Interactive() {
		return ttyMonitorDeployment(ctx, ui, client, deployID, index, wait, verbose)
	}
	// Fall back to basic text output
	return basicMonitorDeployment(ctx, ui, client, deployID, index, wait, verbose)
}

// deploymentResult holds the result of monitoring a deployment
type deploymentResult struct {
	jobID  string
	status string
	err    error
}

// monitorAllDeployments monitors all deployments in parallel.
// It calls the existing monitorDeployment function for each deployment in a goroutine,
// which handles TTY (with spinner) and non-TTY output appropriately.
func monitorAllDeployments(ctx context.Context, ui terminal.UI, client *api.Client, deployments []deploymentInfo, verbose bool) int {
	if len(deployments) == 0 {
		return 0
	}

	// For a single deployment, just monitor directly
	if len(deployments) == 1 {
		d := deployments[0]
		status, err := monitorDeployment(ctx, ui, client, d.deploymentID, 0, d.wait, verbose)
		if err != nil || status != api.DeploymentStatusSuccessful {
			return 1
		}
		return 0
	}

	// Multiple deployments - monitor in parallel
	ui.Info(fmt.Sprintf("\n%s: Monitoring %d deployment(s)...", formatTime(time.Now()), len(deployments)))

	results := make(chan deploymentResult, len(deployments))
	var wg sync.WaitGroup

	for _, d := range deployments {
		wg.Add(1)
		go func(info deploymentInfo) {
			defer wg.Done()

			status, err := monitorDeployment(ctx, ui, client, info.deploymentID, 0, info.wait, verbose)
			results <- deploymentResult{jobID: info.jobID, status: status, err: err}
		}(d)
	}

	// Wait for all monitors to complete
	go func() {
		wg.Wait()
		close(results)
	}()

	// Collect results
	var exitCode int
	for result := range results {
		if result.err != nil || result.status != api.DeploymentStatusSuccessful {
			exitCode = 1
		}
	}

	return exitCode
}

// ttyMonitorDeployment provides a rich terminal UI for monitoring deployments
// using a single LiveView that combines spinner, deployment details, and allocations.
func ttyMonitorDeployment(ctx context.Context, ui terminal.UI, client *api.Client, deployID string, index uint64, wait time.Duration, verbose bool) (status string, err error) {
	var length int
	if verbose {
		length = fullId
	} else {
		length = shortId
	}

	// Single LiveView for all components
	liveView := ui.LiveView()
	defer liveView.Close()

	// Status component for spinner
	st := terminal.CreateComponent(terminal.NewGlintStatus)
	defer st.Close()

	st.Update(fmt.Sprintf("Deployment %q in progress...", limit(deployID, length)))

	q := &api.QueryOptions{
		AllowStale: true,
		WaitIndex:  index,
		WaitTime:   wait,
	}
	q = q.WithContext(ctx)

	for {
		// Check for context cancellation
		select {
		case <-ctx.Done():
			st.Close()
			liveView.Close()
			err = ctx.Err()
			return
		default:
		}

		var deploy *api.Deployment
		var meta *api.QueryMeta
		deploy, meta, err = client.Deployments().Info(deployID, q)
		if err != nil {
			st.Step(terminal.StatusError, fmt.Sprintf("Error fetching deployment %q: %v", limit(deployID, length), err))
			return
		}

		status = deploy.Status

		// Build the combined component
		components := []glint.Component{
			glint.Layout(
				glint.Text(fmt.Sprintf("\n%s: Monitoring deployment %q", formatTime(time.Now()), limit(deployID, length))),
			).MarginLeft(2),
			st,
			glint.Layout(
				glint.Text(""),
				glint.Text(formatTime(time.Now())),
				glint.Text(formatDeployment(client, deploy, length)),
			).MarginLeft(4),
		}

		// Add allocations if verbose
		if verbose {
			allocs, _, allocErr := client.Deployments().Allocations(deployID, nil)
			if allocErr != nil {
				components = append(components, glint.Layout(
					glint.Text(""),
					glint.Style(
						glint.Text("Allocations"),
						glint.Bold(),
					),
					glint.Style(
						glint.Text(fmt.Sprintf("Error fetching allocations: %v", allocErr)),
						glint.Color("red"),
					),
				).MarginLeft(4))
			} else if len(allocs) > 0 {
				components = append(components, glint.Layout(
					glint.Text(""),
					glint.Style(
						glint.Text("Allocations"),
						glint.Bold(),
					),
					glint.Text(formatAllocListStubs(allocs, verbose, length)),
				).MarginLeft(4))
			}
		}

		// Update single LiveView with all components
		liveView.SetComponents(components...)

		switch status {
		case api.DeploymentStatusFailed:
			if hasAutoRevert(deploy) {
				st.Step(terminal.StatusWarn, fmt.Sprintf("Deployment %q failed, waiting for rollback...", limit(deployID, length)))

				// Wait for rollback to launch
				select {
				case <-ctx.Done():
					st.Close()
					liveView.Close()
					err = ctx.Err()
					return
				case <-time.After(1 * time.Second):
				}
				var rollback *api.Deployment
				rollback, _, err = client.Jobs().LatestDeployment(deploy.JobID, nil)

				if err != nil {
					st.Step(terminal.StatusError, fmt.Sprintf("Error fetching rollback deployment for %q: %v", limit(deployID, length), err))
					return
				}

				// Check for noop/no target rollbacks
				if rollback == nil || rollback.ID == deploy.ID {
					st.Step(terminal.StatusError, fmt.Sprintf("Deployment %q failed", limit(deployID, length)))
					return
				}

				// Close current views before monitoring rollback
				st.Close()
				liveView.Close()
				return ttyMonitorDeployment(ctx, ui, client, rollback.ID, index, wait, verbose)
			}
			st.Step(terminal.StatusError, fmt.Sprintf("Deployment %q failed", limit(deployID, length)))
			return
		case api.DeploymentStatusSuccessful:
			st.Step(terminal.StatusOK, fmt.Sprintf("Deployment %q successful", limit(deployID, length)))
			return
		case api.DeploymentStatusCancelled:
			st.Step(terminal.StatusWarn, fmt.Sprintf("Deployment %q cancelled", limit(deployID, length)))
			return
		case api.DeploymentStatusBlocked:
			st.Step(terminal.StatusWarn, fmt.Sprintf("Deployment %q blocked", limit(deployID, length)))
			return
		default:
			q.WaitIndex = meta.LastIndex
			continue
		}
	}
}

// basicMonitorDeployment provides simple text-based monitoring for non-TTY UIs.
// It polls the deployment status and only prints the final result.
func basicMonitorDeployment(ctx context.Context, ui terminal.UI, client *api.Client, deployID string, index uint64, wait time.Duration, verbose bool) (status string, err error) {
	var length int
	if verbose {
		length = fullId
	} else {
		length = shortId
	}

	ui.Info(fmt.Sprintf("\n%s: Monitoring deployment %q", formatTime(time.Now()), limit(deployID, length)))

	q := &api.QueryOptions{
		AllowStale: true,
		WaitIndex:  index,
		WaitTime:   wait,
	}
	q = q.WithContext(ctx)

	var deploy *api.Deployment
	for {
		// Check for context cancellation
		select {
		case <-ctx.Done():
			ui.Warning(fmt.Sprintf("%s: Deployment %q monitoring cancelled", formatTime(time.Now()), limit(deployID, length)))
			err = ctx.Err()
			return
		default:
		}

		var meta *api.QueryMeta
		deploy, meta, err = client.Deployments().Info(deployID, q)
		if err != nil {
			if ctx.Err() != nil {
				ui.Warning(fmt.Sprintf("%s: Deployment %q monitoring cancelled", formatTime(time.Now()), limit(deployID, length)))
				err = ctx.Err()
				return
			}
			ui.Error(fmt.Sprintf("%s: Error fetching deployment %q: %v", formatTime(time.Now()), limit(deployID, length), err))
			return
		}

		status = deploy.Status

		switch status {
		case api.DeploymentStatusFailed:
			if hasAutoRevert(deploy) {
				ui.Warning(fmt.Sprintf("%s: Deployment %q failed, waiting for rollback...", formatTime(time.Now()), limit(deployID, length)))
				select {
				case <-ctx.Done():
					ui.Warning(fmt.Sprintf("%s: Deployment %q monitoring cancelled", formatTime(time.Now()), limit(deployID, length)))
					err = ctx.Err()
					return
				case <-time.After(1 * time.Second):
				}
				var rollback *api.Deployment
				rollback, _, err = client.Jobs().LatestDeployment(deploy.JobID, nil)
				if err != nil {
					ui.Error(fmt.Sprintf("%s: Error fetching rollback deployment for %q: %v", formatTime(time.Now()), limit(deployID, length), err))
					return
				}
				if rollback == nil || rollback.ID == deploy.ID {
					ui.Error(fmt.Sprintf("%s: Deployment %q failed", formatTime(time.Now()), limit(deployID, length)))
					return
				}
				return basicMonitorDeployment(ctx, ui, client, rollback.ID, index, wait, verbose)
			}
			ui.Error(fmt.Sprintf("%s: Deployment %q failed", formatTime(time.Now()), limit(deployID, length)))
		case api.DeploymentStatusSuccessful:
			ui.Success(fmt.Sprintf("%s: Deployment %q successful", formatTime(time.Now()), limit(deployID, length)))
		case api.DeploymentStatusCancelled:
			ui.Warning(fmt.Sprintf("%s: Deployment %q cancelled", formatTime(time.Now()), limit(deployID, length)))
		case api.DeploymentStatusBlocked:
			ui.Warning(fmt.Sprintf("%s: Deployment %q blocked", formatTime(time.Now()), limit(deployID, length)))
		default:
			q.WaitIndex = meta.LastIndex
			select {
			case <-ctx.Done():
				err = ctx.Err()
				return
			case <-time.After(updateWait):
			}
			continue
		}
		break
	}

	// Print final deployment details
	ui.Info("")
	ui.Info(formatTime(time.Now()))
	ui.Info(formatDeployment(client, deploy, length))

	// Print allocations if verbose
	if verbose {

		allocs, _, allocErr := client.Deployments().Allocations(deployID, nil)
		if allocErr != nil {
			ui.Error(fmt.Sprintf("%s: Error fetching allocations for deployment %q: %v", formatTime(time.Now()), limit(deployID, length), allocErr))
		} else if len(allocs) > 0 {
			ui.Info("")
			ui.Info(bold.Sprint("Allocations"))
			ui.Info(formatAllocListStubs(allocs, verbose, length))
		}
	}

	return
}

// formatAllocMetrics iterates the passed allocation metrics and returns a
// formatted string representation. Critical or important information is colored
// red to draw attention.
func formatAllocMetrics(
	metrics *api.AllocationMetric,
	scores bool,
	prefix string,
) string {

	// Print a helpful message if we have an eligibility problem
	var out string

	if metrics.NodesEvaluated == 0 {
		out += fmt.Sprintf("%s* No nodes were eligible for evaluation\n", prefix)
	}

	// Print a helpful message if the user has asked for a DC that has no
	// available nodes.
	for dc, available := range metrics.NodesAvailable {
		if available == 0 {
			out += fmt.Sprintf("%s* No nodes are available in datacenter %q\n", prefix, dc)
		}
	}

	// Print filter info
	for class, num := range metrics.ClassFiltered {
		out += fmt.Sprintf("%s* Class %q: %d nodes excluded by filter\n", prefix, class, num)
	}

	// Iterate the placement constraints, highlighting missing drivers in red
	// as this is a common problem we want to draw attention to.
	for cs, num := range metrics.ConstraintFiltered {
		if strings.Contains(cs, "missing drivers") {
			out += color.RedString(
				"%s* Constraint %q: %d nodes excluded by filter",
				prefix, cs, num,
			)
			out += "\n"
		} else {
			out += fmt.Sprintf("%s* Constraint %q: %d nodes excluded by filter\n", prefix, cs, num)
		}
	}

	// Print exhaustion info
	if ne := metrics.NodesExhausted; ne > 0 {
		out += fmt.Sprintf("%s* Resources exhausted on %d nodes\n", prefix, ne)
	}
	for class, num := range metrics.ClassExhausted {
		out += fmt.Sprintf("%s* Class %q exhausted on %d nodes\n", prefix, class, num)
	}
	for dim, num := range metrics.DimensionExhausted {
		out += fmt.Sprintf("%s* Dimension %q exhausted on %d nodes\n", prefix, dim, num)
	}

	// Print quota info
	for _, dim := range metrics.QuotaExhausted {
		out += fmt.Sprintf("%s* Quota limit hit %q\n", prefix, dim)
	}

	// Print scores
	if scores {
		if len(metrics.ScoreMetaData) > 0 {
			scoreOutput := make([]string, len(metrics.ScoreMetaData)+1)

			// Find all possible scores and build header row.
			allScores := make(map[string]struct{})
			for _, scoreMeta := range metrics.ScoreMetaData {
				for score := range scoreMeta.Scores {
					allScores[score] = struct{}{}
				}
			}
			// Sort scores alphabetically.
			scores := make([]string, 0, len(allScores))
			for score := range allScores {
				scores = append(scores, score)
			}
			sort.Strings(scores)
			scoreOutput[0] = fmt.Sprintf("Node|%s|final score", strings.Join(scores, "|"))

			// Build row for each score.
			for i, scoreMeta := range metrics.ScoreMetaData {
				scoreOutput[i+1] = fmt.Sprintf("%v|", scoreMeta.NodeID)
				for _, scorerName := range scores {
					scoreVal := scoreMeta.Scores[scorerName]
					scoreOutput[i+1] += fmt.Sprintf("%.3g|", scoreVal)
				}
				scoreOutput[i+1] += fmt.Sprintf("%.3g", scoreMeta.NormScore)
			}

			out += formatList(scoreOutput)
		} else {
			// Backwards compatibility for old allocs
			for name, score := range metrics.Scores {
				out += fmt.Sprintf("%s* Score %q = %f\n", prefix, name, score)
			}
		}
	}

	out = strings.TrimSuffix(out, "\n")
	return out
}

// hasAutoRevert checks if any task group in the deployment has auto-revert enabled
func hasAutoRevert(deploy *api.Deployment) bool {
	for _, state := range deploy.TaskGroups {
		if state.AutoRevert {
			return true
		}
	}
	return false
}

// regionResult holds the result of fetching a deployment from a specific region
type regionResult struct {
	d      *api.Deployment
	err    error
	region string
}

// formatDeployment formats deployment information for display
func formatDeployment(c *api.Client, d *api.Deployment, uuidLength int) string {
	if d == nil {
		return "No deployment found"
	}
	// Format the high-level elements
	high := []string{
		fmt.Sprintf("ID|%s", limit(d.ID, uuidLength)),
		fmt.Sprintf("Job ID|%s", d.JobID),
		fmt.Sprintf("Job Version|%d", d.JobVersion),
		fmt.Sprintf("Status|%s", d.Status),
		fmt.Sprintf("Description|%s", d.StatusDescription),
	}

	base := formatKV(high)

	// Fetch and Format Multi-region info
	if d.IsMultiregion {
		regions, err := fetchMultiRegionDeployments(c, d)
		if err != nil {
			base += "\n\nError fetching multiregion deployment\n\n"
			base += fmt.Sprintf("%v\n\n", err)
		} else if len(regions) > 0 {
			base += "\n\n" + bold.Sprint("Multiregion Deployment") + "\n"
			base += formatMultiregionDeployment(regions, uuidLength)
		}
	}

	if len(d.TaskGroups) == 0 {
		return base
	}
	base += "\n\n" + bold.Sprint("Deployed") + "\n"
	base += formatDeploymentGroups(d, uuidLength)
	return base
}

// fetchMultiRegionDeployments fetches deploymenmts from all regions for a multiregion job
func fetchMultiRegionDeployments(c *api.Client, d *api.Deployment) (map[string]*api.Deployment, error) {
	results := make(map[string]*api.Deployment)

	job, _, err := c.Jobs().Info(d.JobID, &api.QueryOptions{})
	if err != nil {
		return nil, fmt.Errorf("error fetching job: %v", err)
	}

	if job.Multiregion == nil || len(job.Multiregion.Regions) == 0 {
		return results, nil
	}

	requests := make(chan regionResult, len(job.Multiregion.Regions))
	for i := 0; i < cap(requests); i++ {
		go func(itr int) {
			region := job.Multiregion.Regions[itr]
			deploy, err := fetchRegionDeployment(c, d, region)
			requests <- regionResult{d: deploy, err: err, region: region.Name}
		}(i)
	}
	for i := 0; i < cap(requests); i++ {
		res := <-requests
		if res.err != nil {
			key := fmt.Sprintf("%s (error)", res.region)
			results[key] = &api.Deployment{}
			continue
		}
		results[res.region] = res.d
	}
	return results, nil
}

// fetchRegionDeployment fetches a deployment from a specific region
func fetchRegionDeployment(c *api.Client, d *api.Deployment, region *api.MultiregionRegion) (*api.Deployment, error) {
	opts := &api.QueryOptions{Region: region.Name}
	deploy, _, err := c.Deployments().Info(d.ID, opts)
	if err != nil {
		return nil, err
	}
	return deploy, nil
}

// formatMultiregionDeployment formats multiregion deployment information
func formatMultiregionDeployment(regions map[string]*api.Deployment, uuidLength int) string {
	rowString := "Region|ID|Status"
	rows := make([]string, len(regions)+1)
	rows[0] = rowString
	i := 1
	for k, v := range regions {
		row := fmt.Sprintf("%s|%s|%s", k, limit(v.ID, uuidLength), v.Status)
		rows[i] = row
		i++
	}
	sort.Strings(rows[1:]) // Sort only the data rows, not the header
	return formatList(rows)
}

// formatDeploymentGroups formats task group deployment information
func formatDeploymentGroups(d *api.Deployment, uuidLength int) string {
	// Detect if we need to add these columns
	var canaries, autorevert, progressDeadline bool
	tgNames := make([]string, 0, len(d.TaskGroups))
	for name, state := range d.TaskGroups {
		tgNames = append(tgNames, name)
		if state.AutoRevert {
			autorevert = true
		}
		if state.DesiredCanaries > 0 {
			canaries = true
		}
		if state.ProgressDeadline != 0 {
			progressDeadline = true
		}
	}

	// Sort the task group names to get a reliable ordering
	sort.Strings(tgNames)

	// Build the row string
	rowString := "Task Group|"
	if autorevert {
		rowString += "Auto Revert|"
	}
	if canaries {
		rowString += "Promoted|"
	}
	rowString += "Desired|"
	if canaries {
		rowString += "Canaries|"
	}
	rowString += "Placed|Healthy|Unhealthy"
	if progressDeadline {
		rowString += "|Progress Deadline"
	}

	rows := make([]string, len(d.TaskGroups)+1)
	rows[0] = rowString
	i := 1
	for _, tg := range tgNames {
		state := d.TaskGroups[tg]
		row := fmt.Sprintf("%s|", tg)
		if autorevert {
			row += fmt.Sprintf("%v|", state.AutoRevert)
		}
		if canaries {
			if state.DesiredCanaries > 0 {
				row += fmt.Sprintf("%v|", state.Promoted)
			} else {
				row += fmt.Sprintf("%v|", "N/A")
			}
		}
		row += fmt.Sprintf("%d|", state.DesiredTotal)
		if canaries {
			row += fmt.Sprintf("%d|", state.DesiredCanaries)
		}
		row += fmt.Sprintf("%d|%d|%d", state.PlacedAllocs, state.HealthyAllocs, state.UnhealthyAllocs)
		if progressDeadline {
			if state.RequireProgressBy.IsZero() {
				row += fmt.Sprintf("|%v", "N/A")
			} else {
				row += fmt.Sprintf("|%v", formatTime(state.RequireProgressBy))
			}
		}
		rows[i] = row
		i++
	}

	return formatList(rows)
}

// formatAllocListStubs formats a list of allocation stubs for display
func formatAllocListStubs(stubs []*api.AllocationListStub, verbose bool, uuidLength int) string {
	if len(stubs) == 0 {
		return "No allocations placed"
	}

	allocs := make([]string, len(stubs)+1)
	if verbose {
		allocs[0] = "ID|Eval ID|Node ID|Node Name|Task Group|Version|Desired|Status|Created|Modified"
		for i, alloc := range stubs {
			allocs[i+1] = fmt.Sprintf("%s|%s|%s|%s|%s|%d|%s|%s|%s|%s",
				limit(alloc.ID, uuidLength),
				limit(alloc.EvalID, uuidLength),
				limit(alloc.NodeID, uuidLength),
				alloc.NodeName,
				alloc.TaskGroup,
				alloc.JobVersion,
				alloc.DesiredStatus,
				alloc.ClientStatus,
				formatUnixNanoTime(alloc.CreateTime),
				formatUnixNanoTime(alloc.ModifyTime))
		}
	} else {
		allocs[0] = "ID|Node ID|Task Group|Version|Desired|Status|Created|Modified"
		for i, alloc := range stubs {
			now := time.Now()
			createTimePretty := prettyTimeDiff(time.Unix(0, alloc.CreateTime), now)
			modTimePretty := prettyTimeDiff(time.Unix(0, alloc.ModifyTime), now)
			allocs[i+1] = fmt.Sprintf("%s|%s|%s|%d|%s|%s|%s|%s",
				limit(alloc.ID, uuidLength),
				limit(alloc.NodeID, uuidLength),
				alloc.TaskGroup,
				alloc.JobVersion,
				alloc.DesiredStatus,
				alloc.ClientStatus,
				createTimePretty,
				modTimePretty)
		}
	}

	return formatList(allocs)

}
