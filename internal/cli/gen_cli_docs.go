// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/hashicorp/go-multierror"
	"github.com/hashicorp/nomad-pack/internal/pkg/flag"
	"github.com/hashicorp/nomad-pack/internal/pkg/helper/filesystem"
	"github.com/mitchellh/cli"
)

type DocGenerateCommand struct {
	*baseCommand

	commands map[string]cli.CommandFactory
	aliases  map[string]string
	mode     string
}

func (c *DocGenerateCommand) Run(args []string) int {
	c.cmdKey = "gen-cli-docs"

	// Initialize. If we fail, we just exit since Init handles the UI.
	if err := c.Init(
		WithExactArgs(1, args),
	); err != nil {
		c.ui.ErrorWithContext(err, "error parsing args or flags")
		return 1
	}

	c.mode = args[0]
	var mErr *multierror.Error
	mErr = multierror.Append(mErr, filesystem.MaybeCreateDestinationDir("./website/content/commands"))
	mErr = multierror.Append(mErr, filesystem.MaybeCreateDestinationDir("./website/data/"))

	if c.mode == "mdx" {
		mErr = multierror.Append(mErr, filesystem.MaybeCreateDestinationDir("./website/content/partials/commands"))
	}

	if mErr != nil && mErr.Len() > 0 {
		c.Log.Error("error making dirs", "error", mErr)
		return 1
	}

	commands := map[string]string{}

	var keys []string

	for k, fact := range c.commands {
		cmd, err := fact()
		if err != nil {
			c.Log.Error("error creating command", "error", err, "command", k)
			return 1
		}

		if _, ok := cmd.(*helpCommand); ok {
			continue
		}

		err = c.genDocs(k, cmd)
		if err != nil {
			c.Log.Error("error generating docs", "error", err, "command", k)
			return 1
		}

		commands[k] = cmd.Synopsis()
		keys = append(keys, k)
	}

	sort.Strings(keys)

	if c.mode == "mdx" {
		w, err := os.Create("./website/content/partials/commands/command-list.mdx")
		if err != nil {
			c.Log.Error("error creating index page", "error", err)
			return 1
		}
		defer w.Close()
	}

	contentMap, err := os.Create("./website/data/commands-nav-data.json")
	if err != nil {
		c.Log.Error("error creating nav-data page", "error", err)
		return 1
	}
	defer contentMap.Close()

	var sb strings.Builder
	var offset = int(0)
	sb.WriteString("[")
	for i, k := range keys {
		if k == "gen-cli-docs" {
			offset += 1
			continue
		}
		if i-offset > 0 {
			sb.WriteString(",\n")
		}
		sb.WriteString(fmt.Sprintf(`{"title":%q,"path":%q}`, k, cleanName(k)))
	}
	sb.WriteString("\n]")
	_, err = contentMap.WriteString(sb.String())
	if err != nil {
		fmt.Println(fmt.Errorf("docgen error: %w", err))
		return 1
	}

	return 0
}

type HasFlags interface {
	Flags() *flag.Sets
}

func cleanName(name string) string {
	return strings.ReplaceAll(name, " ", "-")
}

func (c *DocGenerateCommand) genDocs(name string, cmd cli.Command) error {
	if name == "gen-cli-docs" {
		return nil
	}

	fmt.Printf("=> %s\n", name)
	goodName := cleanName(name)
	path := filepath.Join("./website", "content", "commands", goodName) + "." + c.mode

	w, err := os.Create(path)
	if err != nil {
		return err
	}

	defer w.Close()

	capital := strings.ToUpper(string(name[0])) + name[1:]

	fmt.Fprintf(w, `---
layout: commands
page_title: "Commands: %s"
sidebar_title: "%s"
description: "%s"
---

`, capital, name, cmd.Synopsis())

	fmt.Fprintf(w, "# Nomad-Pack %s\n\nCommand: `nomad-pack %s`\n\n%s\n\n", capital, name, cmd.Synopsis())

	if c.mode == "mdx" {
		descFile := goodName + "_desc.mdx"
		fmt.Fprintf(w, "@include \"commands/%s\"\n\n", descFile)
		err = c.touch("./website/content/partials/commands/" + descFile)
		if err != nil {
			return err
		}
	}

	if hf, ok := cmd.(HasFlags); ok {
		flags := hf.Flags()

		// Generate the Usage headers based on the cmd Help text
		helpText := strings.Split(cmd.Help(), "\n")
		usage := helpText[0]

		var optionalAlias string
		if len(helpText) > 1 {
			optionalAlias = helpText[1]
		}

		reUsage := regexp.MustCompile(`nomad-pack (?P<cmd>.*)$`)
		reAlias := regexp.MustCompile(`Alias: `)

		matches := reUsage.FindStringSubmatch(usage)

		if len(matches) > 0 {
			fmt.Fprintf(w, "## Usage\n\nUsage: `nomad-pack %s`\n", matches[1])

			hasAlias := false
			if optionalAlias != "" {
				matchAlias := reAlias.FindStringSubmatch(optionalAlias)
				if len(matchAlias) > 0 {
					hasAlias = true
					aliasMatch := reUsage.FindStringSubmatch(optionalAlias)
					fmt.Fprintf(w, "\nAlias: `nomad-pack %s`\n", aliasMatch[1])
				}
			}

			// Don't include flag options, we do this later. We trim it here because
			// most commands include it in the help text.
			reOptions := regexp.MustCompile(` Options:`)
			optionsIndex := 0
			for i, opt := range helpText {
				optMatch := reOptions.FindStringSubmatch(opt)
				if len(optMatch) > 0 {
					optionsIndex = i
					break
				}
			}

			if optionsIndex > 1 {
				// Assume all commands have at least "Global Options"
				startIndex := 1
				helpMsg := ""

				if hasAlias {
					startIndex = 2
				}

				helpMsg = strings.Join(helpText[startIndex:optionsIndex], "\n")

				// Strip any color formatting
				reAsciColor := regexp.MustCompile(ansi)
				helpMsg = reAsciColor.ReplaceAllString(helpMsg, "")

				// Trim any left leading whitespace, if any. We do this because any
				// chunk of text that's indented in markdown will be rendered as a
				// code block rather than a paragraph of text.
				helpMsg = strings.TrimLeft(helpMsg, " ")
				fmt.Fprintf(w, "\n%s", helpMsg)
			}
		} else {
			// TODO: Fix comment
			// Fail over to simple docs gen. These are for top level commands
			// like `nomad-pack context` that don't work without a subcommand and fail the regex match.
			fmt.Fprintf(w, "## Usage\n\nUsage: `nomad-pack %s [options]`\n", name)
		}

		// Generate flag options
		flags.VisitSets(func(name string, set *flag.Set) {
			// Only print a set if it contains vars
			numVars := 0
			set.VisitVars(func(f *flag.VarFlagP) { numVars++ })
			if numVars == 0 {
				return
			}

			fmt.Fprintf(w, "\n#### %s\n\n", name)

			set.VisitVars(func(f *flag.VarFlagP) {
				if h, ok := f.Value.(flag.FlagVisibility); ok && h.Hidden() {
					return
				}

				name := f.Name
				if t, ok := f.Value.(flag.FlagExample); ok {
					example := t.Example()
					if example != "" {
						name += "=<" + example + ">"
					}
				}

				if len(f.Aliases) > 0 {
					aliases := strings.Join(f.Aliases, "`, `-")

					fmt.Fprintf(w, "- `-%s` (`-%s`) - %s\n", name, aliases, f.Usage)
				} else {
					fmt.Fprintf(w, "- `-%s` - %s\n", name, f.Usage)
				}
			})
		})
	} else {
		fmt.Printf("  ! has no flags\n")
	}

	if c.mode == "mdx" {
		moreFile := goodName + "_more.mdx"
		fmt.Fprintf(w, "\n@include \"commands/%s\"\n", moreFile)
		err = c.touch("./website/content/partials/commands/" + moreFile)
		if err != nil {
			return err
		}
	}

	return nil
}

func (c *DocGenerateCommand) touch(name string) error {
	f, err := os.OpenFile(name, os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}

	f.Close()

	return nil
}

const (
	// NOTE: adapted from https://github.com/acarl005/stripansi
	ansi = "[\u001B\u009B][[\\]()#;?]*(?:(?:(?:[a-zA-Z\\d]*(?:;[a-zA-Z\\d]*)*)?\u0007)|(?:(?:\\d{1,4}(?:;\\d{0,4})*)?[\\dA-PRZcf-ntqry=><~]))"
)
