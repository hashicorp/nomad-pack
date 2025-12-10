// Copyright IBM Corp. 2021, 2025
// SPDX-License-Identifier: MPL-2.0

package creator

import (
	"embed"
	"text/template"
)

// content holds the go-templates used by creator while
// making packs and registries.
//
//go:embed templates/*
var content embed.FS

var (
	tpl *template.Template
)

func init() {
	var err error
	tpl, err = template.ParseFS(content, "templates/*")
	if err != nil {
		panic(err)
	}
}
