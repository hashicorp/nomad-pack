package cli

import (
	"context"
	"errors"
	"fmt"
	"github.com/hashicorp/nomad-pack/internal/pkg/cache"
	"io"
	"os"
	"path"
	"runtime"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/mitchellh/go-wordwrap"
	"github.com/posener/complete"

	flag "github.com/hashicorp/nomad-pack/flag"
	"github.com/hashicorp/nomad-pack/terminal"
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

	// flagLabels are set via -label if flagSetOperation is set.
	flagLabels map[string]string

	// vars sets values for defined input variables
	vars map[string]string

	// varFiles is an HCL file(s) setting one or more values
	// for defined input variables
	varFiles []string

	// deploymentName is the unique identifier of the deployed
	// instance of a specified pack. Used for running more than
	// one instance of a pack within the same cluster
	deploymentName string

	// args that were present after parsing flags
	args []string

	// options passed in at the global level
	globalOptions []Option

	// The home directory that we loaded the waypoint config from
	homeConfigPath string
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

	// Perform the cache ensure, but skip if we are running the version
	// command.
	if c.cmdKey != "version" {
		return c.ensureCache()
	}

	return nil
}

func (c *baseCommand) ensureCache() error {
	// Creates global cache
	globalCache, err := cache.NewCache(&cache.CacheConfig{
		Path:   cache.DefaultCachePath(),
		Logger: c.ui,
	})
	if err != nil {
		return err
	}

	// Check if default registry exists
	_, err = os.Stat(path.Join(cache.DefaultCachePath(), cache.DefaultRegistryName))
	// If it does not error, then the registry already exists
	if err == nil {
		return nil
	}

	// Add the registry or registry target to the global cache
	_, err = globalCache.Add(&cache.AddOpts{
		RegistryName: cache.DefaultRegistryName,
		Source:       cache.DefaultRegistrySource,
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
	{
		// f := set.NewSet("Global Options")

		// f.BoolVar(&flag.BoolVar{
		// 	Name:    "plain",
		// 	Target:  &c.flagPlain,
		// 	Default: false,
		// 	Usage:   "Plain output: no colors, no animation.",
		// })

		// f.StringVar(&flag.StringVar{
		// 	Name:    "app",
		// 	Target:  &c.flagApp,
		// 	Default: "",
		// 	Usage: "App to target. Certain commands require a single app target for " +
		// 		"Waypoint configurations with multiple apps. If you have a single app, " +
		// 		"then this can be ignored.",
		// })

		// f.StringVar(&flag.StringVar{
		// 	Name:    "workspace",
		// 	Target:  &c.flagWorkspace,
		// 	Default: "default",
		// 	Usage:   "Workspace to operate in.",
		// })
	}
	if bit&flagSetOperation != 0 {
		f := set.NewSet("Operation Options")
		f.StringSliceVarP(&flag.StringSliceVarP{
			StringSliceVar: &flag.StringSliceVar{
				Name:    "var-file",
				Target:  &c.varFiles,
				Default: make([]string, 0),
				Usage: `Specifies the path to a variable override file. This can be provided 
				multiple times on a single command to result in a list of files.`,
				Completion: complete.PredictOr(complete.PredictFiles("*.var"), complete.PredictFiles("*.hcl")),
			},
			Shorthand: "f",
		})

		f.StringMapVar(&flag.StringMapVar{
			Name:    "var",
			Target:  &c.vars,
			Default: make(map[string]string),
			Usage: `Specifies single override variables in the form of HCL syntax and
				can be specified multiple times per command.`,
		})

		f.StringVar(&flag.StringVar{
			Name:    "name",
			Target:  &c.deploymentName,
			Default: "",
			Usage: `If set, this will be the unique identifier of this deployed
                      instance of the specified pack. If not set, the pack name will
                      be used. This is useful for running more than one instance
			of a pack within the same cluster. Note that this name
                      must be globally unique within a cluster. Running the run
                      command multiple times with the same name, will just re-submit
	                the same pack, and apply changes if you have made any to
                      the underlying pack. Be mindful that, whether you have made
                      changes or not, the underlying Allocations will be replaced. 
                      When managing packs, the name specified here is the name that
                      should be passed to the plan or destroy commands.
                      `,
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
	flagSetNone      flagSetBit = 1 << iota
	flagSetOperation            // shared flags for operations (run, plan, etc)
)

var (
	// ErrSentinel is a sentinel value that we can return from Init to force an exit.
	ErrSentinel = errors.New("error sentinel")

	// ErrParsingArgsOrFlags should be used in the Init method of a CLI command
	// if it returns an error.
	ErrParsingArgsOrFlags = "error parsing args or flags"
)

func Humanize(err error) string {
	if err == nil {
		return ""
	}

	if IsCanceled(err) {
		return "operation canceled"
	}

	v := err.Error()
	if s, ok := status.FromError(err); ok {
		v = s.Message()
	}

	return wordwrap.WrapString(v, 80)
}

// IsCanceled is true if the error represents a cancellation. This detects
// context cancellation as well as gRPC cancellation codes.
func IsCanceled(err error) bool {
	if err == context.Canceled {
		return true
	}

	s, ok := status.FromError(err)
	if !ok {
		return false
	}

	return s.Code() == codes.Canceled
}
