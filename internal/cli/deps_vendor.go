package cli

import (
	"context"
	"time"

	"github.com/posener/complete"

	"github.com/hashicorp/nomad-pack/internal/pkg/cache"
	"github.com/hashicorp/nomad-pack/internal/pkg/deps"
	"github.com/hashicorp/nomad-pack/internal/pkg/errors"
	"github.com/hashicorp/nomad-pack/internal/pkg/flag"
)

type depsVendorCommand struct {
	*baseCommand
}

func (d *depsVendorCommand) Run(args []string) int {
	d.cmdKey = "deps vendor"
	flagSet := d.Flags()

	// Initialize. If we fail, we just exit since Init handles the UI.
	if err := d.Init(
		WithNoArgs(args),
		WithFlags(flagSet),
		WithNoConfig(),
		WithClient(false),
	); err != nil {
		d.ui.ErrorWithContext(err, "error parsing args or flags")
		return 1
	}

	// Generate our UI error context.
	errorContext := errors.NewUIErrorContext()

	// Get the global cache dir - may be configurable in the future, so using this
	// helper function rather than a direct reference to the CONST.
	globalCache, err := cache.NewCache(&cache.CacheConfig{
		Path:   cache.DefaultCachePath(),
		Logger: d.ui,
	})
	if err != nil {
		return 1
	}

	// Load the list of registries.
	err = globalCache.Load()
	if err != nil {
		return 1
	}

	// FIXME make these flags
	copyToCache := true
	targetPath := ""
	timeout := 30 * time.Second
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	err = deps.Vendor(ctx, d.ui, globalCache, copyToCache, targetPath)
	if err != nil {
		d.ui.ErrorWithContext(err, "failed to vendor dependencies", errorContext.GetAll()...)
		return 1
	}
	return 0
}

func (d *depsVendorCommand) Flags() *flag.Sets {
	return d.flagSet(0, nil)
}

func (d *depsVendorCommand) AutocompleteArgs() complete.Predictor {
	return complete.PredictNothing
}

func (d *depsVendorCommand) AutocompleteFlags() complete.Flags {
	return d.Flags().Completions()
}

func (d *depsVendorCommand) Synopsis() string {
	return "Vendor dependencies for a pack."
}

func (d *depsVendorCommand) Help() string {
	return formatHelp(`
	Usage: nomad-pack deps vendor

	Vendor dependencies for a pack in the current directory.

` + d.GetExample() + d.Flags().Help())
}
