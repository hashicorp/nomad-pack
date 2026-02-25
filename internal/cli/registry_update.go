// Copyright IBM Corp. 2021, 2025
// SPDX-License-Identifier: MPL-2.0

package cli

import (
	"fmt"
	"strings"

	"github.com/posener/complete"

	"github.com/hashicorp/nomad-pack/internal/pkg/caching"
	"github.com/hashicorp/nomad-pack/internal/pkg/errors"
	"github.com/hashicorp/nomad-pack/internal/pkg/flag"
	"github.com/hashicorp/nomad-pack/terminal"
)

// RegistryUpdateCommand updates a previously added registry in the global cache.
type RegistryUpdateCommand struct {
	*baseCommand
	name   string
	target string
	ref    string
}

func (c *RegistryUpdateCommand) Run(args []string) int {
	c.cmdKey = "registry update"
	flagSet := c.Flags()

	// Initialize. If we fail, we just exit since Init handles the UI.
	if err := c.Init(
		WithExactArgs(1, args),
		WithFlags(flagSet),
		WithNoConfig(),
		WithClient(false),
	); err != nil {
		c.ui.ErrorWithContext(err, ErrParsingArgsOrFlags)
		c.ui.Info(c.helpUsageMessage())
		return 1
	}

	errorContext := errors.NewUIErrorContext()

	c.name = args[0]

	errorContext.Add(errors.UIContextPrefixRegistryName, c.name)

	if c.target != "" {
		errorContext.Add(errors.UIContextPrefixRegistryTarget, c.target)
	}

	// Initialize the global cache so we can look up the existing registry.
	globalCache, err := caching.NewCache(&caching.CacheConfig{
		Path:   caching.DefaultCachePath(),
		Logger: c.ui,
	})
	if err != nil {
		c.ui.ErrorWithContext(err, "failed to initialize cache")
		return 1
	}

	// Load the cache so we can verify the registry exists and retrieve its source.
	err = globalCache.Load()
	if err != nil {
		c.ui.ErrorWithContext(err, "failed to load global cache while verifying registry")
		return 1
	}

	// Find the existing registry by name so we can reuse its source URL.
	var existingRegistry *caching.Registry
	for _, r := range globalCache.Registries() {
		if r.Name == c.name {
			existingRegistry = r
			break
		}
	}

	if existingRegistry == nil {
		c.ui.ErrorWithContext(
			errors.New("registry not found in cache"),
			fmt.Sprintf("Registry %q has not been added yet. Use \"nomad-pack registry add\" first.", c.name),
			errorContext.GetAll()...,
		)
		return 1
	}

	// Verify the cached registry has a valid source URL.
	if existingRegistry.Source == "" {
		c.ui.ErrorWithContext(
			errors.New("registry source not found"),
			fmt.Sprintf("Registry %q exists but has no source URL recorded. Please delete and re-add the registry.", c.name),
			errorContext.GetAll()...,
		)
		return 1
	}

	source := existingRegistry.Source
	errorContext.Add(errors.UIContextPrefixGitRegistryURL, source)

	newRegistry, err := globalCache.Add(&caching.AddOpts{
		RegistryName: c.name,
		Source:       source,
		PackName:     c.target,
		Ref:          c.ref,
	})
	if err != nil {
		c.ui.ErrorWithContext(err, "failed to update registry")
		return 1
	}

	// If subprocess fails to update any packs, report this to the user.
	if newRegistry == nil || len(newRegistry.Packs) == 0 {
		c.ui.ErrorWithContext(errors.New("failed to update packs for registry"), "see output for reason", errorContext.GetAll()...)
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

	c.ui.Info("Registry successfully updated in cache.")
	c.ui.Table(table)

	if validPack != nil {
		c.ui.Info(fmt.Sprintf("Try running one of the packs you just updated like this\n\n  nomad-pack run %s --registry=%s --ref=%s", validPack.Name(), newRegistry.Name, validPack.Ref))
	}

	return 0
}

func (c *RegistryUpdateCommand) Flags() *flag.Sets {
	return c.flagSet(0, func(set *flag.Sets) {
		f := set.NewSet("Registry Options")

		f.StringVar(&flag.StringVar{
			Name:    "target",
			Target:  &c.target,
			Default: "",
			Usage:   `A specific pack within the registry to be updated.`,
		})

		f.StringVar(&flag.StringVar{
			Name:    "ref",
			Target:  &c.ref,
			Default: "",
			Usage: `Specific git ref of the registry or pack to be updated.
					Supports tags, SHA, and latest. If no ref is specified,
					defaults to latest. Running "nomad registry update"
					multiple times for the same ref is idempotent, however
					running "nomad-pack registry update" without specifying a
					ref, or when specifying @latest, is destructive, and will
					overwrite current @latest in the global cache.

					Using ref with a file path is not supported.`,
		})
	})
}

func (c *RegistryUpdateCommand) AutocompleteArgs() complete.Predictor {
	return complete.PredictNothing
}

func (c *RegistryUpdateCommand) AutocompleteFlags() complete.Flags {
	return c.Flags().Completions()
}

func (c *RegistryUpdateCommand) Synopsis() string {
	return "Update registries or packs in the local environment."
}

func (c *RegistryUpdateCommand) Help() string {
	c.Example = `
	# Update a previously added registry to the latest ref.
	nomad-pack registry update community

	# Update a specific pack from a previously added registry.
	nomad-pack registry update community --target=nomad_example

	# Update a previously added registry at a specific tag/release/SHA.
	nomad-pack registry update community --ref=v0.1.0
	`
	return formatHelp(`
	Usage: nomad-pack registry update <name> [options]

	Update a previously added nomad pack registry. The source URL is
	automatically retrieved from the cached registry metadata, so only
	the registry name is required. The registry must have been previously
	added using "nomad-pack registry add".

` + c.GetExample() + c.Flags().Help())
}
