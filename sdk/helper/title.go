// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package helper

import (
	"golang.org/x/text/cases"
	"golang.org/x/text/language"
)

var (
	titleFmt = cases.Title(language.AmericanEnglish)
)

// Title returns the American English title format of s.
func Title(s string) string {
	return titleFmt.String(s)
}
