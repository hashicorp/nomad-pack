package cli

import (
	"fmt"

	v1 "github.com/hashicorp/nomad-openapi/v1"
	flag "github.com/hashicorp/nomad-pack/flag"
	"github.com/hashicorp/nomad-pack/internal/pkg/errors"
	"github.com/hashicorp/nomad-pack/terminal"
	"github.com/posener/complete"
)

type StatusCommand struct {
	*baseCommand
	packName     string
	registryName string
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

	// Generate our UI error context.
	errorContext := errors.NewUIErrorContext()

	// Check if pack name specified
	packName := ""
	registryName := ""
	if len(args) > 0 {
		var err error
		packRegistryName := c.args[0]
		registryName, packName, err = parseRegistryAndPackName(packRegistryName)
		if err != nil {
			c.ui.ErrorWithContext(err, "failed to parse pack name", errorContext.GetAll()...)
			return 1
		}
		errorContext.Add(errors.UIContextPrefixRegistryName, registryName)
		errorContext.Add(errors.UIContextPrefixPackName, packName)
	}
	c.packName = packName
	c.registryName = registryName

	client, err := v1.NewClient()
	if err != nil {
		c.ui.ErrorWithContext(err, "failed to initialize client", errorContext.GetAll()...)
		return 1
	}
	jobsApi := client.Jobs()
	// If pack name isn't specified, return all deployed packs
	if c.packName == "" {
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

	packJobs, jobErrs, err := getDeployedPackJobs(jobsApi, c.packName, c.registryName, c.deploymentName)
	if err != nil {
		c.ui.ErrorWithContext(err, "error retrieving jobs", errorContext.GetAll()...)
		return 1
	}
	if len(packJobs) == 0 {
		msg := fmt.Sprintf("no jobs found for pack %q", packName)
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

func (c *StatusCommand) Flags() *flag.Sets {
	return c.flagSet(0, func(set *flag.Sets) {
		f := set.NewSet("Status Options")

		f.StringVar(&flag.StringVar{
			Name:    "name",
			Target:  &c.deploymentName,
			Default: "",
			Usage: `If set, will filter the list of deployed jobs to only those
			in this deployment and belonging to the specified pack. Can only
			be used if pack name is provided. Specifying --name without providing
			a pack name will result in an error. 
			`,
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
	`

	return formatHelp(`
	Usage: nomad-pack status [options]

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
