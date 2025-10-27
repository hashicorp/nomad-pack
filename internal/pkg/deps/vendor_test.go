// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package deps

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"path"
	"testing"
	"time"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing/object"
	"github.com/hashicorp/hcl/v2/gohcl"
	"github.com/hashicorp/hcl/v2/hclwrite"
	"github.com/shoenig/test/must"

	"github.com/hashicorp/nomad-pack/internal/pkg/helper"
	"github.com/hashicorp/nomad-pack/internal/pkg/helper/filesystem"
	"github.com/hashicorp/nomad-pack/internal/pkg/testfixture"
	"github.com/hashicorp/nomad-pack/internal/testui"
	"github.com/hashicorp/nomad-pack/sdk/pack"
)

var emptyMetadata = pack.Metadata{
	App:          &pack.MetadataApp{},
	Pack:         &pack.MetadataPack{},
	Integration:  &pack.MetadataIntegration{},
	Dependencies: []*pack.Dependency{},
}

var goodMetadata = pack.Metadata{
	App: &pack.MetadataApp{
		URL: "",
	},
	Pack: &pack.MetadataPack{
		Name:        "deps_test",
		Description: "This pack tests dependencies",
		Version:     "0.0.1",
	},
	Dependencies: []*pack.Dependency{
		{
			Name:   "simple_raw_exec",
			Source: "",
		},
	},
}

func TestVendor(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	uiStdout := new(bytes.Buffer)
	uiStderr := new(bytes.Buffer)
	uiCtx, cancel := helper.WithInterrupt(context.Background())
	defer cancel()
	ui := testui.NonInteractiveTestUI(uiCtx, uiStdout, uiStderr)

	// first run against an empty directory
	tmpDir1 := t.TempDir()
	err := Vendor(ctx, ui, tmpDir1)
	must.NotNil(t, err)
	must.ErrorContains(t, err, "does not exist")

	// run against a metadata file with empty dependencies
	tmpDir2 := t.TempDir()
	f, err := os.Create(path.Join(tmpDir2, "metadata.hcl"))
	if err != nil {
		t.Error(err)
	}

	fw := hclwrite.NewEmptyFile()
	gohcl.EncodeIntoBody(&emptyMetadata, fw.Body())
	_, err = fw.WriteTo(f)
	if err != nil {
		t.Error(err)
	}

	err = Vendor(ctx, ui, tmpDir2)
	must.NotNil(t, err)
	must.ErrorContains(t, err, "does not contain any dependencies")

	// test overwriting a vendored pack
	tmpPackDir2 := t.TempDir()
	f, err = os.Create(path.Join(tmpPackDir2, "metadata.hcl"))
	if err != nil {
		t.Error(err)
	}

	tmpDependencySourceDir2 := t.TempDir()
	must.NoError(t, createTestDepRepo(t, tmpDependencySourceDir2))
	goodMetadata.Dependencies[0].Source = path.Join(
		tmpDependencySourceDir2,
		"simple_raw_exec",
	)

	fw = hclwrite.NewEmptyFile()
	gohcl.EncodeIntoBody(&goodMetadata, fw.Body())
	_, err = fw.WriteTo(f)
	if err != nil {
		t.Error(err)
	}

	err = Vendor(ctx, ui, tmpPackDir2)
	must.Nil(t, err, must.Sprintf("vendoring failure: %v", err))
	must.StrContains(t, uiStdout.String(), "success")

}

// createTestDepRepo creates a git repository with a dependency pack in it
func createTestDepRepo(t *testing.T, dst string) error {

	err := filesystem.CopyDir(
		testfixture.AbsPath(t, "v2/test_registry/packs/simple_raw_exec"),
		path.Join(dst, "simple_raw_exec"),
		false,
		NoopLogger{},
	)
	if err != nil {
		return fmt.Errorf("unable to copy test fixtures to test git repo: %v", err)
	}

	// initialize git repo...
	r, err := git.PlainInit(dst, false)
	if err != nil {
		return fmt.Errorf("unable to initialize test git repo: %v", err)
	}
	// ...and worktree
	w, err := r.Worktree()
	if err != nil {
		return fmt.Errorf("unable to initialize worktree for test git repo: %v", err)
	}

	_, err = w.Add(".")
	if err != nil {
		return fmt.Errorf("unable to stage test files to test git repo: %v", err)
	}

	commitOptions := &git.CommitOptions{Author: &object.Signature{
		Name:  "Github Action Test User",
		Email: "test@example.com",
		When:  time.Now(),
	}}

	_, err = w.Commit("Initial Commit", commitOptions)
	if err != nil {
		return fmt.Errorf("unable to commit test files to test git repo: %v", err)
	}

	return nil
}

type NoopLogger struct{}

func (NoopLogger) Debug(_ string)                                  {}
func (NoopLogger) Error(_ string)                                  {}
func (NoopLogger) ErrorWithContext(_ error, _ string, _ ...string) {}
func (NoopLogger) Info(_ string)                                   {}
func (NoopLogger) Trace(_ string)                                  {}
func (NoopLogger) Warning(_ string)                                {}
