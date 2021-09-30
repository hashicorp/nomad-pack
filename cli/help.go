package cli

import (
	"bytes"
	"regexp"
	"strings"

	"github.com/mitchellh/cli"
	"github.com/mitchellh/go-glint"
)

// formatHelp takes a raw help string and attempts to colorize it automatically.
func formatHelp(v string) string {
	// Trim the empty space
	v = strings.TrimSpace(v)

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

	// seenHeader is flipped to true once we see any reHelpHeader match.
	seenHeader := false

	for _, line := range strings.Split(v, "\n") {
		// Usage: prefix lines
		prefix := "Usage: "
		if strings.HasPrefix(line, prefix) {
			d.Append(glint.Layout(
				glint.Style(
					glint.Text(prefix),
					glint.Color("lightMagenta"),
				),
				glint.Text(line[len(prefix):]),
			).Row())

			continue
		}

		// Alias: prefix lines
		prefix = "Alias: "
		if strings.HasPrefix(line, prefix) {
			d.Append(glint.Layout(
				glint.Style(
					glint.Text(prefix),
					glint.Color("lightMagenta"),
				),
				glint.Text(line[len(prefix):]),
			).Row())

			continue
		}

		// Example: prefix lines
		prefix = "Examples:"
		if strings.HasPrefix(line, prefix) {
			d.Append(glint.Layout(
				glint.Style(
					glint.Text(prefix),
					glint.Color("lightMagenta"),
				),
				glint.Text(line[len(prefix):]),
			).Row())

			continue
		}

		// A header line
		if reHelpHeader.MatchString(line) {
			seenHeader = true

			d.Append(glint.Style(
				glint.Text(line),
				glint.Bold(),
			))

			continue
		}

		// If we have a command in the line, then highlight that.
		if matches := reCommand.FindAllStringIndex(line, -1); len(matches) > 0 {
			var cs []glint.Component
			idx := 0
			for _, match := range matches {
				start := match[0] + 1
				end := match[1] - 1

				cs = append(
					cs,
					glint.Text(line[idx:start]),
					glint.Style(
						glint.Text(line[start:end]),
						glint.Color("lightMagenta"),
					),
				)

				idx = end
			}

			// Add the rest of the text
			cs = append(cs, glint.Text(line[idx:]))

			d.Append(glint.Layout(cs...).Row())
			continue
		}

		// The styles in this block we only want to apply before any headers.
		if !seenHeader {
			// If we have a flag in the line, then highlight that.
			if matches := reFlag.FindAllStringSubmatchIndex(line, -1); len(matches) > 0 {
				const matchGroup = 2 // the subgroup that has the actual flag

				var cs []glint.Component
				idx := 0
				for _, match := range matches {
					start := match[matchGroup*2]
					end := match[matchGroup*2+1]

					cs = append(
						cs,
						glint.Text(line[idx:start]),
						glint.Style(
							glint.Text(line[start:end]),
							glint.Color("lightMagenta"),
						),
					)

					idx = end
				}

				// Add the rest of the text
				cs = append(cs, glint.Text(line[idx:]))

				d.Append(glint.Layout(cs...).Row())
				continue
			}
		}

		// Normal line
		d.Append(glint.Text(line))
	}

	d.RenderFrame()
	return buf.String()
}

type helpCommand struct {
	synopsis string
	help     string
}

func (c *helpCommand) Run(args []string) int {
	return cli.RunResultHelp
}

func (c *helpCommand) Synopsis() string {
	return strings.TrimSpace(c.synopsis)
}

func (c *helpCommand) Help() string {
	if c.help == "" {
		return c.synopsis
	}

	return formatHelp(c.help)
}

func (c *helpCommand) HelpTemplate() string {
	return formatHelp(helpTemplate)
}

var (
	reHelpHeader = regexp.MustCompile(`^[a-zA-Z0-9_-].*:$`)
	reCommand    = regexp.MustCompile(`"nomad-pack (\w\s?)+"`)
	reFlag       = regexp.MustCompile(`(\s|^|")(-[\w-]+)(\s|$|"|=)`)
)

const helpTemplate = `
Usage: {{.Name}} {{.SubcommandName}} SUBCOMMAND

{{indent 2 (trim .Help)}}{{if gt (len .Subcommands) 0}}

Subcommands:
{{- range $value := .Subcommands }}
    {{ $value.NameAligned }}    {{ $value.Synopsis }}{{ end }}

{{- end }}
`

var helpText = map[string][2]string{
	"repo": {
		"Nomad Pack git repository management",
		`
Nomad Pack git repository management.
The repo commands can be used to list repositories containing Nomad packs, push
updates to a repository you control, add a new repository to your local cache, etc.
Nomad Pack will search for packs in local repositories to match the pack name(s) specified
in the run command.
`,
	},
	"repo list": {
		"List local Nomad Pack repositories",
		`
List local Nomad Pack repositories.
The repo list command can be used to list local repositories containing Nomad packs.
Nomad Pack will search for packs in local repositories to match the pack name(s) specified
in the run command.
`,
	},
	"run": {
		"Run one or more Nomad packs",
		`
Run one or more Nomad packs.
The run command is used to install a Nomad Pack to a configured Nomad cluster.
Nomad Pack will search for packs in local repositories to match the pack name(s) specified
in the run command.
`,
	},
	"plan": {
		"Plan invokes a dry-run of the scheduler to determine the effects of submitting either a new or updated version of a job",
		`
Plan invokes a dry-run of the scheduler to determine the effects of submitting
either a new or updated version of a job. The plan will not result in any changes 
to the cluster but gives insight into whether the job could be run successfully 
and how it would affect existing allocations.
`,
	},
	"destroy": {
		"Destroy stops a running job",
		`
Destroy stops a running job. Purge is used to stop the job and purge it from the system.
 If not set, the job will still be queryable and will be purged by the garbage collector. 
Global will stop a multi-region job in all its regions. By default, job stop will stop 
only a single region at a time. Ignored for single-region jobs. After the deregister 
command is submitted, a new evaluation ID is printed to the screen, which can be 
used to examine the evaluation.
`,
	},
	"info": {
		"Info gets information on a pack",
		`
Info reads from a pack's metadata.hcl and variables.hcl files and prints out the details
of a pack.
`,
	},
}
