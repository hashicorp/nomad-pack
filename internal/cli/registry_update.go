// Copyright IBM Corp. 2021, 2025
// SPDX-License-Identifier: MPL-2.0

package cli

import (
	"fmt"
	"slices"
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
	source string
	name   string
	target string
	ref    string
}

func (c *RegistryUpdateCommand) Run(args []string) int {
	c.cmdKey = "registry update"
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

	errorContext := errors.NewUIErrorContext()

	c.name = args[0]
	c.source = args[1]

	errorContext.Add(errors.UIContextPrefixRegistryName, c.name)
	errorContext.Add(errors.UIContextPrefixGitRegistryURL, c.source)

	if c.target != "" {
		errorContext.Add(errors.UIContextPrefixRegistryTarget, c.target)
	}

	// Update the registry or registry target in the global cache
	globalCache, err := caching.NewCache(&caching.CacheConfig{
		Path:   caching.DefaultCachePath(),
		Logger: c.ui,
	})
	if err != nil {
		c.ui.ErrorWithContext(err, "failed to initialize cache")
		return 1
	}

	// Load the cache so we can verify the registry exists before updating.
	err = globalCache.Load()
	if err != nil {
		c.ui.ErrorWithContext(err, "failed to load global cache while verifying registry")
		return 1
	}

	// Check that the registry has been previously added.
	registryExists := slices.ContainsFunc(
		globalCache.Registries(),
		func(r *caching.Registry) bool {
			return r.Name == c.name
		},
	)

	if !registryExists {
		c.ui.ErrorWithContext(
			errors.New("registry not found in cache"),
			fmt.Sprintf("Registry %q has not been added yet. Use \"nomad-pack registry add\" first.", c.name),
			errorContext.GetAll()...,
		)
		return 1
	}

	newRegistry, err := globalCache.Add(&caching.AddOpts{
		RegistryName: c.name,
		Source:       c.source,
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
	# Update latest ref of the pack registry in the global cache.
	nomad-pack registry update community github.com/hashicorp/nomad-pack-community-registry

	# Update latest ref of a specific pack from the registry in the global cache.
	nomad-pack registry update community github.com/hashicorp/nomad-pack-community-registry --target=nomad_example

	# Update packs from a registry at a specific tag/release/SHA.
	nomad-pack registry update community github.com/hashicorp/nomad-pack-community-registry --ref=v0.1.0
	`
	return formatHelp(`
	Usage: nomad-pack registry update <name> <source> [options]

	Update nomad pack registries. The registry must have been previously added
	using "nomad-pack registry add".

` + c.GetExample() + c.Flags().Help())
}
