// Copyright IBM Corp. 2023, 2026
// SPDX-License-Identifier: MPL-2.0

package cli

import (
	"fmt"
	"strings"
	"time"

	"github.com/ryanuber/columnize"
)

// formatList takes a set of strings and formats them into properly
// aligned output, replacing any blank fields with a placeholder
// for awk-ability.
func formatList(in []string) string {
	columnConf := columnize.DefaultConfig()
	columnConf.Empty = "<none>"
	return columnize.Format(in, columnConf)
}

// formatKV takes a set of key-value strings and formats them into properly
// aligned output using " = " as a delimiter.
func formatKV(in []string) string {
	columnConf := columnize.DefaultConfig()
	columnConf.Empty = "<none>"
	columnConf.Glue = " = "
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

func formatSHA1Reference(in string) string {
	// a SHA1 hash is 20 bytes written as a hexadecimal string (40 chars)
	if len(in) != 40 && len(strings.Trim(strings.ToLower(in), "0123456789abcdef")) != 0 {
		// if it can't be a sha1, return it unchanged
		return in
	}
	l := 8
	if len(in) < l {
		l = len(in)
	}
	return in[:l]
}

// formatUnixNanoTime formats a unix nano timestamp to a time string
func formatUnixNanoTime(nano int64) string {
	t := time.Unix(0, nano)
	return formatTime(t)
}

// prettyTimeDiff formats the time difference between two times in a human-readable format
func prettyTimeDiff(first, second time.Time) string {
	// Handle zero times
	if first.IsZero() || first.Unix() == 0 {
		return "N/A"
	}

	d := second.Sub(first)
	switch {
	case d < time.Minute:
		return fmt.Sprintf("%ds ago", int(d.Seconds()))
	case d < time.Hour:
		return fmt.Sprintf("%dm%ds ago", int(d.Minutes()), int(d.Seconds())%60)
	case d < 24*time.Hour:
		return fmt.Sprintf("%dh%dm ago", int(d.Hours()), int(d.Minutes())%60)
	default:
		days := int(d.Hours() / 24)
		hours := int(d.Hours()) % 24
		return fmt.Sprintf("%dd%dh ago", days, hours)
	}
}
