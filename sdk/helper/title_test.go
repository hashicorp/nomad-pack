package helper

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestTitle(t *testing.T) {
	cases := []struct {
		s   string
		exp string
	}{
		{s: "hello", exp: "Hello"},
		{s: "hello world", exp: "Hello World"},
	}

	for _, tc := range cases {
		result := Title(tc.s)
		require.Equal(t, tc.exp, result)
	}
}
