// Copyright IBM Corp. 2023, 2026
// SPDX-License-Identifier: MPL-2.0

package cli

import (
	"fmt"
	"os/exec"
	"regexp"
	"strings"

	"github.com/posener/complete"

	"github.com/hashicorp/nomad-pack/internal/pkg/caching"
	"github.com/hashicorp/nomad-pack/internal/pkg/errors"
	"github.com/hashicorp/nomad-pack/internal/pkg/flag"
	"github.com/hashicorp/nomad-pack/terminal"
)

// Compile regex once for performance
var shaRegex = regexp.MustCompile(`^[0-9a-f]{7,40}$`)

// RegistryAddCommand adds a registry to the global cache.
type RegistryAddCommand struct {
	*baseCommand
	source     string
	name       string
	target     string
	ref        string
	branch     string
	skipVerify bool
}

func (c *RegistryAddCommand) Run(args []string) int {
	c.cmdKey = "registry add"
	flagSet := c.Flags()

	// Initialize. If we fail, we just exit since Init handles the UI.
	if err := c.Init(
		WithExactArgs(2, args),
		WithFlags(flagSet),
		WithNoConfig(),
		WithClient(false),
	); err != nil {
		c.ui.ErrorWithContext(err, ErrParsingArgsOrFlags)
		c.ui.Info(c.helpUsageMessage())
		return 1
	}

	if c.branch != "" && c.ref != "" {
		c.ui.Error("Cannot specify both --ref and --branch")
		c.ui.Info("  Use --ref for tags, SHAs, or any git reference")
		c.ui.Info("  Use --branch specifically for branch names")
		return 1
	}

	errorContext := errors.NewUIErrorContext()
	c.name = args[0]
	c.source = args[1]

	errorContext.Add(errors.UIContextPrefixRegistryName, c.name)
	errorContext.Add(errors.UIContextPrefixGitRegistryURL, c.source)

	if c.branch != "" {
		if err := c.processBranch(); err != nil {
			c.ui.Error(fmt.Sprintf("Branch validation failed: %s", err))
			return 1
		}
	}

	if c.target != "" {
		errorContext.Add(errors.UIContextPrefixRegistryTarget, c.target)
	}

	// Add the registry or registry target to the global cache
	globalCache, err := caching.NewCache(&caching.CacheConfig{
		Path:   caching.DefaultCachePath(),
		Logger: c.ui,
	})
	if err != nil {
		return 1
	}

	newRegistry, err := globalCache.Add(&caching.AddOpts{
		RegistryName: c.name,
		Source:       c.source,
		PackName:     c.target,
		Ref:          c.ref,
	})
	if err != nil {
		return 1
	}

	// If subprocess fails to add any packs, report this to the user.
	if newRegistry == nil || len(newRegistry.Packs) == 0 {
		c.ui.ErrorWithContext(errors.New("failed to add packs for registry"), "see output for reason", errorContext.GetAll()...)
		return 1
	}

	// Initialize output table
	var table *terminal.Table
	var validPack *caching.Pack
	// If only targeting a single pack, only output a single row
	if c.target != "" {
		table = registryPackTable()
		// It is safe to target pack 0 here because registry.AddFromGitURL will
		// ensure only the target pack is returned.
		tableRow := registryPackRow(newRegistry, newRegistry.Packs[0])
		table.Rows = append(table.Rows, tableRow)
		for _, registryPack := range newRegistry.Packs {
			if !strings.Contains(strings.ToLower(registryPack.Ref), "invalid") {
				validPack = registryPack
			}
		}
	} else {
		table = registryTable()
		for _, registry := range globalCache.Registries() {
			tableRow := registryTableRow(registry)
			table.Rows = append(table.Rows, tableRow)
		}
	}

	c.ui.Info("Registry successfully added to cache.")
	c.ui.Table(table)

	if validPack != nil {
		c.ui.Info(fmt.Sprintf("Try running one the packs you just added liked this\n\n  nomad-pack run %s --registry=%s --ref=%s", validPack.Name(), newRegistry.Name, validPack.Ref))
	}

	return 0
}

func (c *RegistryAddCommand) Flags() *flag.Sets {
	return c.flagSet(0, func(set *flag.Sets) {
		f := set.NewSet("Registry Options")

		f.StringVar(&flag.StringVar{
			Name:    "target",
			Target:  &c.target,
			Default: "",
			Usage:   `A specific pack within the registry to be added.`,
		})

		f.StringVar(&flag.StringVar{
			Name:    "ref",
			Target:  &c.ref,
			Default: "",
			Usage: `Specific git ref of the registry or pack to be added.
					Supports tags, SHA, and latest. If no ref is specified,
					defaults to latest. Running "nomad registry add" multiple
					times for the same ref is idempotent, however running
					"nomad-pack registry add" without specifying a ref, or when
					specifying @latest, is destructive, and will overwrite
					current @latest in the global cache.

					Using ref with a file path is not supported.`,
		})

		f.StringVar(&flag.StringVar{
			Name:    "branch",
			Target:  &c.branch,
			Default: "",
			Usage: `Specific git branch of the registry or pack to be added.
					Cannot be used with --ref. Branch names are case-sensitive.`,
		})

		f.BoolVar(&flag.BoolVar{
			Name:    "skip-verify",
			Target:  &c.skipVerify,
			Default: false,
			Usage:   `Skip remote branch verification (faster but less safe).`,
		})
	})
}

func (c *RegistryAddCommand) AutocompleteArgs() complete.Predictor {
	return complete.PredictNothing
}

func (c *RegistryAddCommand) AutocompleteFlags() complete.Flags {
	return c.Flags().Completions()
}

func (c *RegistryAddCommand) Synopsis() string {
	return "Add registries or packs to the local environment."
}

// processBranch validates and processes the branch flag
func (c *RegistryAddCommand) processBranch() error {
	// Validate branch name
	if err := validateBranchName(c.branch); err != nil {
		return err
	}

	// Warn if branch name looks like SHA
	if looksLikeSHA(c.branch) {
		c.ui.Warning(fmt.Sprintf(
			"Branch name '%s' looks like a commit SHA.\n"+
				"If you meant to use a SHA, use --ref instead of --branch.",
			c.branch))
	}

	// Verify branch exists remotely (unless skipped)
	if !c.skipVerify {
		if err := c.verifyRemoteBranch(); err != nil {
			return err
		}
	}

	c.ref = c.branch
	return nil
}

// validateBranchName validates branch name format
func validateBranchName(branch string) error {
	if len(branch) == 0 {
		return errors.New("branch name cannot be empty")
	}

	if len(branch) > 255 {
		return fmt.Errorf("branch name too long (max 255 characters, got %d)", len(branch))
	}

	// Check for invalid characters
	invalidChars := []string{"\x00", "..", "~", "^", ":", "?", "*", "[", "\\"}
	for _, char := range invalidChars {
		if strings.Contains(branch, char) {
			return fmt.Errorf("branch name contains invalid character: %q", char)
		}
	}

	// Cannot start/end with slash or dot
	if strings.HasPrefix(branch, "/") || strings.HasSuffix(branch, "/") {
		return errors.New("branch name cannot start or end with '/'")
	}

	if strings.HasPrefix(branch, ".") || strings.HasSuffix(branch, ".") {
		return errors.New("branch name cannot start or end with '.'")
	}

	// Cannot have consecutive slashes
	if strings.Contains(branch, "//") {
		return errors.New("branch name cannot contain consecutive slashes")
	}

	return nil
}

// looksLikeSHA checks if string looks like a git SHA (optimized with pre-compiled regex)
func looksLikeSHA(s string) bool {
	return shaRegex.MatchString(s)
}

// convertToGitURL converts go-getter style URLs to proper git URLs
func convertToGitURL(source string) string {
	// If already has protocol, return as-is
	if strings.HasPrefix(source, "http://") ||
		strings.HasPrefix(source, "https://") ||
		strings.HasPrefix(source, "git@") {
		return source
	}

	// Convert github.com/owner/repo to https://github.com/owner/repo
	if strings.HasPrefix(source, "github.com/") {
		return "https://" + source
	}

	// For other cases, try adding https://
	return "https://" + source
}

// verifyRemoteBranch checks if branch exists in remote repository
func (c *RegistryAddCommand) verifyRemoteBranch() error {
	// Validate source is not empty
	if c.source == "" {
		return errors.New("repository source cannot be empty")
	}
	gitURL := convertToGitURL(c.source)
	cmd := exec.Command("git", "ls-remote", "--heads", gitURL, c.branch)
	output, err := cmd.CombinedOutput()

	if err != nil {
		// Check for authentication errors
		if strings.Contains(string(output), "Authentication failed") ||
			strings.Contains(string(output), "Permission denied") {
			return fmt.Errorf(
				"authentication failed for repository '%s'.\n"+
					"Ensure you have access and credentials are configured",
				c.source)
		}

		// Check for connection errors
		if strings.Contains(string(output), "Could not resolve host") {
			return fmt.Errorf(
				"could not connect to repository '%s'.\n"+
					"Check the URL and your network connection",
				c.source)
		}

		return fmt.Errorf("failed to verify branch: %w", err)
	}

	// Branch not found
	if len(output) == 0 {
		return fmt.Errorf(
			"branch '%s' not found in remote repository '%s'.\n"+
				"List available branches with: git ls-remote --heads %s",
			c.branch, c.source, c.source)
	}

	return nil
}

func (c *RegistryAddCommand) Help() string {
	c.Example = `
	# Download latest ref of the pack registry to the global cache.
	nomad-pack registry add community github.com/hashicorp/nomad-pack-community-registry

	# Download latest ref of a specific pack from the registry to the global cache.
	nomad-pack registry add community github.com/hashicorp/nomad-pack-community-registry --target=nomad_example

	# Download packs from a registry at a specific tag/release/SHA.
	nomad-pack registry add community github.com/hashicorp/nomad-pack-community-registry  --ref=v0.1.0

	# Download packs from a specific branch.
	nomad-pack registry add community github.com/hashicorp/nomad-pack-community-registry --branch=main

	# Download packs from a feature branch (use --skip-verify for faster operation).
    nomad-pack registry add community github.com/hashicorp/nomad-pack-community-registry --branch=feature/add-templates --skip-verify

    # Note: Branch names with slashes may fail due to underlying library limitations.
    # Consider using underscores instead: feature_add_templates
	`
	return formatHelp(`
	Usage: nomad-pack registry add <name> <source> [options]

	Add nomad pack registries.

` + c.GetExample() + c.Flags().Help())
}
