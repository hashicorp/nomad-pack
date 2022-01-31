package cli

import (
	"errors"
	"fmt"
)

type ValidationFn func(c *baseCommand, args []string) error

// Returns an error if any args provided
func NoArgs(c *baseCommand, args []string) error {
	if len(args) != 0 {
		return errors.New("this command takes no args")
	}
	return nil
}

// Returns an error if fewer than N args provided
func MinimumNArgs(n int) ValidationFn {
	return func(c *baseCommand, args []string) error {
		if len(args) < n {
			return fmt.Errorf("this command requires at least %d arg(s), received %d", n, len(args))
		}
		return nil
	}
}

// Returns an error if more than N args provided
func MaximumNArgs(n int) ValidationFn {
	return func(c *baseCommand, args []string) error {
		if len(args) > n {
			return fmt.Errorf("this command accepts at most %d arg(s), received %d", n, len(args))
		}
		return nil
	}
}

// Returns an error if exactly N args aren't provided
func ExactArgs(n int) ValidationFn {
	return func(c *baseCommand, args []string) error {
		if len(args) != n {
			return fmt.Errorf("this command requires exactly %d args(s), received %d", n, len(args))
		}
		return nil
	}
}
