package cli

import (
	"bytes"
	"context"
	"fmt"
	"text/tabwriter"

	flag "github.com/hashicorp/nomad-pack/flag"
	"github.com/hashicorp/nomad-pack/internal/pkg/version"
	"github.com/mitchellh/cli"
	"github.com/mitchellh/go-glint"
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
		"info",
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

	// Build our cancellation context
	ctx, closer := WithInterrupt(context.Background())
	defer closer()

	// Get our base command
	fset := flag.NewSets()
	base, commands := Commands(ctx, WithFlags((fset)))
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

// Commands returns the map of commands that can be used to initialize a CLI.
func Commands(
	ctx context.Context,
	opts ...Option,
) (*baseCommand, map[string]cli.CommandFactory) {
	baseCommand := &baseCommand{
		Ctx:           ctx,
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
		"info": func() (cli.Command, error) {
			return &InfoCommand{
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
