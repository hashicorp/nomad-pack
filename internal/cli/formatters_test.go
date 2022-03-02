package cli

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
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
			require.Equal(t, tC.expected, out)
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
			require.Equal(t, tC.expected, formatTime(tC.input))
		})
	}
}

// formatTimeDifference takes two times and determines their duration difference
// truncating to a passed unit.
// E.g. formatTimeDifference(first=1m22s33ms, second=1m28s55ms, time.Second) -> 6s
func Test_FormatTimeDifference(t *testing.T) {
	first := time.Now()
	second := first.Add(6*time.Second + 22*time.Millisecond)
	require.Equal(t, "6s", formatTimeDifference(first, second, time.Second))
}
