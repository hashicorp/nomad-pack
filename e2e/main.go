package main

import (
	"fmt"
)

// This file exists so that e2e is a separate package, and does not import the
// Nomad dependencies into the main nomad-pack binary.
func main() {
	fmt.Println("Run make e2e to run end-to-end tests...")
}
