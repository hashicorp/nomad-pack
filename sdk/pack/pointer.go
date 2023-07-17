// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package pack

// pointerOf returns a pointer to a.
func pointerOf[A any](a A) *A {
	return &a
}
