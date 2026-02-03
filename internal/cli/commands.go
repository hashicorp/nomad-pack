// Copyright IBM Corp. 2021, 2025
// SPDX-License-Identifier: MPL-2.0

package cli

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"runtime"
	"strconv"

	"github.com/hashicorp/go-hclog"
	"github.com/hashicorp/nomad/api"
	"github.com/posener/complete"

	"github.com/hashicorp/nomad-pack/internal/pkg/caching"
	flag "github.com/hashicorp/nomad-pack/internal/pkg/flag"
	"github.com/hashicorp/nomad-pack/internal/pkg/variable/envloader"
	"github.com/hashicorp/nomad-pack/terminal"
)

const (
	// EnvAllowUnsetVars is the env var that prevents errors from unset variables.
	EnvAllowUnsetVars = "NOMAD_PACK_ALLOW_UNSET_VARS"
)

// baseCommand is embedded in all commands to provide common logic and data.
//
// The unexported values are not available until after Init is called. Some
// values are only available in certain circumstances, read the documentation
// for the field to determine if that is the case.
type baseCommand struct {
	cmdKey string
	// Ctx is the base context for the command. It is up to commands to
	// utilize this context so that cancellation works in a timely manner.
	Ctx context.Context

	// Log is the logger to use.
	Log hclog.Logger

	// Example usage
	Example string

	//---------------------------------------------------------------
	// The fields below are only available after calling Init.

	// UI is used to write to the CLI.
	ui terminal.UI

	//---------------------------------------------------------------
	// Internal fields that should not be accessed directly

	// flagPlain is whether the output should be in plain mode.
	flagPlain bool

	// vars sets values for defined input variables
	vars map[string]string

	// envVars sets values for defined input variables from the environment
	envVars map[string]string

	// varFiles is an HCL file(s) setting one or more values
	// for defined input variables
	varFiles []string

	// allowUnsetVars suppresses errors from variables with nil values,
	// i.e. those that are not set and have no default
	allowUnsetVars bool

	// ignoreMissingVars determines whether variable overrides that do not correspond
	// to variables defined in the pack should be ignored or produce an error
	ignoreMissingVars bool

	// autoApproved is true when the user supplies the --auto-approve or -y flag
	autoApproved bool

	// deploymentName is the unique identifier of the deployed
	// instance of a specified pack. Used for running more than
	// one instance of a pack within the same cluster
	deploymentName string

	// useParserV1 is true when the user supplies the --parser-v1 flag
	useParserV1 bool

	// args that were present after parsing flags
	args []string

	// options passed in at the global level
	globalOptions []Option

	// The home directory that we loaded the nomad-pack config from
	// TODO: Implement config file
	// homeConfigPath string

	ExposeDocs bool

	// configuration struct to carry nomad client config values from flags.
	nomadConfig nomadConfig
}

func (c *baseCommand) Help() string {
	return helpText[c.cmdKey][1]
}

func (c *baseCommand) Synopsis() string {
	return helpText[c.cmdKey][0]
}

// Close cleans up any resources that the command created. This should be
// defered by any CLI command that embeds baseCommand in the Run command.
func (c *baseCommand) Close() error {
	// Close our UI if it implements it. The glint-based UI does for example
	// to finish up all the CLI output.
	if closer, ok := c.ui.(io.Closer); ok && closer != nil {
		closer.Close()
	}

	return nil
}

func (c *baseCommand) IsWindows() bool {
	return runtime.GOOS == "windows"
}

func (c *baseCommand) IsLinux() bool {
	return runtime.GOOS == "linux"
}

func (c *baseCommand) IsMac() bool {
	return runtime.GOOS == "darwin"
}

func (c *baseCommand) GetExample() string {
	if len(c.Example) > 0 {
		return "Examples:" + c.Example + "\n"
	}
	return ""
}

// Copied from waypoint/internal/cli/option.go
type baseConfig struct {
	Args              []string
	Flags             *flag.Sets
	Config            bool
	ConfigOptional    bool
	Client            bool
	AppTargetRequired bool
	UI                terminal.UI
	Validation        ValidationFn
}

type nomadConfig struct {
	address       string
	namespace     string
	region        string
	token         string
	tlsSkipVerify bool
	tlsServerName string
	caCert        string
	clientCert    string
	clientKey     string
}

// Init initializes the command by parsing flags, parsing the configuration,
// setting up the project, etc. You can control what is done by using the
// options.
//
// Init should be called FIRST within the Run function implementation. Many
// options will affect behavior of other functions that can be called later.
func (c *baseCommand) Init(opts ...Option) error {
	baseCfg := baseConfig{
		Config: true,
		Client: true,
	}

	for _, opt := range c.globalOptions {
		opt(&baseCfg)
	}

	for _, opt := range opts {
		opt(&baseCfg)
	}

	// Init our UI first so we can write output to the user immediately.
	ui := baseCfg.UI
	if ui == nil {
		ui = terminal.ConsoleUI(c.Ctx)
	}

	c.ui = ui

	// Parse flags
	err := baseCfg.Flags.Parse(baseCfg.Args)
	if err != nil {
		return err
	}
	c.args = baseCfg.Flags.Args()

	c.envVars = envloader.New().GetVarsFromEnv()

	// if no flag, check env vars
	if !c.allowUnsetVars {
		c.allowUnsetVars, _ = strconv.ParseBool(os.Getenv(EnvAllowUnsetVars))
	}

	// Do any validation after parsing
	if baseCfg.Validation != nil {
		err := baseCfg.Validation(c, c.args)
		if err != nil {
			return err
		}
	}

	// Reset the UI to plain if that was set
	if c.flagPlain {
		c.ui = terminal.NonInteractiveUI(c.Ctx)
	}

	// Perform the caching ensure, but skip if we are running the version
	// command.
	if c.cmdKey != "version" {
		return c.ensureCache()
	}

	return nil
}

func (c *baseCommand) ensureCache() error {
	// Creates global caching
	_, err := caching.NewCache(&caching.CacheConfig{
		Path:   caching.DefaultCachePath(),
		Logger: c.ui,
	})
	if err != nil {
		return err
	}

	return nil
}

// flagSet creates the flags for this command. The callback should be used
// to configure the set with your own custom options.
func (c *baseCommand) flagSet(bit flagSetBit, f func(*flag.Sets)) *flag.Sets {
	set := flag.NewSets()
	if bit&flagSetOperation != 0 {
		f := set.NewSet("Operation Options")
		f.StringSliceVarP(&flag.StringSliceVarP{
			StringSliceVar: &flag.StringSliceVar{
				Name:    "var-file",
				Target:  &c.varFiles,
				Default: make([]string, 0),
				Usage: `Specifies the path to a variable override file. This can
						be provided multiple times on a single command to result
						in a list of files.`,
				Completion: complete.PredictOr(complete.PredictFiles("*.var"), complete.PredictFiles("*.hcl")),
			},
			Shorthand: "f",
		})

		f.BoolVar(&flag.BoolVar{
			Name:    "allow-unset-vars",
			Target:  &c.allowUnsetVars,
			Default: false,
			Usage: fmt.Sprintf(`Suppress errors from unset variables without default values.
					Equivalent of setting the %s environment variable to "1" or "true".`,
				EnvAllowUnsetVars),
		})

		f.BoolVar(&flag.BoolVar{
			Name:    "ignore-missing-vars",
			Target:  &c.ignoreMissingVars,
			Default: false,
			Usage: `Determines whether override variables not present in the
					pack should be ignored or produce an error.`,
		})

		f.StringMapVar(&flag.StringMapVar{
			Name:    "var",
			Target:  &c.vars,
			Default: make(map[string]string),
			Usage: `Specifies single override variables in the form of HCL
					syntax and can be specified multiple times per command.`,
		})

		f.StringVar(&flag.StringVar{
			Name:    "name",
			Target:  &c.deploymentName,
			Default: "",
			Usage: `If set, this will be the unique identifier of this deployed
					instance of the specified pack. If not set, the pack name
					will be used. This is useful for running more than one
					instance of a pack within the same cluster. Note that this
					name must be globally unique within a cluster. Running the
					run command multiple times with the same name, will just
					re-submit the same pack, and apply changes if you have made
					any to the underlying pack. Be mindful that, whether you
					have made changes or not, the underlying Allocations will be
					replaced. When managing packs, the name specified here is
					the name that should be passed to nomad-pack's plan and
					destroy commands.`,
		})

		f.BoolVar(&flag.BoolVar{
			Name:    "parser-v1",
			Target:  &c.useParserV1,
			Default: false,
			Usage: `Use the legacy syntax parser to parse your job. This
			enables pack to run packs for earlier versions while you are
			migrating them to the new syntax`,
		})
	}
	if bit&flagSetNeedsApproval != 0 {
		f := set.NewSet("Approval Options")
		f.BoolVarP(&flag.BoolVarP{
			BoolVar: &flag.BoolVar{
				Name:    "auto-approve",
				Target:  &c.autoApproved,
				Default: false,
				Usage: `Automatically answer confirmation prompts in the
						affirmative.`,
			},
			Shorthand: "y",
		})
	}

	if bit&flagSetNomadClient != 0 {
		f := set.NewSet("Nomad Cluster Options")
		f.StringVar(&flag.StringVar{
			Name:    "address",
			Target:  &c.nomadConfig.address,
			Default: "",
			Usage: `The address of the Nomad server.
					Overrides the NOMAD_ADDR environment variable if set.`,
		})

		f.StringVar(&flag.StringVar{
			Name:    "namespace",
			Target:  &c.nomadConfig.namespace,
			Default: "",
			Usage: `The target namespace for queries and actions bound to a namespace.
					Overrides the NOMAD_NAMESPACE environment variable if set.`,
		})

		f.StringVar(&flag.StringVar{
			Name:    "region",
			Target:  &c.nomadConfig.region,
			Default: "",
			Usage: `The region of the Nomad servers to forward commands to.
					Overrides the NOMAD_REGION environment variable if set.`,
		})

		f.StringVar(&flag.StringVar{
			Name:    "ca-cert",
			Target:  &c.nomadConfig.caCert,
			Default: "",
			Usage: `Path to a PEM encoded CA cert file to use to verify the
					Nomad server SSL certificate. Overrides the NOMAD_CACERT
					environment variable if set.`,
		})

		f.StringVar(&flag.StringVar{
			Name:    "client-cert",
			Target:  &c.nomadConfig.clientCert,
			Default: "",
			Usage: `Path to a PEM encoded client certificate for TLS authentication
					to the Nomad server. Must also specify --client-key. Overrides
					the NOMAD_CLIENT_CERT environment variable if set.`,
		})

		f.StringVar(&flag.StringVar{
			Name:    "client-key",
			Target:  &c.nomadConfig.clientKey,
			Default: "",
			Usage: `Path to an unencrypted PEM encoded private key matching the
					client certificate from --client-cert. Overrides the
					NOMAD_CLIENT_KEY environment variable if set.`,
		})

		f.StringVar(&flag.StringVar{
			Name:    "tls-server-name",
			Target:  &c.nomadConfig.tlsServerName,
			Default: "",
			Usage: `The server name to use as the SNI host when connecting via
					TLS. Overrides the NOMAD_TLS_SERVER_NAME environment variable
					if set.`,
		})

		f.StringVar(&flag.StringVar{
			Name:    "token",
			Target:  &c.nomadConfig.token,
			Default: "",
			Usage: `The SecretID of an ACL token to use to authenticate API requests with.
					Overrides the NOMAD_TOKEN environment variable if set.`,
		})

		f.BoolVarP(&flag.BoolVarP{
			BoolVar: &flag.BoolVar{
				Name:    "tls-skip-verify",
				Target:  &c.nomadConfig.tlsSkipVerify,
				Default: false,
				Usage: `Do not verify TLS certificate. This is highly not recommended.
						Verification will also be skipped if NOMAD_SKIP_VERIFY is set.`,
			},
		})
	}

	if f != nil {
		// Configure our values
		f(set)
	}

	return set
}

// Returns minimal help usage message
// Used on flag/arg parse error in c.Init method
func (c *baseCommand) helpUsageMessage() string {
	if c.cmdKey == "" {
		return `See "nomad-pack --help"`
	}
	return fmt.Sprintf(`See "nomad-pack %s --help"`, c.cmdKey)
}

// flagSetBit is used with baseCommand.flagSet
type flagSetBit uint

const (
	flagSetNone          flagSetBit = 1 << iota // nolint:deadcode,varcheck,unused
	flagSetOperation                            // shared flags for operations (run, plan, etc)
	flagSetNeedsApproval                        // adds the -y flag for commands that require approval to run
	flagSetNomadClient                          // adds client config flags
)

var (
	// ErrSentinel is a sentinel value that we can return from Init to force an exit.
	ErrSentinel = errors.New("error sentinel")

	// ErrParsingArgsOrFlags should be used in the Init method of a CLI command
	// if it returns an error.
	ErrParsingArgsOrFlags = "error parsing args or flags"
)

func (c *baseCommand) getAPIClient() (*api.Client, error) {
	return api.NewClient(clientOptsFromCLI(c))
}
