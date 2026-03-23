// Copyright IBM Corp. 2023, 2026
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

func Test_FormatKV(t *testing.T) {
	testCases := []struct {
		name     string
		input    []string
		contains []string
	}{
		{
			name:     "empty",
			input:    []string{},
			contains: []string{},
		},
		{
			name:     "single key value",
			input:    []string{"ID|abc123"},
			contains: []string{"ID", "=", "abc123"},
		},
		{
			name:     "multiple key values",
			input:    []string{"ID|abc123", "Status|running", "Version|1"},
			contains: []string{"ID", "Status", "Version", "abc123", "running", "1"},
		},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			out := formatKV(tc.input)
			for _, expected := range tc.contains {
				must.True(t, len(out) == 0 || contains(out, expected),
					must.Sprintf("expected %q to contain %q", out, expected))
			}
		})
	}
}

func Test_FormatUnixNanoTime(t *testing.T) {
	// Test with a known timestamp
	originalTime := time.Date(2026, 2, 25, 12, 30, 45, 0, time.UTC)
	nano := originalTime.UnixNano()
	result := formatUnixNanoTime(nano)

	// Parse the result back and compare the underlying time
	parsedTime, err := time.Parse("2006-01-02T15:04:05Z07:00", result)
	must.NoError(t, err)
	must.True(t, originalTime.Equal(parsedTime),
		must.Sprintf("expected %v to equal %v", parsedTime, originalTime))

	// Test with zero time
	result = formatUnixNanoTime(0)
	must.Eq(t, "", result)
}

func Test_PrettyTimeDiff(t *testing.T) {
	now := time.Now()

	testCases := []struct {
		name     string
		first    time.Time
		second   time.Time
		expected string
	}{
		{
			name:     "zero time",
			first:    time.Time{},
			second:   now,
			expected: "N/A",
		},
		{
			name:     "unix epoch",
			first:    time.Unix(0, 0),
			second:   now,
			expected: "N/A",
		},
		{
			name:     "seconds ago",
			first:    now.Add(-30 * time.Second),
			second:   now,
			expected: "30s ago",
		},
		{
			name:     "minutes and seconds ago",
			first:    now.Add(-5*time.Minute - 30*time.Second),
			second:   now,
			expected: "5m30s ago",
		},
		{
			name:     "hours and minutes ago",
			first:    now.Add(-2*time.Hour - 15*time.Minute),
			second:   now,
			expected: "2h15m ago",
		},
		{
			name:     "days and hours ago",
			first:    now.Add(-3*24*time.Hour - 5*time.Hour),
			second:   now,
			expected: "3d5h ago",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := prettyTimeDiff(tc.first, tc.second)
			must.Eq(t, tc.expected, result)
		})
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(substr) == 0 ||
		(len(s) > 0 && len(substr) > 0 && findSubstring(s, substr)))
}

func findSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
