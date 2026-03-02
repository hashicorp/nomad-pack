// Copyright IBM Corp. 2021, 2025
// SPDX-License-Identifier: MPL-2.0

package cli

import (
	"testing"

	"github.com/shoenig/test/must"
)

// TestCLI_RegistryUpdate_MissingRegistry verifies that updating a registry
// that has not been previously added returns an error.
func TestCLI_RegistryUpdate_MissingRegistry(t *testing.T) {
	result := runPackCmd(t, []string{"registry", "update", "nonexistent-registry"})
	must.Eq(t, 1, result.exitCode)
	must.StrContains(t, result.cmdOut.String(), "Has Not Been Added Yet")
}

// TestCLI_RegistryUpdate_TooManyArgs verifies that providing a source argument
// (the old behavior) now returns an error, since only the registry name is required.
func TestCLI_RegistryUpdate_TooManyArgs(t *testing.T) {
	result := runPackCmd(t, []string{"registry", "update", "my-registry", "github.com/some/repo"})
	must.Eq(t, 1, result.exitCode)
	must.StrContains(t, result.cmdOut.String(), "this command requires exactly 1 args")
}

// TestCLI_RegistryUpdate_NoArgs verifies that providing no arguments returns an error
// and shows usage information.
func TestCLI_RegistryUpdate_NoArgs(t *testing.T) {
	result := runPackCmd(t, []string{"registry", "update"})
	must.Eq(t, 1, result.exitCode)
	must.StrContains(t, result.cmdOut.String(), "this command requires exactly 1 args")
	must.StrContains(t, result.cmdOut.String(), `See "nomad-pack registry update --help"`)
}

// TestCLI_RegistryUpdate_ExistingRegistry verifies that updating a previously
// added registry works using only the registry name. The source URL is
// automatically retrieved from the cached registry metadata.
func TestCLI_RegistryUpdate_ExistingRegistry(t *testing.T) {
	// Create a test registry in the cache (this creates both "latest" and testRef refs).
	reg, _, regPath := createTestRegistries(t)
	defer cleanTestRegistry(t, regPath)

	// Attempt to update. This will use the source URL from the cached metadata.
	// Because the source is a test URL that doesn't point to a real git repo,
	// the clone step inside Add will fail. But the important thing is that the
	// command gets past argument parsing and registry lookup successfully.
	result := runPackCmd(t, []string{"registry", "update", reg.Name})

	// The command will fail at the git clone stage because the test source URL
	// is not a real repository, but it should NOT fail with "has not been added yet".
	out := result.cmdOut.String()
	must.StrNotContains(t, out, "Has Not Been Added Yet")
}

// TestCLI_RegistryUpdate_WithRef verifies that the --ref flag is accepted when
// updating a previously added registry and the command gets past argument
// parsing and registry lookup.
func TestCLI_RegistryUpdate_WithRef(t *testing.T) {
	reg, _, regPath := createTestRegistries(t)
	defer cleanTestRegistry(t, regPath)

	result := runPackCmd(t, []string{"registry", "update", reg.Name, "--ref=v0.1.0"})

	out := result.cmdOut.String()
	// Should not fail with registry-not-found or argument parsing errors.
	must.StrNotContains(t, out, "Has Not Been Added Yet")
	must.StrNotContains(t, out, "this command requires exactly 1 args")
}
