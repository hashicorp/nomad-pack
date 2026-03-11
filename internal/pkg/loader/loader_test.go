// Copyright IBM Corp. 2021, 2025
// SPDX-License-Identifier: MPL-2.0

package loader

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/hashicorp/nomad/ci"
	"github.com/shoenig/test/must"
)

// fixturePath returns the absolute path to the given fixture pack directory.
// It is derived from the location of this source file so it is unaffected by
// os.Chdir calls in parallel tests.
func fixturePath(t *testing.T, parts ...string) string {
	t.Helper()
	// __file__ is .../internal/pkg/loader/loader_test.go
	// Three directory levels up lands at the repo root.
	_, file, _, ok := runtime.Caller(0)
	must.True(t, ok, must.Sprint("runtime.Caller failed"))
	repoRoot := filepath.Join(filepath.Dir(file), "..", "..", "..")
	abs, err := filepath.Abs(filepath.Join(append([]string{repoRoot, "fixtures"}, parts...)...))
	must.NoError(t, err)
	return abs
}

// TestLoad_NonExistentPath verifies Load returns an error when the path does
// not exist.
func TestLoad_NonExistentPath(t *testing.T) {
	ci.Parallel(t)
	_, err := Load("/this/path/does/not/exist")
	must.Error(t, err)
}

// TestLoad_NonDirectory verifies Load returns an error when the path points to
// a regular file rather than a directory.
func TestLoad_NonDirectory(t *testing.T) {
	ci.Parallel(t)

	f, err := os.CreateTemp(t.TempDir(), "not-a-dir-*.hcl")
	must.NoError(t, err)
	must.NoError(t, f.Close())

	_, err = Load(f.Name())
	must.Error(t, err)
	must.ErrorContains(t, err, "non-directory")
}

// TestLoad_MissingMetadata verifies Load returns an error when the pack
// directory contains no metadata.hcl.
func TestLoad_MissingMetadata(t *testing.T) {
	ci.Parallel(t)

	dir := t.TempDir()
	// Write a dummy template so the directory is not totally empty.
	must.NoError(t, os.MkdirAll(filepath.Join(dir, "templates"), 0o755))
	must.NoError(t, os.WriteFile(
		filepath.Join(dir, "templates", "job.nomad.tpl"),
		[]byte(`job "example" {}`),
		0o644,
	))

	_, err := Load(dir)
	must.Error(t, err)
	must.ErrorContains(t, err, "metadata.hcl")
}

// TestLoad_SetsPackPath verifies that p.Path is set to the absolute, clean
// root directory of the pack after loading. This is the primary behaviour
// added to support pack-relative file references in templates (the
// fileRelative template function and meta "pack.path").
func TestLoad_SetsPackPath(t *testing.T) {
	ci.Parallel(t)

	packDir := fixturePath(t, "v1", "simple_raw_exec_v1")
	absPackDir, err := filepath.Abs(packDir)
	must.NoError(t, err)

	p, err := Load(packDir)
	must.NoError(t, err)
	must.NotNil(t, p)

	// Path must be the clean absolute root – no trailing separator.
	must.Eq(t, absPackDir, p.Path)
}

// TestLoad_SetsPackPath_RelativeInput verifies that a relative directory
// argument is correctly resolved to an absolute path on p.Path.
func TestLoad_SetsPackPath_RelativeInput(t *testing.T) {
	ci.Parallel(t)

	packDir := fixturePath(t, "v1", "simple_raw_exec_v1")
	absPackDir, err := filepath.Abs(packDir)
	must.NoError(t, err)

	// Change working directory so we can use a relative path.
	orig, err := os.Getwd()
	must.NoError(t, err)
	t.Cleanup(func() { _ = os.Chdir(orig) })
	must.NoError(t, os.Chdir(filepath.Dir(packDir)))

	p, err := Load(filepath.Base(packDir))
	must.NoError(t, err)
	must.NotNil(t, p)

	must.Eq(t, absPackDir, p.Path)
}

// TestLoad_PackPath_NoTrailingSeparator verifies that p.Path never ends with a
// path separator, which would cause filepath.Join(p.Path, relFile) to produce
// a double-separator path on some platforms.
func TestLoad_PackPath_NoTrailingSeparator(t *testing.T) {
	ci.Parallel(t)

	packDir := fixturePath(t, "v1", "simple_raw_exec_v1")
	p, err := Load(packDir)
	must.NoError(t, err)
	must.NotNil(t, p)

	last := p.Path[len(p.Path)-1]
	must.True(t, last != filepath.Separator, must.Sprint("p.Path must not end with a separator"))
}

// TestLoad_MetadataParsed verifies that the metadata.hcl file is decoded
// correctly, and that the resulting Metadata matches expected values from the
// fixture.
func TestLoad_MetadataParsed(t *testing.T) {
	ci.Parallel(t)

	p, err := Load(fixturePath(t, "v1", "simple_raw_exec_v1"))
	must.NoError(t, err)
	must.NotNil(t, p.Metadata)
	must.NotNil(t, p.Metadata.Pack)

	must.Eq(t, "simple_raw_exec", p.Metadata.Pack.Name)
	must.Eq(t, "0.0.1", p.Metadata.Pack.Version)
}

// TestLoad_TemplateFilesLoaded verifies that files under templates/ with the
// .nomad.tpl extension are collected into TemplateFiles.
func TestLoad_TemplateFilesLoaded(t *testing.T) {
	ci.Parallel(t)

	p, err := Load(fixturePath(t, "v1", "simple_raw_exec_v1"))
	must.NoError(t, err)
	must.NotNil(t, p.TemplateFiles)
	must.Positive(t, len(p.TemplateFiles))

	for _, f := range p.TemplateFiles {
		must.True(t,
			strings.HasPrefix(f.Name, "templates/"),
			must.Sprintf("template file %q must be under templates/", f.Name),
		)
	}
}

// TestLoad_RootVariableFileLoaded verifies that variables.hcl is wired up as
// the RootVariableFile.
func TestLoad_RootVariableFileLoaded(t *testing.T) {
	ci.Parallel(t)

	p, err := Load(fixturePath(t, "v1", "simple_raw_exec_v1"))
	must.NoError(t, err)
	must.NotNil(t, p.RootVariableFile)
	must.Eq(t, "variables.hcl", p.RootVariableFile.Name)
}

// TestLoad_OutputTemplateFileLoaded verifies that outputs.tpl is wired up as
// the OutputTemplateFile.
func TestLoad_OutputTemplateFileLoaded(t *testing.T) {
	ci.Parallel(t)

	p, err := Load(fixturePath(t, "v1", "simple_raw_exec_v1"))
	must.NoError(t, err)
	must.NotNil(t, p.OutputTemplateFile)
	must.Eq(t, "outputs.tpl", p.OutputTemplateFile.Name)
}

// TestLoad_FileContentNotEmpty verifies that file content is actually read
// from disk (not left as nil/empty slices).
func TestLoad_FileContentNotEmpty(t *testing.T) {
	ci.Parallel(t)

	p, err := Load(fixturePath(t, "v1", "simple_raw_exec_v1"))
	must.NoError(t, err)

	for _, f := range p.TemplateFiles {
		must.Positive(t, len(f.Content),
			must.Sprintf("template file %q should have non-empty content", f.Name))
	}

	must.Positive(t, len(p.RootVariableFile.Content))
}
