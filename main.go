package main

import (
	"os"
	"path/filepath"

	"github.com/hashicorp/nomad-pack/cli"
)

func main() {
	os.Args[0] = filepath.Base(os.Args[0])
	os.Exit(cli.Main(os.Args))
}
