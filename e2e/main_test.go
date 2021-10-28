package main

import (
	"os"
	"testing"

	"github.com/hashicorp/nomad/e2e/framework"
)

func TestE2E(t *testing.T) {
	_ = os.Setenv("NOMAD_E2E", "1")
	if os.Getenv("NOMAD_E2E") == "" {
		t.Skip("Skipping e2e tests, NOMAD_E2E not set")
	} else {
		framework.Run(t)
	}
}
