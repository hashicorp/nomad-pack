// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package cli

import (
	"context"
	"time"

	"github.com/posener/complete"

	"github.com/hashicorp/nomad-pack/internal/pkg/deps"
	"github.com/hashicorp/nomad-pack/internal/pkg/errors"
	"github.com/hashicorp/nomad-pack/internal/pkg/flag"
)

type depsVendorCommand struct {
	*baseCommand
	targetPath string
	seconds    int
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
		d.ui.ErrorWithContext(err, ErrParsingArgsOrFlags)
		d.ui.Info(d.helpUsageMessage())
		return 1
	}

	// Generate our UI error context.
	errorContext := errors.NewUIErrorContext()

	timeout := time.Duration(d.seconds) * time.Second
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	err := deps.Vendor(ctx, d.ui, d.targetPath)
	if err != nil {
		d.ui.ErrorWithContext(err, "failed to vendor dependencies", errorContext.GetAll()...)
		return 1
	}
	return 0
}

func (d *depsVendorCommand) Flags() *flag.Sets {
	return d.flagSet(0, func(set *flag.Sets) {
		f := set.NewSet("Vendoring Options")

		f.StringVar(&flag.StringVar{
			Name:    "path",
			Target:  &d.targetPath,
			Default: "",
			Usage: `Full path to the pack which contains dependencies to be
				    vendored. All the dependencies will then be downloaded
				    into a 'deps/' subdirectory of that path. `,
		})

		f.IntVar(&flag.IntVar{
			Name:    "timeout",
			Target:  &d.seconds,
			Default: 30,
			Usage:   `Timeout (in seconds) for downloading dependencies.`,
		})
	})
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
