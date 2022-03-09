package creator

import (
	"embed"
	"text/template"
)

// content holds the go-templates used by creator while
// making packs and registries.
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
