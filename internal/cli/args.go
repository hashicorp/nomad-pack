// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package cli

import (
	"fmt"
)

type ValidationErr struct {
	exp int
	got int
	cmp int
}

func tooFewArgs(expected, got int) ValidationErr {
	return ValidationErr{exp: expected, got: got, cmp: -1}
}
func tooManyArgs(expected, got int) ValidationErr {
	return ValidationErr{exp: expected, got: got, cmp: 1}
}
func wrongNumArgs(expected, got int) ValidationErr {
	return ValidationErr{exp: expected, got: got, cmp: 0}
}

func (v ValidationErr) Error() string {
	// special case for no arguments
	if v.exp == 0 {
		return "this command takes no arguments"
	}

	var amt string
	switch {
	case v.cmp < 0: // less
		amt = "at least"
	case v.cmp > 0: // greater
		amt = "at most"
	default:
		amt = "exactly"
	}

	argStr := "argument"
	if v.exp > 1 {
		argStr = "arguments"
	}

	return fmt.Sprintf("this command requires %s %d %s, got %d", amt, v.exp, argStr, v.got)
}

type ValidationFn func(c *baseCommand, args []string) error

// Returns an error if any args provided
func NoArgs(c *baseCommand, args []string) error {
	if len(args) != 0 {
		return wrongNumArgs(0, len(args))
	}
	return nil
}

// Returns an error if fewer than N args provided
func MinimumNArgs(n int) ValidationFn {
	return func(c *baseCommand, args []string) error {
		if len(args) < n {
			return tooFewArgs(n, len(args))
		}
		return nil
	}
}

// Returns an error if more than N args provided
func MaximumNArgs(n int) ValidationFn {
	return func(c *baseCommand, args []string) error {
		if len(args) > n {
			return tooManyArgs(n, len(args))
		}
		return nil
	}
}

// Returns an error if exactly N args aren't provided
func ExactArgs(n int) ValidationFn {
	return func(c *baseCommand, args []string) error {
		if len(args) != n {
			return wrongNumArgs(n, len(args))
		}
		return nil
	}
}
