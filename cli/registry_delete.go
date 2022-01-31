package cli

import (
	"fmt"
	"strings"

	"github.com/hashicorp/nomad-pack/internal/pkg/cache"
	"github.com/hashicorp/nomad-pack/internal/pkg/errors"
	"github.com/hashicorp/nomad-pack/internal/pkg/flag"
	"github.com/posener/complete"
)

// RegistryDeleteCommand deletes a registry from the global cache.
type RegistryDeleteCommand struct {
	*baseCommand
	command string
	name    string
	target  string
	ref     string
}

func (c *RegistryDeleteCommand) Run(args []string) int {
	c.cmdKey = "registry delete"
	flagSet := c.Flags()

	// Initialize. If we fail, we just exit since Init handles the UI.
	if err := c.Init(
		WithExactArgs(1, args),
		WithFlags(flagSet),
		WithNoConfig(),
		WithClient(false),
	); err != nil {
		c.ui.ErrorWithContext(err, "error parsing args or flags")
		return 1
	}

	c.name = args[0]

	errorContext := errors.NewUIErrorContext()
	errorContext.Add(errors.UIContextPrefixRegistryName, c.name)
	errorContext.Add(errors.UIContextPrefixRegistryTarget, c.target)

	// Get the global cache dir - may be configurable in the future, so using this
	// helper function rather than a direct reference to the CONST.
	globalCache, err := cache.NewCache(&cache.CacheConfig{
		Path:   cache.DefaultCachePath(),
		Logger: c.ui,
	})
	if err != nil {
		return 1
	}

	err = globalCache.Delete(&cache.DeleteOpts{
		RegistryName: c.name,
		PackName:     c.target,
		Ref:          c.ref,
	})
	if err != nil {
		c.ui.ErrorWithContext(err, "error deleting registry")
		return 1
	}

	c.ui.Info(c.formatOutput())

	return 0
}

func (c *RegistryDeleteCommand) formatOutput() string {
	// Format output based on passed flags.
	var output strings.Builder
	output.WriteString(fmt.Sprintf("\nregistry %s", c.name))

	if c.target != "" {
		output.WriteString(fmt.Sprintf(" pack %s", c.target))
	}

	if c.ref != "" {
		output.WriteString(fmt.Sprintf(" at ref %s", c.ref))
	}

	output.WriteString(" deleted")

	return output.String()
}

func (c *RegistryDeleteCommand) Flags() *flag.Sets {
	return c.flagSet(0, func(set *flag.Sets) {
		f := set.NewSet("Registry Options")

		f.StringVar(&flag.StringVar{
			Name:    "target",
			Target:  &c.target,
			Default: "",
			Usage: `A specific pack within the registry to be deleted. 
If a ref flag has been added, only that ref of the target pack will be deleted.
`,
		})

		f.StringVar(&flag.StringVar{
			Name:    "ref",
			Target:  &c.ref,
			Default: "",
			Usage: `Specific git ref of the registry or pack to be deleted. 
Supports tags, SHA, and latest. If no ref is specified, defaults to 
latest.

Using ref with a file path is not supported.`,
		})
	})
}

func (c *RegistryDeleteCommand) AutocompleteArgs() complete.Predictor {
	return complete.PredictNothing
}

func (c *RegistryDeleteCommand) AutocompleteFlags() complete.Flags {
	return c.Flags().Completions()
}

func (c *RegistryDeleteCommand) Synopsis() string {
	return "Delete registries or packs from the local environment."
}

func (c *RegistryDeleteCommand) Help() string {
	c.Example = `
	# Delete a pack registry, optionally at a specific tag/release/SHA.
	If no target or tag/release/SHA defined, will delete the entire registry.

	nomad-pack registry delete community --target=traefik --ref=v0.0.1
	`
	return formatHelp(`
	Usage: nomad-pack registry delete <name> [options]

	Delete nomad pack registries or packs.
	
` + c.GetExample() + c.Flags().Help())
}
