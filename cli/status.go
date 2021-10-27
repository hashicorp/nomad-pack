package cli

import (
	"fmt"

	v1 "github.com/hashicorp/nomad-openapi/v1"
	flag "github.com/hashicorp/nomad-pack/flag"
	"github.com/hashicorp/nomad-pack/internal/pkg/cache"
	"github.com/hashicorp/nomad-pack/internal/pkg/errors"
	"github.com/hashicorp/nomad-pack/terminal"
	"github.com/posener/complete"
)

type StatusCommand struct {
	*baseCommand
	packConfig *cache.PackConfig
}

func (c *StatusCommand) Run(args []string) int {
	c.cmdKey = "status" // Add cmdKey here to print out helpUsageMessage on Init error
	// Initialize. If we fail, we just exit since Init handles the UI.
	if err := c.Init(
		WithCustomArgs(args, validateStatusArgs),
		WithFlags(c.Flags()),
		WithNoConfig(),
	); err != nil {
		c.ui.ErrorWithContext(err, ErrParsingArgsOrFlags)
		c.ui.Info(c.helpUsageMessage())
		return 1
	}

	if len(c.args) > 0 {
		c.packConfig.Name = c.args[0]
	}

	// Set the packConfig defaults if necessary and generate our UI error context.
	errorContext := errors.NewUIErrorContext()
	errorContext.Add(errors.UIContextPrefixPackName, c.packConfig.Name)

	client, err := v1.NewClient()
	if err != nil {
		c.ui.ErrorWithContext(err, "failed to initialize client", errorContext.GetAll()...)
		return 1
	}

	jobsApi := client.Jobs()

	// If pack name isn't specified, return all deployed packs
	if c.packConfig.Name == "" {
		return c.renderAllDeployedPacks(jobsApi, errorContext)
	}

	return c.renderDeployedPackJobs(err, jobsApi, errorContext)
}

func (c *StatusCommand) renderDeployedPackJobs(err error, jobsApi *v1.Jobs, errorContext *errors.UIErrorContext) int {
	packJobs, jobErrs, err := getDeployedPackJobs(jobsApi, c.packConfig, c.deploymentName)
	if err != nil {
		c.ui.ErrorWithContext(err, "error retrieving jobs", errorContext.GetAll()...)
		return 1
	}

	if len(packJobs) == 0 {
		msg := fmt.Sprintf("no jobs found for pack %q", c.packConfig.Name)
		if c.deploymentName != "" {
			msg += fmt.Sprintf(" in deployment %q", c.deploymentName)
		}
		c.ui.Warning(msg)
		return 0
	}

	c.ui.Table(formatDeployedPackJobs(packJobs))

	if len(jobErrs) > 0 {
		c.ui.WarningBold("error retrieving job status for the following jobs:")
		c.ui.Table(formatDeployedPackErrs(jobErrs))
	}

	return 0
}

func (c *StatusCommand) renderAllDeployedPacks(jobsApi *v1.Jobs, errorContext *errors.UIErrorContext) int {
	packRegistryMap, err := getDeployedPacks(jobsApi)
	if err != nil {
		c.ui.ErrorWithContext(err, "error retrieving packs", errorContext.GetAll()...)
		return 1
	}

	if len(packRegistryMap) == 0 {
		c.ui.Warning("no packs found")
		return 0
	}

	c.ui.Table(formatDeployedPacks(packRegistryMap))

	return 0
}

func (c *StatusCommand) Flags() *flag.Sets {
	return c.flagSet(0, func(set *flag.Sets) {
		c.packConfig = &cache.PackConfig{}

		f := set.NewSet("Status Options")

		f.StringVar(&flag.StringVar{
			Name:    "registry",
			Target:  &c.packConfig.Registry,
			Default: "",
			Usage: `Specific registry name containing the pack to inspect.
If not specified, the default registry will be used.`,
		})

		f.StringVar(&flag.StringVar{
			Name:    "ref",
			Target:  &c.packConfig.Ref,
			Default: "",
			Usage: `Specific git ref of the pack to inspect. 
Supports tags, SHA, and latest. If no ref is specified, defaults to latest.

Using ref with a file path is not supported.`,
		})
	})
}

func (c *StatusCommand) AutocompleteArgs() complete.Predictor {
	return complete.PredictNothing
}

func (c *StatusCommand) AutocompleteFlags() complete.Flags {
	return c.Flags().Completions()
}

func (c *StatusCommand) Help() string {
	c.Example = `
	# Get a list of all deployed packs and their registries
	nomad-pack status
	
	# Get a list of all deployed jobs in pack example, along with their status and deployment names
	nomad-pack status example

	# Get a list of all deployed jobs and their status for an example pack in the deployment name "dev"
	nomad-pack status example --name=dev

    # Get a list of all deployed jobs and their status for an example pack in the deployment name "dev"
	nomad-pack status example --name=dev --registry=community
	`

	return formatHelp(`
	Usage: nomad-pack status <name> [options]

	Get information on deployed Nomad Packs. If no pack name is specified, it will return
	a list of all deployed packs. If pack name is specified, it will return a list of all
	deployed jobs belonging to that pack, along with their status and deployment names.

` + c.GetExample() + c.Flags().Help())
}

func (c *StatusCommand) Synopsis() string {
	return "Get information on deployed packs"
}

// Custom validation function
func validateStatusArgs(b *baseCommand, args []string) error {
	if len(args) > 1 {
		return fmt.Errorf("this command accepts at most 1 arg, received %d", len(args))
	}

	// Flags are already parsed when this function is run
	// Verify pack name is provided if --name flag is used
	if b.deploymentName != "" && len(args) == 0 {
		return fmt.Errorf("--name can only be used if pack name is provided")
	}
	return nil
}

func formatDeployedPacks(packRegistryMap map[string]map[string]struct{}) *terminal.Table {
	tbl := terminal.NewTable("Pack Name", "Registry Name")
	for packName, registryMap := range packRegistryMap {
		for registryName := range registryMap {
			row := []terminal.TableEntry{}
			row = append(row, terminal.TableEntry{Value: packName})
			row = append(row, terminal.TableEntry{Value: registryName})
			tbl.Rows = append(tbl.Rows, row)
		}
	}
	return tbl
}

func formatDeployedPackJobs(packJobs []JobStatusInfo) *terminal.Table {
	tbl := terminal.NewTable("Pack Name", "Registry Name", "Deployment Name", "Job Name", "Status")
	for _, jobInfo := range packJobs {
		row := []terminal.TableEntry{}
		row = append(row, terminal.TableEntry{Value: jobInfo.packName})
		row = append(row, terminal.TableEntry{Value: jobInfo.registryName})
		row = append(row, terminal.TableEntry{Value: jobInfo.deploymentName})
		row = append(row, terminal.TableEntry{Value: jobInfo.jobID})
		row = append(row, terminal.TableEntry{Value: jobInfo.status})
		tbl.Rows = append(tbl.Rows, row)
	}
	return tbl
}

func formatDeployedPackErrs(packErrs []JobStatusError) *terminal.Table {
	tbl := terminal.NewTable("Job Name", "Error")
	for _, jobInfo := range packErrs {
		row := []terminal.TableEntry{}
		row = append(row, terminal.TableEntry{Value: jobInfo.jobID})
		row = append(row, terminal.TableEntry{Value: jobInfo.jobError.Error(), Color: terminal.Red})
		tbl.Rows = append(tbl.Rows, row)
	}
	return tbl
}
