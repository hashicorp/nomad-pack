// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package helper

import (
	"testing"

	"github.com/shoenig/test/must"
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
		must.Eq(t, tc.exp, result)
	}
}
