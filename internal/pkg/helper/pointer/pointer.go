// Copyright IBM Corp. 2021, 2025
// SPDX-License-Identifier: MPL-2.0

package pointer

// Of returns a pointer to a.
func Of[A any](a A) *A {
	return &a
}
