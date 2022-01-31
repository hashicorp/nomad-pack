package cli

import (
	v1 "github.com/hashicorp/nomad-openapi/v1"
	"github.com/hashicorp/nomad-pack/internal/pkg/cache"
	"github.com/hashicorp/nomad-pack/internal/pkg/errors"
	"github.com/hashicorp/nomad-pack/internal/pkg/flag"
	"github.com/hashicorp/nomad-pack/internal/runner"
	"github.com/hashicorp/nomad-pack/internal/runner/job"
	"github.com/posener/complete"
)

type PlanCommand struct {
	*baseCommand
	packConfig *cache.PackConfig
	jobConfig  *job.CLIConfig
}

func (c *PlanCommand) Run(args []string) int {
	var err error
	c.cmdKey = "plan" // Add cmdKey here to print out helpUsageMessage on Init error
	// Initialize. If we fail, we just exit since Init handles the UI.
	if err := c.Init(
		WithExactArgs(1, args),
		WithFlags(c.Flags()),
		WithNoConfig(),
	); err != nil {
		c.ui.ErrorWithContext(err, ErrParsingArgsOrFlags)
		c.ui.Info(c.helpUsageMessage())
		return 255
	}

	c.packConfig.Name = c.args[0]

	// Set the packConfig defaults if necessary and generate our UI error context.
	errorContext := initPackCommand(c.packConfig)

	// verify packs exist before planning jobs
	if err = cache.VerifyPackExists(c.packConfig, errorContext, c.ui); err != nil {
		return 255
	}

	// If no deploymentName set default to pack@ref
	c.deploymentName = getDeploymentName(c.baseCommand, c.packConfig)
	errorContext.Add(errors.UIContextPrefixDeploymentName, c.deploymentName)

	client, err := v1.NewClient()
	if err != nil {
		c.ui.ErrorWithContext(err, "failed to initialize client", errorContext.GetAll()...)
		return 255
	}

	packManager := generatePackManager(c.baseCommand, client, c.packConfig)

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
		PackName:       c.packConfig.Name,
		PathPath:       c.packConfig.Path,
		PackRef:        c.packConfig.Ref,
		DeploymentName: c.deploymentName,
		RegistryName:   c.packConfig.Registry,
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

	if canonicalizeErrs := jobRunner.CanonicalizeTemplates(); canonicalizeErrs != nil {
		for _, canonicalizeErr := range canonicalizeErrs {
			canonicalizeErr.Context.Append(errorContext)
			c.ui.ErrorWithContext(canonicalizeErr.Err, canonicalizeErr.Subject, canonicalizeErr.Context.GetAll()...)
		}
		return 1
	}

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
	c.packConfig = &cache.PackConfig{}

	return c.flagSet(flagSetOperation, func(set *flag.Sets) {
		f := set.NewSet("Plan Options")

		c.jobConfig = &job.CLIConfig{
			RunConfig:  &job.RunCLIConfig{},
			PlanConfig: &job.PlanCLIConfig{},
		}

		f.StringVar(&flag.StringVar{
			Name:    "registry",
			Target:  &c.packConfig.Registry,
			Default: "",
			Usage:   `Specific registry name containing the pack to be planned.`,
		})

		f.StringVar(&flag.StringVar{
			Name:    "ref",
			Target:  &c.packConfig.Ref,
			Default: "",
			Usage: `Specific git ref of the pack to be planned. 
Supports tags, SHA, and latest. If no ref is specified, defaults to 
latest.

Using ref with a file path is not supported.`,
		})

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
	# Plan an example pack with the default deployment name
	nomad-pack plan example

	# Plan an example pack at a specific ref
	nomad-pack plan example --ref=v0.0.1

	# Plan a pack from a registry other than the default registry
	nomad-pack plan traefik --registry=community --ref=v0.0.1

	# Plan an example pack without showing the diff
	nomad-pack plan example --diff=false

    # Plan a pack under development from the filesystem - supports current working 
    # directory or relative path
	nomad-pack plan . 
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
