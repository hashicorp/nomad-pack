package cli

import (
	"fmt"
	"path"

	v1 "github.com/hashicorp/nomad-openapi/v1"
	"github.com/hashicorp/nomad-pack/flag"
	"github.com/hashicorp/nomad-pack/internal/pkg/errors"
	"github.com/hashicorp/nomad-pack/internal/pkg/registry"
	"github.com/hashicorp/nomad-pack/internal/runner"
	"github.com/hashicorp/nomad-pack/internal/runner/job"
	"github.com/posener/complete"
)

type PlanCommand struct {
	*baseCommand
	packName     string
	packVersion  string
	registryName string
	jobConfig    *job.CLIConfig
}

func (c *PlanCommand) Run(args []string) int {
	var err error
	c.cmdKey = "plan"
	// Initialize. If we fail, we just exit since Init handles the UI.
	if err := c.Init(
		WithExactArgs(1, args),
		WithFlags(c.Flags()),
		WithNoConfig(),
	); err != nil {
		c.ui.ErrorWithContext(err, "error parsing args or flags")
		return 255
	}

	c.packName = c.args[0]

	// Generate our UI error context.
	errorContext := errors.NewUIErrorContext()

	c.registryName, c.packName, err = parseRegistryAndPackName(c.packName)
	if err != nil {
		c.ui.ErrorWithContext(err, "failed to parse pack name", errorContext.GetAll()...)
		return 1
	}

	errorContext.Add(errors.UIContextPrefixPackName, c.packName)
	errorContext.Add(errors.UIContextPrefixRegistryName, c.registryName)

	registryPath, err := getRegistryPath(c.registryName, c.ui, errorContext)
	if err != nil {
		c.ui.ErrorWithContext(err, "failed to identify repository path")
		return 255
	}

	// Add the path to the pack on the error context.
	errorContext.Add(errors.UIContextPrefixPackPath, registryPath)

	// verify packs exist before planning jobs
	if err = verifyPackExist(c.ui, c.packName, registryPath, errorContext); err != nil {
		return 255
	}

	// split pack name and version
	// TODO: Get this from pack metadata.
	c.packName, c.packVersion, err = registry.ParsePackNameAndVersion(c.packName)
	if err != nil {
		c.ui.ErrorWithContext(err, "failed to determine pack version", errorContext.GetAll()...)
	}

	// Add the path to the pack on the error context.
	errorContext.Add(errors.UIContextPrefixPackVersion, c.packVersion)

	// If no deploymentName set default to pack@version
	c.deploymentName = getDeploymentName(c.baseCommand, c.packName, c.packVersion)
	errorContext.Add(errors.UIContextPrefixDeploymentName, c.deploymentName)

	client, err := v1.NewClient()
	if err != nil {
		c.ui.ErrorWithContext(err, "failed to initialize client", errorContext.GetAll()...)
		return 255
	}

	packManager := generatePackManager(c.baseCommand, client, registryPath, fmt.Sprintf("%s@%s", c.packName, c.packVersion))

	// load pack
	r, err := renderPack(packManager, c.baseCommand.ui, errorContext)
	if err != nil {
		return 255
	}

	// Commands that render templates are required to render at least one
	// parent template.
	if r.LenParentRenders() < 1 {
		c.ui.ErrorWithContext(errors.ErrNoTemplatesRendered, "no templates rendered", errorContext.GetAll()...)
		return 255
	}

	depConfig := runner.Config{
		PackName:       c.packName,
		PathPath:       path.Join(registryPath, c.packName),
		PackVersion:    c.packVersion,
		DeploymentName: c.deploymentName,
	}

	// TODO(jrasell) come up with a better way to pass the appropriate config.
	jobRunner, err := generateRunner(client, "job", c.jobConfig, &depConfig)
	if err != nil {
		c.ui.ErrorWithContext(err, "failed to generate deployer", errorContext.GetAll()...)
		return 255
	}

	// Set the rendered templates on the job deployer.
	jobRunner.SetTemplates(r.ParentRenders())

	// Parse the templates. If we have any error, output this and exit.
	if validateErrs := jobRunner.ParseTemplates(); validateErrs != nil {
		for _, validateErr := range validateErrs {
			validateErr.Context.Append(errorContext)
			c.ui.ErrorWithContext(validateErr.Err, validateErr.Subject, validateErr.Context.GetAll()...)
		}
		return 255
	}

	// TODO(jrasell) we should call canonicalize here, but need additional CMD
	//  flags.

	if conflictErrs := jobRunner.CheckForConflicts(errorContext); conflictErrs != nil {
		for _, conflictErr := range conflictErrs {
			c.ui.ErrorWithContext(conflictErr.Err, conflictErr.Subject, conflictErr.Context.GetAll()...)
		}
		return 255
	}

	planExitCode, planErrs := jobRunner.PlanDeployment(c.ui, errorContext)
	if planErrs != nil {
		for _, planErrs := range planErrs {
			c.ui.ErrorWithContext(planErrs.Err, planErrs.Subject, planErrs.Context.GetAll()...)
		}
	}

	if planExitCode < 2 {
		c.ui.Success("Plan succeeded")
	}
	return planExitCode
}

func (c *PlanCommand) Flags() *flag.Sets {
	return c.flagSet(flagSetOperation, func(set *flag.Sets) {
		f := set.NewSet("Plan Options")

		c.jobConfig = &job.CLIConfig{
			RunConfig:  &job.RunCLIConfig{},
			PlanConfig: &job.PlanCLIConfig{},
		}

		f.BoolVar(&flag.BoolVar{
			Name:    "diff",
			Target:  &c.jobConfig.PlanConfig.Diff,
			Default: true,
			Usage: `Determines whether the diff between the remote job and planned 
                    job is shown. Defaults to true.`,
		})

		f.BoolVar(&flag.BoolVar{
			Name:    "policy-override",
			Target:  &c.jobConfig.PlanConfig.PolicyOverride,
			Default: false,
			Usage:   `Sets the flag to force override any soft mandatory Sentinel policies.`,
		})

		f.BoolVar(&flag.BoolVar{
			Name:    "hcl1",
			Target:  &c.jobConfig.PlanConfig.HCL1,
			Default: false,
			Usage:   `If set, HCL1 parser is used for parsing the job spec.`,
		})

		f.BoolVarP(&flag.BoolVarP{
			BoolVar: &flag.BoolVar{
				Name:    "verbose",
				Target:  &c.jobConfig.PlanConfig.Verbose,
				Default: false,
				Usage:   `Increase diff verbosity.`,
			},
			Shorthand: "v",
		})
	})
}

func (c *PlanCommand) AutocompleteArgs() complete.Predictor {
	return complete.PredictNothing
}

func (c *PlanCommand) AutocompleteFlags() complete.Flags {
	return c.Flags().Completions()
}

func (c *PlanCommand) Help() string {
	c.Example = `
	# Plan an example pack with the default deployment name "example@86a9235"
    # (default is <pack-name>@version).
	nomad-pack plan example

	# Plan an example pack with deployment name "dev"
	nomad-pack plan example --name=dev

	# Plan an example pack without showing the diff
	nomad-pack plan example --diff=false
	`

	return formatHelp(`
	Usage: nomad-pack plan <pack-name> [options]

	Determine the effects of submitting a new or updated Nomad Pack

    Plan will return one of the following exit codes:
		* code 0:   No objects will be created or destroyed.
		* code 1:   Objects will be created or destroyed.
		* code 255: An error occurred determining the plan.

` + c.GetExample() + c.Flags().Help())
}

// Synopsis satisfies the Synopsis function of the cli.Command interface.
func (c *PlanCommand) Synopsis() string {
	return "Dry-run a pack update to determine its effects"
}
