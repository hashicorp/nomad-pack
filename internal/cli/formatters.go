// Copyright IBM Corp. 2021, 2025
// SPDX-License-Identifier: MPL-2.0

package cli

import (
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
