// Copyright IBM Corp. 2023, 2026
// SPDX-License-Identifier: MPL-2.0

package cli

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/mitchellh/cli"
	"github.com/shoenig/test/must"

	flag "github.com/hashicorp/nomad-pack/internal/pkg/flag"
	"github.com/hashicorp/nomad-pack/internal/pkg/helper"
	"github.com/hashicorp/nomad-pack/internal/testui"
	"github.com/hashicorp/nomad/ci"
)

// ── Unit tests for formatTemplate() ──────────────────────────────────────────

// runPackCmdAllowErrors is like runPackCmd but does NOT assert that cmdErr is empty.
// Use this for tests where the command is expected to print errors.
func runPackCmdAllowErrors(t *testing.T, args []string) PackCommandResult {
	t.Helper()
	cmdOut := bytes.NewBuffer(make([]byte, 0))
	cmdErr := bytes.NewBuffer(make([]byte, 0))

	ctx, closer := helper.WithInterrupt(context.Background())
	defer closer()

	ui := testui.NonInteractiveTestUI(ctx, cmdOut, cmdErr)
	fset := flag.NewSets()
	base, commands := Commands(ctx, WithFlags(fset), WithUI(ui))
	defer base.Close()

	command := &cli.CLI{
		Name:                       "nomad-pack",
		Args:                       args,
		Commands:                   commands,
		Autocomplete:               true,
		AutocompleteNoDefaultFlags: true,
		HelpFunc:                   GroupedHelpFunc(cli.BasicHelpFunc(cliName)),
		HelpWriter:                 cmdOut,
		ErrorWriter:                cmdErr,
	}
	exitCode, err := command.Run()
	if err != nil {
		panic(err)
	}
	// NOTE: No cmdErr assertion here — we allow errors
	return PackCommandResult{exitCode: exitCode, cmdOut: cmdOut, cmdErr: cmdErr}
}

func TestFmtCommand_FormatTemplate_AlreadyFormatted(t *testing.T) {
	t.Parallel()
	c := &FmtCommand{}
	input := "job \"example\" {\n  datacenters = [\"dc1\"]\n}\n"
	result, err := c.formatTemplate(input)
	must.NoError(t, err)
	must.Eq(t, input, result)
}

func TestFmtCommand_FormatTemplate_PreservesDelimiters(t *testing.T) {
	t.Parallel()
	c := &FmtCommand{}
	input := "job [[ var \"job_name\" . | quote ]] {\n  datacenters = [[ var \"datacenters\" . | toJson ]]\n}\n"
	result, err := c.formatTemplate(input)
	must.NoError(t, err)
	must.StrContains(t, result, `[[ var "job_name" . | quote ]]`)
	must.StrContains(t, result, `[[ var "datacenters" . | toJson ]]`)
}

func TestFmtCommand_FormatTemplate_FixesIndentation(t *testing.T) {
	t.Parallel()
	c := &FmtCommand{}
	// badly indented HCL
	input := "job \"x\" {\ngroup \"g\" {\ntask \"t\" {\n}\n}\n}\n"
	result, err := c.formatTemplate(input)
	must.NoError(t, err)
	// hclwrite.Format should add indentation
	must.StrContains(t, result, "  group")
}

func TestFmtCommand_FormatTemplate_MultipleDelimiters(t *testing.T) {
	t.Parallel()
	c := &FmtCommand{}
	input := "job [[ .name ]] {\n  count = [[ .count ]]\n}\n"
	result, err := c.formatTemplate(input)
	must.NoError(t, err)
	must.StrContains(t, result, "[[ .name ]]")
	must.StrContains(t, result, "[[ .count ]]")
}

func TestFmtCommand_formatHCL(t *testing.T) {
	ci.Parallel(t)

	c := &FmtCommand{}

	// Unformatted HCL content
	input := `variable "test" {
type = string
default = "value"
description = "A test variable"
}`

	result, err := c.formatHCL(input)
	must.NoError(t, err)

	// hclwrite.Format should add proper indentation
	must.StrContains(t, result, "  type")
	must.StrContains(t, result, "  default")
	must.StrContains(t, result, "  description")
}

// ── Integration tests via runPackCmd() ───────────────────────────────────────

func TestCLI_Fmt_NoTemplateFiles(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	// Writing a non-formattable file
	must.NoError(t, os.WriteFile(filepath.Join(dir, "README.md"), []byte("# Test\n"), 0644))

	result := runPackCmd(t, []string{"fmt", dir})
	must.Zero(t, result.exitCode)
	must.StrContains(t, result.cmdOut.String(), "No formattable files")
}

func TestFmtCommand_FormatMetadataHCL(t *testing.T) {
	ci.Parallel(t)

	tmpDir := t.TempDir()
	metadataPath := filepath.Join(tmpDir, "metadata.hcl")

	unformattedContent := `app {
url = "https://example.com"
author = "test"
}`

	err := os.WriteFile(metadataPath, []byte(unformattedContent), 0644)
	must.NoError(t, err)

	// Use the existing test helper
	result := runPackCmd(t, []string{"fmt", metadataPath})
	must.Eq(t, 0, result.exitCode)
	must.StrContains(t, result.cmdOut.String(), "Formatted")

	// Verify file was formatted
	formatted, err := os.ReadFile(metadataPath)
	must.NoError(t, err)
	must.StrContains(t, string(formatted), "  url")
	must.StrContains(t, string(formatted), "  author")
}

func TestFmtCommand_FormatVariablesHCL(t *testing.T) {
	ci.Parallel(t)

	tmpDir := t.TempDir()
	variablesPath := filepath.Join(tmpDir, "variables.hcl")

	unformattedContent := `variable "job_name" {
type = string
default = "example"
}

variable "count" {
type = number
default = 1
}`

	err := os.WriteFile(variablesPath, []byte(unformattedContent), 0644)
	must.NoError(t, err)

	// Use the existing test helper
	result := runPackCmd(t, []string{"fmt", variablesPath})
	must.Eq(t, 0, result.exitCode)
	must.StrContains(t, result.cmdOut.String(), "Formatted")

	formatted, err := os.ReadFile(variablesPath)
	must.NoError(t, err)
	must.StrContains(t, string(formatted), "  type")
	must.StrContains(t, string(formatted), "  default")
}

func TestFmtCommand_FormatMixedDirectory(t *testing.T) {
	ci.Parallel(t)

	tmpDir := t.TempDir()

	// Create unformatted .tpl file
	tplPath := filepath.Join(tmpDir, "app.nomad.tpl")
	tplContent := `job "example" {
group "app" {
count = [[ var "count" . ]]
}
}`
	err := os.WriteFile(tplPath, []byte(tplContent), 0644)
	must.NoError(t, err)

	// Create unformatted .hcl file
	hclPath := filepath.Join(tmpDir, "variables.hcl")
	hclContent := `variable "count" {
type = number
default = 1
}`
	err = os.WriteFile(hclPath, []byte(hclContent), 0644)
	must.NoError(t, err)

	// Run fmt with recursive flag
	result := runPackCmd(t, []string{"fmt", "--recursive", tmpDir})
	must.Eq(t, 0, result.exitCode)

	// Verify both files were formatted
	formattedTpl, err := os.ReadFile(tplPath)
	must.NoError(t, err)
	must.StrContains(t, string(formattedTpl), "  group")
	must.StrContains(t, string(formattedTpl), "[[ var \"count\" . ]]") // Template syntax preserved

	formattedHcl, err := os.ReadFile(hclPath)
	must.NoError(t, err)
	must.StrContains(t, string(formattedHcl), "  type")
}

func TestFmtCommand_CheckModeWithHCL(t *testing.T) {
	ci.Parallel(t)

	tmpDir := t.TempDir()
	hclPath := filepath.Join(tmpDir, "metadata.hcl")

	// Create unformatted HCL file
	unformattedContent := `app {
url = "https://example.com"
}`
	err := os.WriteFile(hclPath, []byte(unformattedContent), 0644)
	must.NoError(t, err)

	// Run with --check flag
	result := runPackCmdAllowErrors(t, []string{"fmt", "--check", hclPath})
	must.Eq(t, 1, result.exitCode) // Should return 1 because file needs formatting

	// Verify file was NOT modified
	content, err := os.ReadFile(hclPath)
	must.NoError(t, err)
	must.Eq(t, unformattedContent, string(content))

	// Verify error message
	must.StrContains(t, result.cmdOut.String(), "not formatted")
}

func TestCLI_Fmt_SingleFile_AlreadyFormatted(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	content := "job \"example\" {\n  datacenters = [\"dc1\"]\n}\n"
	tplPath := filepath.Join(dir, "test.nomad.tpl")
	must.NoError(t, os.WriteFile(tplPath, []byte(content), 0644))

	result := runPackCmd(t, []string{"fmt", tplPath})
	must.Zero(t, result.exitCode)
	// file content should be unchanged
	got, err := os.ReadFile(tplPath)
	must.NoError(t, err)
	must.Eq(t, content, string(got))
}

func TestCLI_Fmt_CheckMode_FormattingNeeded(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	// badly indented
	content := "job \"x\" {\ngroup \"g\" {\n}\n}\n"
	tplPath := filepath.Join(dir, "test.nomad.tpl")
	must.NoError(t, os.WriteFile(tplPath, []byte(content), 0644))

	result := runPackCmdAllowErrors(t, []string{"fmt", "--check", tplPath})
	must.Eq(t, 1, result.exitCode)
	must.StrContains(t, result.cmdOut.String(), "not formatted")
}

func TestCLI_Fmt_CheckMode_AlreadyFormatted(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	content := "job \"example\" {\n  datacenters = [\"dc1\"]\n}\n"
	tplPath := filepath.Join(dir, "test.nomad.tpl")
	must.NoError(t, os.WriteFile(tplPath, []byte(content), 0644))

	result := runPackCmd(t, []string{"fmt", "--check", tplPath})
	must.Zero(t, result.exitCode)
}

func TestCLI_Fmt_WriteMode_FormatsFile(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	content := "job \"x\" {\ngroup \"g\" {\n}\n}\n"
	tplPath := filepath.Join(dir, "test.nomad.tpl")
	must.NoError(t, os.WriteFile(tplPath, []byte(content), 0644))

	result := runPackCmd(t, []string{"fmt", "--write=true", tplPath})
	must.Zero(t, result.exitCode)
	// file will now be formatted
	got, err := os.ReadFile(tplPath)
	must.NoError(t, err)
	must.StrContains(t, string(got), "  group")
}

func TestCLI_Fmt_ListMode_PrintsFilename(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	content := "job \"x\" {\ngroup \"g\" {\n}\n}\n"
	tplPath := filepath.Join(dir, "test.nomad.tpl")
	must.NoError(t, os.WriteFile(tplPath, []byte(content), 0644))

	result := runPackCmd(t, []string{"fmt", "--write=false", "--list=true", tplPath})
	must.Zero(t, result.exitCode)
	must.StrContains(t, result.cmdOut.String(), tplPath)
}

func TestCLI_Fmt_RecursiveFlag(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	subdir := filepath.Join(dir, "templates")
	must.NoError(t, os.MkdirAll(subdir, 0755))
	content := "job \"x\" {\ngroup \"g\" {\n}\n}\n"
	must.NoError(t, os.WriteFile(filepath.Join(subdir, "test.nomad.tpl"), []byte(content), 0644))

	// Without --recursive: no files found in subdir
	result := runPackCmd(t, []string{"fmt", "--write=false", "--list=true", dir})
	must.Zero(t, result.exitCode)
	must.StrContains(t, result.cmdOut.String(), "No formattable files")

	// With --recursive: file found
	result = runPackCmd(t, []string{"fmt", "--write=false", "--list=true", "--recursive", dir})
	must.Zero(t, result.exitCode)
	must.StrContains(t, result.cmdOut.String(), "test.nomad.tpl")
}

func TestCLI_Fmt_NonExistentPath(t *testing.T) {
	t.Parallel()
	result := runPackCmdAllowErrors(t, []string{"fmt", "/nonexistent/path/that/does/not/exist"})
	must.Eq(t, 1, result.exitCode)
}

func TestCLI_Fmt_PreservesTemplateSyntax(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	content := "job [[ var \"job_name\" . | quote ]] {\n  datacenters = [[ var \"datacenters\" . | toJson ]]\n}\n"
	tplPath := filepath.Join(dir, "test.nomad.tpl")
	must.NoError(t, os.WriteFile(tplPath, []byte(content), 0644))

	result := runPackCmd(t, []string{"fmt", "--write=true", tplPath})
	must.Zero(t, result.exitCode)

	got, err := os.ReadFile(tplPath)
	must.NoError(t, err)
	must.StrContains(t, string(got), `[[ var "job_name" . | quote ]]`)
	must.StrContains(t, string(got), `[[ var "datacenters" . | toJson ]]`)
}
