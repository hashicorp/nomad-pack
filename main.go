package main

import (
	"github.com/hashicorp/nom/cli"
	"os"
	"path/filepath"
)

func main() {
	os.Args[0] = filepath.Base(os.Args[0])
	os.Exit(cli.Main(os.Args))
}
