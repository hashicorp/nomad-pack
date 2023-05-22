// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package helper

// BoolToPtr returns a pointer to the input boolean value.
func BoolToPtr(i bool) *bool { return &i }

// StringToPtr returns the pointer to the input string.
func StringToPtr(str string) *string { return &str }

// MapToPtr returns the pointer to the input map.
func MapToPtr(m map[string]string) *map[string]string { return &m }
