// Copyright IBM Corp. 2021, 2025
// SPDX-License-Identifier: MPL-2.0

package cli

import (
	"testing"
	"time"

	"github.com/shoenig/test/must"
)

func Test_FormatList(t *testing.T) {
	testCases := []struct {
		name     string
		input    []string
		expected string
	}{
		{
			name:     "empty",
			input:    make([]string, 0),
			expected: "",
		},
		{
			name:     "abc",
			input:    []string{"a", "b", "c"},
			expected: "a\nb\nc",
		},
	}
	for _, tC := range testCases {
		t.Run(tC.name, func(t *testing.T) {
			out := formatList(tC.input)
			must.Eq(t, tC.expected, out)
		})
	}
}

func Test_FormatTime(t *testing.T) {
	testCases := []struct {
		name     string
		input    time.Time
		expected string
	}{
		{
			name:     "date",
			input:    time.Date(2000, 1, 1, 12, 34, 56, 00, time.UTC),
			expected: "2000-01-01T12:34:56Z",
		},
	}
	for _, tC := range testCases {
		t.Run(tC.name, func(t *testing.T) {
			must.Eq(t, tC.expected, formatTime(tC.input))
		})
	}
}

// formatTimeDifference takes two times and determines their duration difference
// truncating to a passed unit.
// E.g. formatTimeDifference(first=1m22s33ms, second=1m28s55ms, time.Second) -> 6s
func Test_FormatTimeDifference(t *testing.T) {
	first := time.Now().Truncate(time.Second)
	second := first.Add(6*time.Second + 22*time.Millisecond)
	must.Eq(t, "6s", formatTimeDifference(first, second, time.Second))
}
