// Copyright IBM Corp. 2021, 2025
// SPDX-License-Identifier: MPL-2.0

package main

import (
	"os"
	"path/filepath"

	"github.com/hashicorp/nomad-pack/internal/cli"
)

func main() {
	os.Args[0] = filepath.Base(os.Args[0])
	os.Exit(cli.Main(os.Args))
}
