package cli

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// formatList takes a set of strings and formats them into properly
// aligned output, replacing any blank fields with a placeholder
// for awk-ability.
func TestFormatList(t *testing.T) {
	testcases := []struct {
		name   string
		in     []string
		expect string
	}{
		{
			name:   "simple",
			in:     []string{"a", "b", "c"},
			expect: "a\nb\nc",
		},
	}
	for _, tc := range testcases {
		t.Run(tc.name, func(t *testing.T) {
			out := formatList(tc.in)
			require.Equal(t, tc.expect, out)
		})
	}
}

// formatTime formats the time to string based on RFC822
func TestFormatTime(t *testing.T) {
	testcases := []struct {
		name   string
		in     time.Time
		expect string
	}{
		{
			name:   "simple",
			in:     time.Date(2020, 12, 25, 0, 0, 1, 0, time.Now().UTC().Location()),
			expect: "2020-12-25T00:00:01Z",
		},
	}
	for _, tc := range testcases {
		t.Run(tc.name, func(t *testing.T) {
			out := formatTime(&tc.in)
			require.Equal(t, tc.expect, out)
		})
	}
}

// formatTimeDifference takes two times and determines their duration difference
// truncating to a passed unit.
// E.g. formatTimeDifference(first=1m22s33ms, second=1m28s55ms, time.Second) -> 6s
func TestFormatTimeDifference(t *testing.T) {
	testcases := []struct {
		name   string
		start  time.Time
		end    time.Time
		expect string
	}{
		{
			name:   "simple",
			start:  time.Date(2020, 12, 25, 0, 0, 1, 0, time.Now().UTC().Location()),
			end:    time.Date(2020, 12, 25, 0, 0, 11, 0, time.Now().UTC().Location()),
			expect: "10s",
		},
	}
	for _, tc := range testcases {
		t.Run(tc.name, func(t *testing.T) {
			out := formatTimeDifference(tc.start, tc.end, time.Second)
			require.Equal(t, tc.expect, out)
		})
	}
}
