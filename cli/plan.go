package cli

import (
	stdErrors "errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/hashicorp/nom/flag"
	"github.com/hashicorp/nom/internal/pkg/errors"
	"github.com/hashicorp/nom/internal/pkg/version"
	"github.com/hashicorp/nom/terminal"
	v1client "github.com/hashicorp/nomad-openapi/clients/go/v1"
	v1 "github.com/hashicorp/nomad-openapi/v1"
	"github.com/hashicorp/nomad/scheduler"
)

const (
	jobModifyIndexHelp = `To submit the job with version verification run:

nomad-pack job run -check-index %d %s

When running the job with the check-index flag, the job will only be run if the
job modify index given matches the server-side version. If the index has
changed, another user has modified the job and the plan's results are
potentially invalid.`

	preemptionDisplayThreshold = 10
)

type PlanCommand struct {
	*baseCommand
	diff           bool
	hcl1           bool
	packName       string
	repoName       string
	jobName        string
	policyOverride bool
	verbose        bool
}

func (c *PlanCommand) Run(args []string) int {
	c.cmdKey = "plan"
	// Initialize. If we fail, we just exit since Init handles the UI.
	if err := c.Init(
		WithExactArgs(1, args),
		WithFlags(c.Flags()),
		WithNoConfig(),
	); err != nil {
		c.ui.ErrorWithContext(err, "error parsing args or flags")
		return 255
	}

	packRepoName := c.args[0]

	// Generate our UI error context.
	errorContext := errors.NewUIErrorContext()

	repoName, packName, err := parseRepoFromPackName(packRepoName)
	if err != nil {
		c.ui.ErrorWithContext(err, "failed to parse pack name", errorContext.GetAll()...)
		return 1
	}
	c.packName = packName
	c.repoName = repoName
	errorContext.Add(errors.UIContextPrefixPackName, c.packName)
	errorContext.Add(errors.UIContextPrefixPackName, c.repoName)

	repoPath, err := getRepoPath(repoName, c.ui, errorContext)
	if err != nil {
		c.ui.ErrorWithContext(err, "failed to identify repository path")
		return 255
	}

	// Add the path to the pack on the error context.
	errorContext.Add(errors.UIContextPrefixPackPath, repoPath)

	// verify packs exist before planning jobs
	if err = verifyPackExist(c.ui, c.packName, repoPath, errorContext); err != nil {
		return 255
	}

	// get pack git version
	// TODO: Get this from pack metadata.
	packVersion, err := version.PackVersion(repoPath)
	if err != nil {
		c.ui.ErrorWithContext(err, "failed to determine pack version", errorContext.GetAll()...)
	}

	// Add the path to the pack on the error context.
	errorContext.Add(errors.UIContextPrefixPackVersion, packVersion)

	// If no deploymentName set default to pack@version
	c.deploymentName = getDeploymentName(c.baseCommand, c.packName, packVersion)
	errorContext.Add(errors.UIContextPrefixDeploymentName, c.deploymentName)

	exitCodes := make([]int, 0)

	client, err := v1.NewClient()
	if err != nil {
		c.ui.ErrorWithContext(err, "failed to initialize client", errorContext.GetAll()...)
		return 255
	}

	packManager := generatePackManager(c.baseCommand, client, repoPath, c.packName)

	// load pack
	r, err := renderPack(packManager, c.baseCommand.ui, errorContext)
	if err != nil {
		return 255
	}

	// Commands that render templates are required to render at least one
	// parent template.
	if r.LenParentRenders() < 1 {
		c.ui.ErrorWithContext(errors.ErrNoTemplatesRendered, "no templates rendered", errorContext.GetAll()...)
		return 1
	}

	jobsApi := client.Jobs()

	for tplName, tpl := range r.ParentRenders() {

		// tplErrorContext forms the basis for error output context as is
		// appended to when new information becomes available.
		tplErrorContext := errorContext.Copy()
		tplErrorContext.Add(errors.UIContextPrefixTemplateName, tplName)

		// get job struct from template
		job, err := parseJob(c.ui, tpl, c.hcl1, tplErrorContext)
		if err != nil {
			exitCodes = append(exitCodes, 255)
			continue
		}

		// Add the jobID to the error context.
		tplErrorContext.Add(errors.UIContextPrefixJobName, job.GetName())

		err = c.checkForConflicts(jobsApi, job)
		if err != nil {
			c.ui.ErrorWithContext(err, "job conflict", tplErrorContext.GetAll()...)
			exitCodes = append(exitCodes, 255)
			continue
		}

		// Set up the options
		planOpts := v1.PlanOpts{}
		if c.diff {
			planOpts.Diff = c.diff
		}
		if c.policyOverride {
			planOpts.PolicyOverride = c.policyOverride
		}

		if jobsApi.IsMultiRegion(job) {
			return c.multiregionPlan(jobsApi, job, &planOpts, c.diff, c.verbose, tplErrorContext)
		}

		// Submit the job
		result, _, err := jobsApi.PlanOpts(newWriteOptsFromJob(job).Ctx(), job, &planOpts)
		if err != nil {
			c.ui.ErrorWithContext(err, "failed to plan job", tplErrorContext.GetAll()...)
			exitCodes = append(exitCodes, 255)
			continue
		}

		exitCode := c.outputPlannedJob(job, result, c.diff, c.verbose)
		exitCodes = append(exitCodes, exitCode)
		// Type conversion is off because OpenAPI doesn't support uints
		// TODO: this still prints nomad job plan command; update with nomad-pack info
		//formatJobModifyIndex(uint64(*result.JobModifyIndex), path, c.ui)
	}

	// Check exit codes for errors (code > 1) and alloc creation/destruction
	allocsCreatedOrDestroyed := false
	for _, exitCode := range exitCodes {
		if exitCode > 1 {
			c.ui.WarningBold("Plan complete with errors")
			return exitCode
		}
		if exitCode == 1 {
			allocsCreatedOrDestroyed = true
		}
	}

	c.ui.Success("Plan succeeded")
	if allocsCreatedOrDestroyed {
		return 1
	}
	return 0
}

func (c *PlanCommand) checkForConflicts(jobsApi *v1.Jobs, job *v1client.Job) error {
	// TODO: Need code review on whether these query opts make sense.
	opts := &v1.QueryOpts{
		Prefix:    *job.ID,
		Region:    *job.Region,
		Namespace: *job.Namespace,
	}
	jobs, _, err := jobsApi.GetJobs(opts.Ctx())
	if err != nil {
		return fmt.Errorf("error checking for conflicts for job %s: %s", *job.ID, err)
	}

	if len(jobs) > 1 {
		return stdErrors.New("job ID matched multiple jobs")
	}

	runningJob, _, err := getJob(jobsApi, *job.ID, opts)
	if err != nil {
		openAPIErr, ok := err.(v1client.GenericOpenAPIError)
		if !ok || string(openAPIErr.Body()) != "job not found" {
			return fmt.Errorf("error checking for conflicts for job %s: %s", *job.ID, err)
		} else {
			return nil
		}
	}

	if runningJob.Meta == nil {
		return fmt.Errorf("job with ID %s running but not managed by pack %s", *runningJob.ID, c.packName)
	}

	meta := *runningJob.Meta
	existingDeploymentName, ok := meta[packDeploymentNameKey]
	if !ok || existingDeploymentName != c.deploymentName {
		return fmt.Errorf("job with ID '%s' running but not managed by pack '%s'", *runningJob.ID, c.packName)
	}

	return nil
}

func (c *PlanCommand) multiregionPlan(jobsApi *v1.Jobs, job *v1client.Job, opts *v1.PlanOpts, diff, verbose bool, errCtx *errors.UIErrorContext) int {
	exitCodes := make([]int, 0)
	plans := map[string]*v1client.JobPlanResponse{}

	// collect all the plans first so that we can report all errors
	for _, region := range *job.Multiregion.Regions {
		regionName := *region.Name
		job.SetRegion(regionName)

		errCtx.Add(errors.UIContextPrefixRegion, regionName)

		err := c.checkForConflicts(jobsApi, job)
		if err != nil {
			c.ui.ErrorWithContext(err, "job conflicts", errCtx.GetAll()...)
			exitCodes = append(exitCodes, 255)
			continue
		}

		// Submit the job for this region
		result, _, err := jobsApi.PlanOpts(newQueryOptsFromJob(job).Ctx(), job, opts)
		if err != nil {
			c.ui.ErrorWithContext(err, "failed to plan regional run", errCtx.GetAll()...)
			exitCodes = append(exitCodes, 255)
			continue
		}
		plans[regionName] = result
	}

	for regionName, resp := range plans {
		c.ui.Info(fmt.Sprintf("Region: %q", regionName))
		regionExitCode := c.outputPlannedJob(job, resp, diff, verbose)
		if regionExitCode > 0 {
			exitCodes = append(exitCodes, regionExitCode)
		}
	}

	// This is kind of weird because we use the same slice
	// to hold errs from operations and errs from output.
	// Nomad plan will actually just fail on operation errors, but
	// we don't want to do that since there may be multiple jobs.
	// tried to adapt this in a reasonable way, but open to suggestions.
	var exitCode int
	for _, exitCode = range exitCodes {
		if exitCode > 1 {
			return exitCode
		}
	}

	return exitCode
}

func (c *PlanCommand) outputPlannedJob(job *v1client.Job, resp *v1client.JobPlanResponse, diff, verbose bool) int {

	// Print the diff if not disabled
	if diff {
		formatJobDiff(*resp.Diff, verbose, c.ui)
	}

	// Print the scheduler dry-run output
	c.ui.Header("Scheduler dry-run:")
	formatDryRun(resp, job, c.ui)

	// Print any warnings if there are any
	if resp.Warnings != nil && *resp.Warnings != "" {
		c.ui.Warning(fmt.Sprintf("\nJob Warnings:\n%s", *resp.Warnings))
	}

	// Print preemptions if there are any
	if resp.Annotations != nil && resp.Annotations.PreemptedAllocs != nil {
		c.addPreemptions(resp)
	}

	return getExitCode(resp)
}

// addPreemptions shows details about preempted allocations
func (c *PlanCommand) addPreemptions(resp *v1client.JobPlanResponse) {
	c.ui.Warning("\nPreemptions:")

	if len(*resp.Annotations.PreemptedAllocs) < preemptionDisplayThreshold {
		var allocs []string
		allocs = append(allocs, "Alloc ID|Job ID|Task Group")
		for _, alloc := range *resp.Annotations.PreemptedAllocs {
			allocs = append(allocs, fmt.Sprintf("%s|%s|%s", *alloc.ID, *alloc.JobID, *alloc.TaskGroup))
		}
		c.ui.Output(formatList(allocs))
		return
	}
	// Display in a summary format if the list is too large
	// Group by job type and job ids
	allocDetails := make(map[string]map[namespaceIdPair]int)
	numJobs := 0
	for _, alloc := range *resp.Annotations.PreemptedAllocs {
		id := namespaceIdPair{*alloc.JobID, *alloc.Namespace}
		countMap := allocDetails[*alloc.JobType]
		if countMap == nil {
			countMap = make(map[namespaceIdPair]int)
		}
		cnt, ok := countMap[id]
		if !ok {
			// First time we are seeing this job, increment counter
			numJobs++
		}
		countMap[id] = cnt + 1
		allocDetails[*alloc.JobType] = countMap
	}

	// Show counts grouped by job ID if its less than a threshold
	var output []string
	if numJobs < preemptionDisplayThreshold {
		output = append(output, "Job ID|Namespace|Job Type|Preemptions")
		for jobType, jobCounts := range allocDetails {
			for jobId, count := range jobCounts {
				output = append(output, fmt.Sprintf("%s|%s|%s|%d", jobId.id, jobId.namespace, jobType, count))
			}
		}
	} else {
		// Show counts grouped by job type
		output = append(output, "Job Type|Preemptions")
		for jobType, jobCounts := range allocDetails {
			total := 0
			for _, count := range jobCounts {
				total += count
			}
			output = append(output, fmt.Sprintf("%s|%d", jobType, total))
		}
	}
	c.ui.Output(formatList(output))
}

func (c *PlanCommand) Flags() *flag.Sets {
	return c.flagSet(flagSetOperation, func(set *flag.Sets) {
		f := set.NewSet("Plan Options")

		f.BoolVar(&flag.BoolVar{
			Name:    "diff",
			Target:  &c.diff,
			Default: true,
			Usage: `Determines whether the diff between the remote job and planned 
                    job is shown. Defaults to true.`,
		})

		f.BoolVar(&flag.BoolVar{
			Name:    "policy-override",
			Target:  &c.policyOverride,
			Default: false,
			Usage:   `Sets the flag to force override any soft mandatory Sentinel policies.`,
		})

		f.BoolVar(&flag.BoolVar{
			Name:    "hcl1",
			Target:  &c.hcl1,
			Default: false,
			Usage:   `If set, HCL1 parser is used for parsing the job spec.`,
		})

		f.BoolVarP(&flag.BoolVarP{
			BoolVar: &flag.BoolVar{
				Name:    "verbose",
				Target:  &c.verbose,
				Default: false,
				Usage:   `Increase diff verbosity.`,
			},
			Shorthand: "v",
		})
	})
}

func (c *PlanCommand) Help() string {
	c.Example = `
	# Plan an example pack with the default deployment name "example@86a9235" (default is <pack-name>@version)
	nomad-pack plan example

	# Plan an example pack with deployment name "dev"
	nomad-pack plan example --name=dev

	# Plan an example pack without showing the diff
	nomad-pack plan example --diff=false
	`

	return formatHelp(`
	Usage: nomad-pack plan <pack-name> [options]

	Determine the effects of submitting a new or updated Nomad Pack

    Plan will return one of the following exit codes:
      * 0: No allocations created or destroyed.
      * 1: Allocations created or destroyed.
      * 255: Error determining plan results.

` + c.GetExample() + c.Flags().Help())
}

// Synopsis satisfies the Synopsis function of the cli.Command interface.
func (c *PlanCommand) Synopsis() string {
	return "Dry-run a pack update to determine its effects"
}

type namespaceIdPair struct {
	id        string
	namespace string
}

// formatJobModifyIndex produces a help string that displays the job modify
// index and how to submit a job with it.
func formatJobModifyIndex(jobModifyIndex uint64, jobName string, ui terminal.UI) {
	help := fmt.Sprintf(jobModifyIndexHelp, jobModifyIndex, jobName)
	ui.Header(fmt.Sprintf("Job Modify Index: %d", jobModifyIndex))
	ui.Info(help)
}

// formatDryRun produces a string explaining the results of the dry run.
func formatDryRun(resp *v1client.JobPlanResponse, job *v1client.Job, ui terminal.UI) {
	var rolling *v1client.Evaluation
	if resp.CreatedEvals != nil {
		for _, eval := range *resp.CreatedEvals {
			if *eval.TriggeredBy == "rolling-update" {
				rolling = &eval
			}
		}
	}

	if resp.FailedTGAllocs == nil {
		ui.Success("- All tasks successfully allocated.")
	} else {
		// Change the output depending on if we are a system job or not
		if job.Type != nil && *job.Type == "system" {
			ui.WarningBold("- WARNING: Failed to place allocations on all nodes.")
		} else {
			ui.WarningBold("- WARNING: Failed to place all allocations.")
		}

		sorted := sortedTaskGroupFromMetrics(*resp.FailedTGAllocs)
		for _, tg := range sorted {
			metrics := (*resp.FailedTGAllocs)[tg]

			noun := "allocation"
			if metrics.CoalescedFailures != nil {
				noun += "s"
			}
			ui.Warning(fmt.Sprintf("%sTaskGroup %q (failed to place %d %s):\n",
				strings.Repeat(" ", 2), tg, *metrics.CoalescedFailures+1, noun))
			formatAllocMetrics(metrics, strings.Repeat(" ", 4), ui)
		}
	}

	if rolling != nil {
		ui.Success(fmt.Sprintf("\n- Rolling update, next evaluation will be in %d.", *rolling.Wait))
	}

	next := resp.NextPeriodicLaunch
	if next != nil && (*next).IsZero() && !IsParameterized(job) {
		loc, err := GetLocation(job.Periodic)
		ui.Output("")
		if err != nil {
			ui.Warning(fmt.Sprintf("- Invalid time zone: %v", err))
		} else {
			now := time.Now().In(loc)
			ui.Success(fmt.Sprintf("- If submitted now, next periodic launch would be at %s (%s from now).",
				formatTime(next), formatTimeDifference(now, *next, time.Second)))
		}
	}
}

func getDiffString(diffType string) (string, string, int) {
	switch diffType {
	case "Added":
		return "+ ", terminal.GreenStyle, 2
	case "Deleted":
		return "- ", terminal.RedStyle, 2
	case "Edited":
		return "+/- ", terminal.LightYellowStyle, 4
	default:
		return "", "", 0
	}
}

func formatJobDiff(job v1client.JobDiff, verbose bool, ui terminal.UI) {
	marker, style, _ := getDiffString(*job.Type)
	ui.AppendToRow(marker, terminal.WithStyle(style))
	ui.AppendToRow("Job: %q\n", *job.ID, terminal.WithStyle(terminal.BoldStyle))

	// Determine the longest markers and fields so that the output can be
	// properly aligned.
	longestField, longestMarker := getLongestPrefixes(job.Fields, job.Objects)
	for _, tg := range *job.TaskGroups {
		if _, _, l := getDiffString(*tg.Type); l > longestMarker {
			longestMarker = l
		}
	}

	// Only show the job's field and object diffs if the job is edited or
	// verbose mode is set.
	if *job.Type == "Edited" || verbose {
		var fields []v1client.FieldDiff
		var objects []v1client.ObjectDiff
		if job.Fields == nil {
			fields = []v1client.FieldDiff{}
		} else {
			fields = *job.Fields
		}
		if job.Objects == nil {
			objects = []v1client.ObjectDiff{}
		} else {
			objects = *job.Objects
		}
		alignedFieldAndObjects(fields, objects, 0, longestField, longestMarker, ui)
		if len(fields) > 0 || len(objects) > 0 {
			ui.AppendToRow("\n")
		}
	}

	// Print the task groups
	for _, tg := range *job.TaskGroups {
		_, _, mLength := getDiffString(*tg.Type)
		kPrefix := longestMarker - mLength
		formatTaskGroupDiff(tg, kPrefix, verbose, ui)
	}
}

// formatTaskGroupDiff produces an annotated diff of a task group. If the
// verbose field is set, the task groups fields and objects are expanded even if
// the full object is an addition or removal. tgPrefix is the number of spaces to prefix
// the output of the task group.
func formatTaskGroupDiff(tg v1client.TaskGroupDiff, tgPrefix int, verbose bool, ui terminal.UI) {
	marker, style, _ := getDiffString(*tg.Type)
	ui.AppendToRow(marker, terminal.WithStyle(style))
	ui.AppendToRow("%s", strings.Repeat("", tgPrefix))
	ui.AppendToRow("Task Group: %q", *tg.Name, terminal.WithStyle(terminal.BoldStyle))

	// Append the updates and colorize them
	if l := len(*tg.Updates); l > 0 {
		order := make([]string, 0, l)
		for updateType := range *tg.Updates {
			order = append(order, updateType)
		}

		sort.Strings(order)
		// Updates enclosed in parens
		ui.AppendToRow(" (")
		for i, updateType := range order {
			// Prepend comma and space for everything after first update
			if i != 0 {
				ui.AppendToRow(", ")
			}

			count := (*tg.Updates)[updateType]
			var color string
			switch updateType {
			case scheduler.UpdateTypeIgnore:
			case scheduler.UpdateTypeCreate:
				color = terminal.GreenStyle
			case scheduler.UpdateTypeDestroy:
				color = terminal.RedStyle
			case scheduler.UpdateTypeMigrate:
				color = terminal.BlueStyle
			case scheduler.UpdateTypeInplaceUpdate:
				color = terminal.CyanStyle
			case scheduler.UpdateTypeDestructiveUpdate:
				color = terminal.YellowStyle
			case scheduler.UpdateTypeCanary:
				color = terminal.LightYellowStyle
			}
			ui.AppendToRow("%d %s", count, updateType, terminal.WithStyle(color))
		}
		ui.AppendToRow(")")
	}
	ui.AppendToRow("\n")

	// Determine the longest field and markers so the output is properly
	// aligned
	longestField, longestMarker := getLongestPrefixes(tg.Fields, tg.Objects)
	for _, task := range *tg.Tasks {
		if _, _, l := getDiffString(*task.Type); l > longestMarker {
			longestMarker = l
		}
	}

	// Only show the task groups's field and object diffs if the group is edited or
	// verbose mode is set.
	subStartPrefix := tgPrefix + 2
	if *tg.Type == "Edited" || verbose {
		// TODO: we check this several times, but the v1client diff isn't always the same type
		// (e.g. job, task, task group). The v1client spec consistently returns pointers so maybe
		// we can add an v1client diff interface with the methods Fields and Objects that returns
		// those pointers so we can one nil check func that we call?
		var fields []v1client.FieldDiff
		var objects []v1client.ObjectDiff
		if tg.Fields == nil {
			fields = []v1client.FieldDiff{}
		} else {
			fields = *tg.Fields
		}
		if tg.Objects == nil {
			objects = []v1client.ObjectDiff{}
		} else {
			objects = *tg.Objects
		}
		alignedFieldAndObjects(fields, objects, subStartPrefix, longestField, longestMarker, ui)
		if len(fields) > 0 || len(objects) > 0 {
			ui.AppendToRow("\n")
		}
	}

	// Output the tasks
	for _, task := range *tg.Tasks {
		_, _, mLength := getDiffString(*task.Type)
		prefix := longestMarker - mLength
		formatTaskDiff(task, subStartPrefix, prefix, verbose, ui)
	}
}

// formatTaskDiff produces an annotated diff of a task. If the verbose field is
// set, the tasks fields and objects are expanded even if the full object is an
// addition or removal. startPrefix is the number of spaces to prefix the output of
// the task and taskPrefix is the number of spaces to put between the marker and
// task name output.
func formatTaskDiff(task v1client.TaskDiff, startPrefix, taskPrefix int, verbose bool, ui terminal.UI) {
	marker, style, _ := getDiffString(*task.Type)
	ui.AppendToRow("%s%s%s", strings.Repeat(" ", startPrefix), marker, strings.Repeat(" ", taskPrefix), terminal.WithStyle(style))
	ui.AppendToRow("Task: %q", *task.Name, terminal.WithStyle(terminal.BoldStyle))

	if task.Annotations != nil {
		printColorAnnotations(*task.Annotations, ui)
	}

	if *task.Type == "None" {
		return
	} else if (*task.Type == "Deleted" || *task.Type == "Added") && !verbose {
		// Exit early if the job was not edited and it isn't verbose output
		return
	}

	ui.AppendToRow("\n")
	subStartPrefix := startPrefix + 2
	longestField, longestMarker := getLongestPrefixes(task.Fields, task.Objects)

	var fields []v1client.FieldDiff
	var objects []v1client.ObjectDiff
	if task.Fields == nil {
		fields = []v1client.FieldDiff{}
	} else {
		fields = *task.Fields
	}
	if task.Objects == nil {
		objects = []v1client.ObjectDiff{}
	} else {
		objects = *task.Objects
	}
	alignedFieldAndObjects(fields, objects, subStartPrefix, longestField, longestMarker, ui)
}

func getLongestPrefixes(fields *[]v1client.FieldDiff, objects *[]v1client.ObjectDiff) (longestField, longestMarker int) {
	if fields != nil {
		for _, field := range *fields {
			if l := len(*field.Name); l > longestField {
				longestField = l
			}
			if _, _, l := getDiffString(*field.Type); l > longestMarker {
				longestMarker = l
			}
		}
	}
	if objects != nil {
		for _, obj := range *objects {
			if _, _, l := getDiffString(*obj.Type); l > longestMarker {
				longestMarker = l
			}
		}
	}
	return longestField, longestMarker
}

func alignedFieldAndObjects(fields []v1client.FieldDiff, objects []v1client.ObjectDiff,
	startPrefix, longestField, longestMarker int, ui terminal.UI) {

	numFields := len(fields)
	numObjects := len(objects)
	haveObjects := numObjects != 0
	for i, field := range fields {
		_, _, mLength := getDiffString(*field.Type)
		kPrefix := longestMarker - mLength
		vPrefix := longestField - len(*field.Name)
		formatFieldDiff(&field, startPrefix, kPrefix, vPrefix, ui)

		// Avoid a dangling new line
		if i+1 != numFields || haveObjects {
			ui.AppendToRow("\n")
		}
	}

	for i, object := range objects {
		_, _, mLength := getDiffString(*object.Type)
		kPrefix := longestMarker - mLength
		formatObjectDiff(&object, startPrefix, kPrefix, ui)

		// Avoid a dangling new line
		if i+1 != numObjects {
			ui.AppendToRow("\n")
		}
	}
}

func formatObjectDiff(diff *v1client.ObjectDiff, startPrefix, keyPrefix int, ui terminal.UI) {
	start := strings.Repeat(" ", startPrefix)
	marker, style, markerLen := getDiffString(*diff.Type)
	ui.AppendToRow("%s%s", start, marker, terminal.WithStyle(style))
	ui.AppendToRow("%s%s {\n", strings.Repeat(" ", keyPrefix), *diff.Name)

	// Determine the length of the longest name and longest diff marker to
	// properly align names and values
	longestField, longestMarker := getLongestPrefixes(diff.Fields, diff.Objects)
	subStartPrefix := startPrefix + keyPrefix + 2

	// Nil pointer check
	var fields []v1client.FieldDiff
	var objects []v1client.ObjectDiff
	if diff.Fields == nil {
		fields = []v1client.FieldDiff{}
	} else {
		fields = *diff.Fields
	}
	if diff.Objects == nil {
		objects = []v1client.ObjectDiff{}
	} else {
		objects = *diff.Objects
	}

	alignedFieldAndObjects(fields, objects, subStartPrefix, longestField, longestMarker, ui)

	endPrefix := strings.Repeat(" ", startPrefix+markerLen+keyPrefix)
	ui.AppendToRow("\n%s}", endPrefix)
}

func formatFieldDiff(diff *v1client.FieldDiff, startPrefix, keyPrefix, valuePrefix int, ui terminal.UI) {
	marker, style, _ := getDiffString(*diff.Type)
	ui.AppendToRow("%s%s", strings.Repeat(" ", startPrefix), marker, terminal.WithStyle(style))
	ui.AppendToRow("%s%s: %s", strings.Repeat(" ", keyPrefix), *diff.Name, strings.Repeat(" ", valuePrefix))

	switch *diff.Type {
	case "Added":
		ui.AppendToRow("%q", *diff.New)
	case "Deleted":
		ui.AppendToRow("%q", *diff.Old)
	case "Edited":
		ui.AppendToRow("%q => %q", *diff.Old, *diff.New)
	default:
		ui.AppendToRow("%q", *diff.New)
	}

	// Color the annotations where possible
	if diff.Annotations != nil {
		printColorAnnotations(*diff.Annotations, ui)
	}
}

// getExitCode returns 0:
// * 0: No allocations created or destroyed.
// * 1: Allocations created or destroyed.
func getExitCode(resp *v1client.JobPlanResponse) int {
	// Check for changes
	for _, d := range *resp.Annotations.DesiredTGUpdates {
		if *d.Stop+*d.Place+*d.Migrate+*d.DestructiveUpdate+*d.Canary > 0 {
			return 1
		}
	}

	return 0
}

func printColorAnnotations(annotations []string, ui terminal.UI) {
	l := len(annotations)
	if l == 0 {
		return
	}

	// Output in parens
	ui.AppendToRow(" (")
	for i, annotation := range annotations {
		// Prepend comma if > 1 annotation
		if i != 0 {
			ui.AppendToRow(", ")
		}
		var color string
		switch annotation {
		case "forces create":
			color = terminal.GreenStyle
		case "forces destroy":
			color = terminal.RedStyle
		case "forces in-place update":
			color = terminal.CyanStyle
		case "forces create/destroy update":
			color = terminal.YellowStyle
		default:
			color = terminal.DefaultStyle
		}
		ui.AppendToRow(annotation, terminal.WithStyle(color))
	}
	ui.AppendToRow(")")
}

func sortedTaskGroupFromMetrics(groups map[string]v1client.AllocationMetric) []string {
	tgs := make([]string, 0, len(groups))
	for tg := range groups {
		tgs = append(tgs, tg)
	}
	sort.Strings(tgs)
	return tgs
}

// TODO: when we turn these into components, we can probably replace prefix (if empty) with glint padding/margin
func formatAllocMetrics(metrics v1client.AllocationMetric, prefix string, ui terminal.UI) {
	// Print a helpful message if we have an eligibility problem
	if metrics.NodesEvaluated == nil {
		ui.Warning(fmt.Sprintf("%s* No nodes were eligible for evaluation", prefix))
	}

	// Print a helpful message if the user has asked for a DC that has no
	// available nodes.
	if metrics.NodesAvailable != nil {
		for dc, available := range *metrics.NodesAvailable {
			if available == 0 {
				ui.Warning(fmt.Sprintf("%s* No nodes are available in datacenter %q", prefix, dc))
			}
		}
	}

	// Print filter info
	if metrics.ClassFiltered != nil {
		for class, num := range *metrics.ClassFiltered {
			ui.Warning(fmt.Sprintf("%s* Class %q: %d nodes excluded by filter", prefix, class, num))
		}
	}
	if metrics.ConstraintFiltered != nil {
		for cs, num := range *metrics.ConstraintFiltered {
			ui.Warning(fmt.Sprintf("%s* Constraint %q: %d nodes excluded by filter", prefix, cs, num))
		}
	}

	// Print exhaustion info
	if metrics.NodesExhausted != nil {
		ui.Warning(fmt.Sprintf("%s* Resources exhausted on %d nodes", prefix, *metrics.NodesExhausted))
	}
	if metrics.ClassExhausted != nil {
		for class, num := range *metrics.ClassExhausted {
			ui.Warning(fmt.Sprintf("%s* Class %q exhausted on %d nodes", prefix, class, num))
		}
	}
	if metrics.DimensionExhausted != nil {
		for dim, num := range *metrics.DimensionExhausted {
			ui.Warning(fmt.Sprintf("%s* Dimension %q exhausted on %d nodes", prefix, dim, num))
		}
	}

	// Print quota info
	if metrics.QuotaExhausted != nil {
		for _, dim := range *metrics.QuotaExhausted {
			ui.Warning(fmt.Sprintf("%s* Quota limit hit %q", prefix, dim))
		}
	}
}

func IsParameterized(j *v1client.Job) bool {
	return j.ParameterizedJob != nil && !*j.Dispatched
}

func GetLocation(p *v1client.PeriodicConfig) (*time.Location, error) {
	if p.TimeZone == nil || *p.TimeZone == "" {
		return time.UTC, nil
	}

	return time.LoadLocation(*p.TimeZone)
}
