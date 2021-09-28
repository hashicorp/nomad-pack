package cli

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"text/tabwriter"

	"github.com/hashicorp/go-hclog"
	"github.com/mitchellh/cli"
	"github.com/mitchellh/go-glint"

	flag "github.com/hashicorp/nom/flag"
	"github.com/hashicorp/nom/internal/pkg/version"
)

const (
	// EnvLogLevel is the env var to set with the log level.
	EnvLogLevel = "NOMAD_PACK_LOG_LEVEL"

	// EnvPlain is the env var that can be set to force plain output mode.
	EnvPlain = "NOMAD_PACK_PLAIN"
)

var (
	// cliName is the name of this CLI.
	cliName = "nomad-pack"

	// commonCommands are the commands that are deemed "common" and shown first
	// in the CLI help output.
	commonCommands = []string{
		"init",
		"plan",
		"render",
		"run",
		"destroy",
	}
)

// Main runs the CLI with the given arguments and returns the exit code.
// The arguments SHOULD include argv[0] as the program name.
func Main(args []string) int {
	// NOTE: This is only for running `nomad-pack -v` and expecting it to return
	// a version. Any other subcommand will expect `-v` to be around verbose
	// logging rather than printing a version
	if len(args) == 2 && args[1] == "-v" {
		args[1] = "--version"
	}

	// Initialize our logger based on env vars
	args, log, logOutput, err := logger(args)
	if err != nil {
		panic(err)
	}

	// Log our versions
	log.Info("Nomad Pack version",
		"version", version.HumanVersion(),
	)

	// Build our cancellation context
	ctx, closer := WithInterrupt(context.Background(), log)
	defer closer()

	// Get our base command
	fset := flag.NewSets()
	base, commands := Commands(ctx, log, logOutput, WithFlags((fset)))
	defer base.Close()

	// Build the CLI. We use a
	//
	//CLI factory function because to modify the
	// args once you call a func on CLI you need to create a new CLI instance.
	cliFactory := func() *cli.CLI {
		return &cli.CLI{
			Name:                       args[0],
			Args:                       args[1:],
			Version:                    version.HumanVersion(),
			Commands:                   commands,
			Autocomplete:               true,
			AutocompleteNoDefaultFlags: true,
			HelpFunc:                   GroupedHelpFunc(cli.BasicHelpFunc(cliName)),
		}
	}

	// Copy the CLI to check if it is a version call. If so, we modify
	// the args to just be the version subcommand. This ensures that
	// --version behaves by calling `nomad-pack version` and we get consistent
	// behavior.
	cli := cliFactory()
	if cli.IsVersion() {
		// We need to re-init because you can't modify fields after calling funcs
		cli = cliFactory()
		cli.Args = []string{"--version"}
	}

	// Run the CLI
	exitCode, err := cli.Run()
	if err != nil {
		panic(err)
	}

	return exitCode
}

// commands returns the map of commands that can be used to initialize a CLI.
func Commands(
	ctx context.Context,
	log hclog.Logger,
	logOutput io.Writer,
	opts ...Option,
) (*baseCommand, map[string]cli.CommandFactory) {
	baseCommand := &baseCommand{
		Ctx:           ctx,
		Log:           log,
		LogOutput:     logOutput,
		globalOptions: opts,
	}

	// aliases is a list of command aliases we have. The key is the CLI
	// command (the alias) and the value is the existing target command.
	// aliases := map[string]string{
	// 	"build":   "artifact build",
	// 	"deploy":  "deployment deploy",
	// 	"install": "server install",
	// }

	// start building our commands
	commands := map[string]cli.CommandFactory{
		"init": func() (cli.Command, error) {
			return &InitCommand{
				baseCommand: baseCommand,
			}, nil
		},
		"repo": func() (cli.Command, error) {
			return &helpCommand{
				synopsis: helpText["repo"][0],
				help:     helpText["repo"][1],
			}, nil
		},
		"list": func() (cli.Command, error) {
			return &ListCommand{
				baseCommand: baseCommand,
			}, nil
		},
		"render": func() (cli.Command, error) {
			return &RenderCommand{
				baseCommand: baseCommand,
			}, nil
		},
		"run": func() (cli.Command, error) {
			return &RunCommand{
				baseCommand: baseCommand,
			}, nil
		},
		"version": func() (cli.Command, error) {
			return &VersionCommand{
				baseCommand: baseCommand,
			}, nil
		},
		"plan": func() (cli.Command, error) {
			return &PlanCommand{
				baseCommand: baseCommand,
			}, nil
		},
		"destroy": func() (cli.Command, error) {
			return &DestroyCommand{
				baseCommand: baseCommand,
			}, nil
		},
	}
	return baseCommand, commands
}

// logger returns the logger to use for the CLI. Output, level, etc. are
// determined based on environment variables if set.
func logger(args []string) ([]string, hclog.Logger, io.Writer, error) {
	app := args[0]

	// Determine our log level if we have any. First override we check if env var
	level := hclog.NoLevel
	if v := os.Getenv(EnvLogLevel); v != "" {
		level = hclog.LevelFromString(v)
		if level == hclog.NoLevel {
			return nil, nil, nil, fmt.Errorf("%s value %q is not a valid log level", EnvLogLevel, v)
		}
	}

	// ProcessTemplates arguments looking for `-v` flags to control the log level.
	// This overrides whatever the env var set.
	// TODO this conflicts with using -v as shorthand for verbose
	// commenting most of this out for now; come back to later maybe?
	var outArgs []string
	for _, arg := range args {
		// if len(arg) != 0 && arg[0] != '-' {
		outArgs = append(outArgs, arg)
		continue
		// }

		// switch arg {
		// case "-v":
		// 	if level == hclog.NoLevel || level > hclog.Info {
		// 		level = hclog.Info
		// 	}
		// case "-vv":
		// 	if level == hclog.NoLevel || level > hclog.Debug {
		// 		level = hclog.Debug
		// 	}
		// case "-vvv":
		// 	if level == hclog.NoLevel || level > hclog.Trace {
		// 		level = hclog.Trace
		// 	}
		// default:
		// 	outArgs = append(outArgs, arg)
		// }
	}

	// Default output is nowhere unless we enable logging.
	var output io.Writer = ioutil.Discard
	color := hclog.ColorOff
	if level != hclog.NoLevel {
		output = os.Stderr
		color = hclog.AutoColor
	}

	logger := hclog.New(&hclog.LoggerOptions{
		Name:   app,
		Level:  level,
		Color:  color,
		Output: output,
	})

	return outArgs, logger, output, nil
}

func GroupedHelpFunc(f cli.HelpFunc) cli.HelpFunc {
	return func(commands map[string]cli.CommandFactory) string {
		var buf bytes.Buffer
		d := glint.New()
		d.SetRenderer(&glint.TerminalRenderer{
			Output: &buf,

			// We set rows/cols here manually. The important bit is the cols
			// needs to be wide enough so glint doesn't clamp any text and
			// lets the terminal just autowrap it. Rows doesn't make a big
			// difference.
			Rows: 10,
			Cols: 180,
		})

		// Header
		d.Append(glint.Style(
			glint.Text("Welcome to Nomad Pack"),
			glint.Bold(),
		))
		d.Append(glint.Layout(
			glint.Style(
				glint.Text("Docs:"),
				glint.Color("lightBlue"),
			),
			glint.Text(" "),
		).Row())
		d.Append(glint.Layout(
			glint.Style(
				glint.Text("Version:"),
				glint.Color("green"),
			),
			glint.Text(" "),
			glint.Text(version.HumanVersion()),
		).Row())
		d.Append(glint.Text(""))

		// Usage
		d.Append(glint.Layout(
			glint.Style(
				glint.Text("Usage:"),
				glint.Color("lightMagenta"),
			),
			glint.Text(" "),
			glint.Text(cliName),
			glint.Text(" "),
			glint.Text("[--version] [--help] [--autocomplete-(un)install] <command> [args]"),
		).Row())
		d.Append(glint.Text(""))

		// Add common commands
		helpCommandsSection(d, "Common commands", commonCommands, commands)

		// // Make our other commands
		// ignoreMap := map[string]struct{}{}
		// for k := range hiddenCommands {
		// 	ignoreMap[k] = struct{}{}
		// }
		// for _, k := range commonCommands {
		// 	ignoreMap[k] = struct{}{}
		// }

		// var otherCommands []string
		// for k := range commands {
		// 	if _, ok := ignoreMap[k]; ok {
		// 		continue
		// 	}

		// 	otherCommands = append(otherCommands, k)
		// }
		// sort.Strings(otherCommands)

		// // Add other commands
		// helpCommandsSection(d, "Other commands", otherCommands, commands)

		d.RenderFrame()
		return buf.String()
	}
}

func helpCommandsSection(
	d *glint.Document,
	header string,
	commands []string,
	factories map[string]cli.CommandFactory,
) {
	// Header
	d.Append(glint.Style(
		glint.Text(header),
		glint.Bold(),
	))

	// Build our commands
	var b bytes.Buffer
	tw := tabwriter.NewWriter(&b, 0, 2, 6, ' ', 0)
	for _, k := range commands {
		fn, ok := factories[k]
		if !ok {
			continue
		}

		cmd, err := fn()
		if err != nil {
			panic(fmt.Sprintf("failed to load %q command: %s", k, err))
		}

		fmt.Fprintf(tw, "%s\t%s\n", k, cmd.Synopsis())
	}
	tw.Flush()

	d.Append(glint.Layout(
		glint.Text(b.String()),
	).PaddingLeft(2))
}
