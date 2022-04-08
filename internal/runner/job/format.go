package job

import (
	"fmt"
	"sort"
	"strings"
	"time"

	v1client "github.com/hashicorp/nomad-openapi/clients/go/v1"
	"github.com/hashicorp/nomad-pack/terminal"
	"github.com/ryanuber/columnize"
)

const (
	preemptionDisplayThreshold = 10
)

// updateType* is copied from github.com/hashicorp/nomad/scheduler, so we don't
// need import it.
const (
	updateTypeIgnore            = "ignore"
	updateTypeCreate            = "create"
	updateTypeDestroy           = "destroy"
	updateTypeMigrate           = "migrate"
	updateTypeCanary            = "canary"
	updateTypeInplaceUpdate     = "in-place update"
	updateTypeDestructiveUpdate = "create/destroy update"
)

type namespaceIdPair struct {
	id        string
	namespace string
}

// formatList takes a set of strings and formats them into properly
// aligned output, replacing any blank fields with a placeholder
// for awk-ability.
func formatList(in []string) string {
	columnConf := columnize.DefaultConfig()
	columnConf.Empty = "<none>"
	return columnize.Format(in, columnConf)
}

// formatTime formats the time to string based on RFC822
func formatTime(t time.Time) string {
	if t.Unix() < 1 {
		// It's more confusing to display the UNIX epoch or a zero value than nothing
		return ""
	}
	// Return ISO_8601 time format GH-3806
	return t.Format("2006-01-02T15:04:05Z07:00")
}

// formatTimeDifference takes two times and determines their duration difference
// truncating to a passed unit.
// E.g. formatTimeDifference(first=1m22s33ms, second=1m28s55ms, time.Second) -> 6s
func formatTimeDifference(first, second time.Time, d time.Duration) string {
	return second.Truncate(d).Sub(first.Truncate(d)).String()
}

//
func formatJobDiff(job v1client.JobDiff, verbose bool, ui terminal.UI) {
	marker, style, _ := getDiffString(job.GetType())
	ui.AppendToRow(marker, terminal.WithStyle(style))
	ui.AppendToRow("Job: %q\n", job.GetID(), terminal.WithStyle(terminal.BoldStyle))

	// Determine the longest markers and fields so that the output can be
	// properly aligned.
	longestField, longestMarker := getLongestPrefixes(job.GetFields(), job.GetObjects())
	for _, tg := range job.GetTaskGroups() {
		if _, _, l := getDiffString(tg.GetType()); l > longestMarker {
			longestMarker = l
		}
	}

	// Only show the job's field and object diffs if the job is edited or
	// verbose mode is set.
	if job.GetType() == "Edited" || verbose {
		fields := job.GetFields()
		objects := job.GetObjects()
		alignedFieldAndObjects(fields, objects, 0, longestField, longestMarker, ui)
		if len(fields) > 0 || len(objects) > 0 {
			ui.AppendToRow("\n")
		}
	}

	// Print the task groups
	for _, tg := range job.GetTaskGroups() {
		_, _, mLength := getDiffString(tg.GetType())
		kPrefix := longestMarker - mLength
		formatTaskGroupDiff(tg, kPrefix, verbose, ui)
	}
}

// formatTaskGroupDiff produces an annotated diff of a task group. If the
// verbose field is set, the task groups fields and objects are expanded even if
// the full object is an addition or removal. tgPrefix is the number of spaces to prefix
// the output of the task group.
func formatTaskGroupDiff(tg v1client.TaskGroupDiff, tgPrefix int, verbose bool, ui terminal.UI) {
	marker, style, _ := getDiffString(tg.GetType())
	ui.AppendToRow(marker, terminal.WithStyle(style))
	ui.AppendToRow("%s", strings.Repeat("", tgPrefix))
	ui.AppendToRow("Task Group: %q", tg.GetName(), terminal.WithStyle(terminal.BoldStyle))

	// Append the updates and colorize them
	if l := len(tg.GetUpdates()); l > 0 {
		order := make([]string, 0, l)
		for updateType := range tg.GetUpdates() {
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

			count := (tg.GetUpdates())[updateType]
			var color string
			switch updateType {
			case updateTypeIgnore:
			case updateTypeCreate:
				color = terminal.GreenStyle
			case updateTypeDestroy:
				color = terminal.RedStyle
			case updateTypeMigrate:
				color = terminal.BlueStyle
			case updateTypeInplaceUpdate:
				color = terminal.CyanStyle
			case updateTypeDestructiveUpdate:
				color = terminal.YellowStyle
			case updateTypeCanary:
				color = terminal.LightYellowStyle
			}
			ui.AppendToRow("%d %s", count, updateType, terminal.WithStyle(color))
		}
		ui.AppendToRow(")")
	}
	ui.AppendToRow("\n")

	// Determine the longest field and markers so the output is properly
	// aligned
	longestField, longestMarker := getLongestPrefixes(tg.GetFields(), tg.GetObjects())
	for _, task := range tg.GetTasks() {
		if _, _, l := getDiffString(task.GetType()); l > longestMarker {
			longestMarker = l
		}
	}

	// Only show the task groups's field and object diffs if the group is edited or
	// verbose mode is set.
	subStartPrefix := tgPrefix + 2
	if tg.GetType() == "Edited" || verbose {
		// TODO: we check this several times, but the v1client diff isn't always the same type
		// (e.g. job, task, task group). The v1client spec consistently returns pointers so maybe
		// we can add an v1client diff interface with the methods Fields and Objects that returns
		// those pointers so we can one nil check func that we call?
		fields := tg.GetFields()
		objects := tg.GetObjects()

		alignedFieldAndObjects(fields, objects, subStartPrefix, longestField, longestMarker, ui)
		if len(fields) > 0 || len(objects) > 0 {
			ui.AppendToRow("\n")
		}
	}

	// Output the tasks
	for _, task := range tg.GetTasks() {
		_, _, mLength := getDiffString(task.GetType())
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
	marker, style, _ := getDiffString(task.GetType())
	ui.AppendToRow("%s%s%s", strings.Repeat(" ", startPrefix), marker, strings.Repeat(" ", taskPrefix), terminal.WithStyle(style))
	ui.AppendToRow("Task: %q", task.GetName(), terminal.WithStyle(terminal.BoldStyle))

	if task.HasAnnotations() {
		printColorAnnotations(task.GetAnnotations(), ui)
	}

	if task.GetType() == "None" {
		return
	} else if (task.GetType() == "Deleted" || task.GetType() == "Added") && !verbose {
		// Exit early if the job was not edited and it isn't verbose output
		return
	}

	ui.AppendToRow("\n")
	subStartPrefix := startPrefix + 2
	longestField, longestMarker := getLongestPrefixes(task.GetFields(), task.GetObjects())

	fields := task.GetFields()
	objects := task.GetObjects()

	alignedFieldAndObjects(fields, objects, subStartPrefix, longestField, longestMarker, ui)
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

func getLongestPrefixes(fields []v1client.FieldDiff, objects []v1client.ObjectDiff) (longestField, longestMarker int) {
	for _, field := range fields {
		if l := len(field.GetName()); l > longestField {
			longestField = l
		}
		if _, _, l := getDiffString(field.GetType()); l > longestMarker {
			longestMarker = l
		}
	}
	for _, obj := range objects {
		if _, _, l := getDiffString(obj.GetType()); l > longestMarker {
			longestMarker = l
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
		_, _, mLength := getDiffString(field.GetType())
		kPrefix := longestMarker - mLength
		vPrefix := longestField - len(field.GetName())
		formatFieldDiff(field, startPrefix, kPrefix, vPrefix, ui)

		// Avoid a dangling new line
		if i+1 != numFields || haveObjects {
			ui.AppendToRow("\n")
		}
	}

	for i, object := range objects {
		_, _, mLength := getDiffString(object.GetType())
		kPrefix := longestMarker - mLength
		formatObjectDiff(object, startPrefix, kPrefix, ui)

		// Avoid a dangling new line
		if i+1 != numObjects {
			ui.AppendToRow("\n")
		}
	}
}

func formatFieldDiff(diff v1client.FieldDiff, startPrefix, keyPrefix, valuePrefix int, ui terminal.UI) {
	marker, style, _ := getDiffString(diff.GetType())
	ui.AppendToRow("%s%s", strings.Repeat(" ", startPrefix), marker, terminal.WithStyle(style))
	ui.AppendToRow("%s%s: %s", strings.Repeat(" ", keyPrefix), diff.GetName(), strings.Repeat(" ", valuePrefix))

	switch diff.GetType() {
	case "Added":
		ui.AppendToRow("%q", diff.GetNew())
	case "Deleted":
		ui.AppendToRow("%q", diff.GetOld())
	case "Edited":
		ui.AppendToRow("%q => %q", diff.GetOld(), diff.GetNew())
	default:
		ui.AppendToRow("%q", diff.GetNew())
	}

	// Color the annotations where possible
	if diff.HasAnnotations() {
		printColorAnnotations(diff.GetAnnotations(), ui)
	}
}

func formatObjectDiff(diff v1client.ObjectDiff, startPrefix, keyPrefix int, ui terminal.UI) {
	start := strings.Repeat(" ", startPrefix)
	marker, style, markerLen := getDiffString(diff.GetType())
	ui.AppendToRow("%s%s", start, marker, terminal.WithStyle(style))
	ui.AppendToRow("%s%s {\n", strings.Repeat(" ", keyPrefix), diff.GetName())

	// Determine the length of the longest name and longest diff marker to
	// properly align names and values
	longestField, longestMarker := getLongestPrefixes(diff.GetFields(), diff.GetObjects())
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
		if job.GetType() == "system" {
			ui.WarningBold("- WARNING: Failed to place allocations on all nodes.")
		} else {
			ui.WarningBold("- WARNING: Failed to place all allocations.")
		}

		sorted := sortedTaskGroupFromMetrics(resp.GetFailedTGAllocs())
		for _, tg := range sorted {
			metrics := (resp.GetFailedTGAllocs())[tg]

			noun := "allocation"
			if metrics.GetCoalescedFailures() > 1 {
				noun += "s"
			}
			ui.Warning(fmt.Sprintf("%sTaskGroup %q (failed to place %d %s):\n",
				strings.Repeat(" ", 2), tg, metrics.GetCoalescedFailures(), noun))
			formatAllocMetrics(metrics, strings.Repeat(" ", 4), ui)
		}
	}

	if rolling != nil {
		ui.Success(fmt.Sprintf("\n- Rolling update, next evaluation will be in %d.", *rolling.Wait))
	}

	next := resp.GetNextPeriodicLaunch()
	if resp.HasNextPeriodicLaunch() && next.IsZero() && !isParameterized(job) {
		loc, err := GetLocation(job.Periodic)
		ui.Output("")
		if err != nil {
			ui.Warning(fmt.Sprintf("- Invalid time zone: %v", err))
		} else {
			now := time.Now().In(loc)
			ui.Success(fmt.Sprintf("- If submitted now, next periodic launch would be at %s (%s from now).",
				formatTime(next), formatTimeDifference(now, next, time.Second)))
		}
	}
}

func sortedTaskGroupFromMetrics(groups map[string]v1client.AllocationMetric) []string {
	tgs := make([]string, 0, len(groups))
	for tg := range groups {
		tgs = append(tgs, tg)
	}
	sort.Strings(tgs)
	return tgs
}

// TODO: when we turn these into components, we can probably replace prefix
//  (if empty) with glint padding/margin.
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

func isParameterized(j *v1client.Job) bool { return j.ParameterizedJob != nil && !*j.Dispatched }

func GetLocation(p *v1client.PeriodicConfig) (*time.Location, error) {
	if p.TimeZone == nil || *p.TimeZone == "" {
		return time.UTC, nil
	}

	return time.LoadLocation(*p.TimeZone)
}

// formatPreemptions shows details about preempted allocations
func formatPreemptions(ui terminal.UI, resp *v1client.JobPlanResponse) {

	ui.Warning("\nPreemptions:")

	if len(*resp.Annotations.PreemptedAllocs) < preemptionDisplayThreshold {
		var allocs []string
		allocs = append(allocs, "Alloc ID|Job ID|Task Group")
		for _, alloc := range *resp.Annotations.PreemptedAllocs {
			allocs = append(allocs, fmt.Sprintf("%s|%s|%s", *alloc.ID, *alloc.JobID, *alloc.TaskGroup))
		}
		ui.Output(formatList(allocs))
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
	ui.Output(formatList(output))
}
