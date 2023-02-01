// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package main

import (
	"fmt"
	"os"

	"github.com/hashicorp/nomad-pack/internal/cli"
)

func main() {
	args := os.Args
	if len(args) != 2 {
		fmt.Printf("gendocs: requires 1 parameter, received %v\n", len(args))
		os.Exit(1)
	}
	mode := args[1]
	switch mode {
	case "md", "mdx":
		// these are valid
	default:
		fmt.Printf("gendocs: type parameter must be one of [md, mdx].\n")
		os.Exit(1)
	}
	cli.ExposeDocs = true
	os.Exit(cli.Main([]string{"nomad-pack", "gen-cli-docs", mode}))
}
