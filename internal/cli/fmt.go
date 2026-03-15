// Copyright IBM Corp. 2023, 2026
// SPDX-License-Identifier: MPL-2.0

package cli

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/hashicorp/hcl/v2/hclwrite"
	"github.com/hashicorp/nomad-pack/internal/pkg/flag"
	"github.com/posener/complete"
)

type FmtCommand struct {
	*baseCommand
	check     bool
	list      bool
	write     bool
	recursive bool
}

func (c *FmtCommand) Run(args []string) int {
	c.cmdKey = "fmt"

	if err := c.Init(
		WithArgs(args),
		WithFlags(c.Flags()),
		WithNoConfig(),
	); err != nil {
		c.ui.ErrorWithContext(err, ErrParsingArgsOrFlags)
		c.ui.Info(c.helpUsageMessage())
		return 1
	}

	paths := c.args
	if len(paths) == 0 {
		paths = []string{"."}
	}

	return c.fmt(paths)
}

func (c *FmtCommand) fmt(paths []string) int {
	var exitCode int
	var filesToFormat []string

	for _, path := range paths {
		info, err := os.Stat(path)
		if err != nil {
			c.ui.Error(fmt.Sprintf("Error accessing %s: %v", path, err))
			return 1
		}

		if info.IsDir() {
			files, err := c.findFormattableFiles(path)
			if err != nil {
				c.ui.Error(fmt.Sprintf("Error scanning directory: %v", err))
				return 1
			}
			filesToFormat = append(filesToFormat, files...)
		} else if c.isFormattableFile(path) {
			filesToFormat = append(filesToFormat, path)
		}
	}

	if len(filesToFormat) == 0 {
		c.ui.Info("No formattable files (.tpl,.hcl) found")
		return 0
	}

	for _, file := range filesToFormat {
		formatted, err := c.formatFile(file)
		if err != nil {
			c.ui.Error(fmt.Sprintf("Error formatting %s: %v", file, err))
			exitCode = 1
			continue
		}

		if formatted {
			if !c.write && c.list {
				c.ui.Output(file)
			}
			if c.check {
				exitCode = 1
			}
		}
	}

	if c.check && exitCode == 1 {
		c.ui.Error("Some files are not formatted. Run 'nomad-pack fmt' to format them.")
	}

	return exitCode
}

func (c *FmtCommand) formatFile(path string) (bool, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		return false, err
	}
	var formatted string
	switch {
	case strings.HasSuffix(path, ".tpl"):
		formatted, err = c.formatTemplate(string(content))
	case strings.HasSuffix(path, ".hcl"):
		formatted, err = c.formatHCL(string(content))
	default:
		return false, fmt.Errorf("unsupported file type: %s", path)
	}
	if err != nil {
		return false, err
	}
	changed := string(content) != formatted

	if changed && c.write && !c.check {
		err = os.WriteFile(path, []byte(formatted), 0644)
		if err != nil {
			return false, err
		}
		c.ui.Output(fmt.Sprintf("Formatted:%s", path))
	}
	return changed, nil
}

func (c *FmtCommand) formatTemplate(content string) (string, error) {
	placeholders := make(map[string]string)
	placeholderIndex := 0

	templateRegex := regexp.MustCompile(`\[\[.*?\]\]`)

	contentWithPlaceholders := templateRegex.ReplaceAllStringFunc(content, func(match string) string {
		placeholder := fmt.Sprintf("__NOMAD_PACK_TPL_%d__", placeholderIndex)
		placeholders[placeholder] = match
		placeholderIndex++
		return placeholder
	})

	formatted := hclwrite.Format([]byte(contentWithPlaceholders))
	formattedStr := string(formatted)

	for placeholder, original := range placeholders {
		formattedStr = strings.ReplaceAll(formattedStr, placeholder, original)
	}

	return formattedStr, nil
}

func (c *FmtCommand) formatHCL(content string) (string, error) {
	formatted := hclwrite.Format([]byte(content))
	return string(formatted), nil
}

func (c *FmtCommand) findFormattableFiles(dir string) ([]string, error) {
	var files []string
	cleanDir := filepath.Clean(dir)
	walkFunc := func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		if d.IsDir() && strings.HasPrefix(d.Name(), ".") {
			return filepath.SkipDir
		}
		if d.IsDir() && path != cleanDir && !c.recursive {
			return filepath.SkipDir
		}
		if !d.IsDir() && c.isFormattableFile(path) {
			files = append(files, path)
		}

		return nil
	}

	err := filepath.WalkDir(cleanDir, walkFunc)
	return files, err
}

func (c *FmtCommand) isFormattableFile(path string) bool {
	return strings.HasSuffix(path, ".tpl") ||
		strings.HasSuffix(path, ".hcl")
}

func (c *FmtCommand) Flags() *flag.Sets {
	return c.flagSet(0, func(set *flag.Sets) {
		f := set.NewSet("Format Options")

		f.BoolVar(&flag.BoolVar{
			Name:    "check",
			Target:  &c.check,
			Default: false,
			Usage:   "Check if files are formatted without modifying them. Returns exit code 1 if formatting is needed.",
		})

		f.BoolVar(&flag.BoolVar{
			Name:    "list",
			Target:  &c.list,
			Default: true,
			Usage:   "List files that would be modified.",
		})

		f.BoolVar(&flag.BoolVar{
			Name:    "write",
			Target:  &c.write,
			Default: true,
			Usage:   "Write formatted content back to files.",
		})

		f.BoolVar(&flag.BoolVar{
			Name:    "recursive",
			Target:  &c.recursive,
			Default: false,
			Usage:   "Process directories recursively.",
		})
	})
}

func (c *FmtCommand) AutocompleteArgs() complete.Predictor {
	return complete.PredictOr(
		complete.PredictFiles("*.tpl"),
		complete.PredictFiles("*.hcl"),
	)
}

func (c *FmtCommand) AutocompleteFlags() complete.Flags {
	return c.Flags().Completions()
}
func (c *FmtCommand) Synopsis() string {
	return "Format pack template and HCL files"
}

func (c *FmtCommand) Help() string {
	return formatHelp(`
Usage: nomad-pack fmt [options] [path...]

  Format pack files (.tpl templates and .hcl files) using HCL formatting rules.
  Template files preserve the [[ ]] syntax, while .hcl files are formatted directly.

  If no path is given, the current directory is used.

Format Options:

  -check
    Check if files are formatted without modifying them.
    Returns exit code 1 if formatting is needed.

  -list
    List files that would be modified (default: true).

  -write
    Write formatted content back to files (default: true).

  -recursive
    Process directories recursively.

Examples:

  Format all templates in current directory:
  $ "nomad-pack fmt"

  Format a specific pack directory:
  $ "nomad-pack fmt my-pack/"

  Check formatting without modifying:
  $ "nomad-pack fmt -check ."
`)
}
