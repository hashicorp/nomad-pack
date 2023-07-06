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

	"github.com/hashicorp/nomad-pack/internal/pkg/cache"
	"github.com/hashicorp/nomad-pack/internal/pkg/helper"
	"github.com/hashicorp/nomad-pack/internal/pkg/helper/filesystem"
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

	cacheDir := t.TempDir()
	globalCache, err := cache.NewCache(&cache.CacheConfig{
		Path:   cacheDir,
		Logger: NoopLogger{},
	})
	must.NoError(t, err)
	must.NotNil(t, globalCache)

	uiStdout := new(bytes.Buffer)
	uiStderr := new(bytes.Buffer)
	uiCtx, cancel := helper.WithInterrupt(context.Background())
	defer cancel()
	ui := testui.NonInteractiveTestUI(uiCtx, uiStdout, uiStderr)

	// first run against an empty directory
	tmpDir1 := t.TempDir()
	err = Vendor(ctx, ui, globalCache, tmpDir1, false)
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

	err = Vendor(ctx, ui, globalCache, tmpDir2, false)
	must.NotNil(t, err)
	must.ErrorContains(t, err, "does not contain any dependencies")

	// test adding to cache
	tmpPackDir := t.TempDir()
	f, err = os.Create(path.Join(tmpPackDir, "metadata.hcl"))
	if err != nil {
		t.Error(err)
	}

	tmpDependencySourceDir := t.TempDir()
	must.NoError(t, createTestDepRepo(tmpDependencySourceDir))
	goodMetadata.Dependencies[0].Source = path.Join(
		tmpDependencySourceDir,
		"simple_raw_exec",
	)

	fw = hclwrite.NewEmptyFile()
	gohcl.EncodeIntoBody(&goodMetadata, fw.Body())
	_, err = fw.WriteTo(f)
	if err != nil {
		t.Error(err)
	}

	err = Vendor(ctx, ui, globalCache, tmpPackDir, true)
	must.Nil(t, err, must.Sprintf("vendoring failure: %v", err))
	must.Eq(t, len(globalCache.Registries()), 1)
	must.Eq(t, globalCache.Registries()[0].Name, "vendor")
	must.Eq(t, len(globalCache.Registries()[0].Packs), 1)
	must.Eq(t, globalCache.Registries()[0].Packs[0].Name(), "simple_raw_exec")
	must.StrContains(t, uiStdout.String(), "success")

	// test overwriting a vendored pack
	tmpPackDir2 := t.TempDir()
	f, err = os.Create(path.Join(tmpPackDir2, "metadata.hcl"))
	if err != nil {
		t.Error(err)
	}

	tmpDependencySourceDir2 := t.TempDir()
	must.NoError(t, createTestDepRepo(tmpDependencySourceDir2))
	goodMetadata.Dependencies[0].Source = path.Join(
		tmpDependencySourceDir,
		"simple_raw_exec",
	)

	fw = hclwrite.NewEmptyFile()
	gohcl.EncodeIntoBody(&goodMetadata, fw.Body())
	_, err = fw.WriteTo(f)
	if err != nil {
		t.Error(err)
	}

	err = Vendor(ctx, ui, globalCache, tmpPackDir, true)
	must.Nil(t, err, must.Sprintf("vendoring failure: %v", err))
	must.Eq(t, len(globalCache.Registries()), 1, must.Sprintf("wrong number of registries"))
	must.Eq(t, globalCache.Registries()[0].Name, "vendor")
	must.Eq(t, len(globalCache.Registries()[0].Packs), 1, must.Sprintf("wrong number of packs"))
	must.Eq(t, globalCache.Registries()[0].Packs[0].Name(), "simple_raw_exec")
	must.StrContains(t, uiStdout.String(), "success")

}

// createTestDepRepo creates a git repository with a dependency pack in it
func createTestDepRepo(dst string) error {
	err := filesystem.CopyDir(
		"../../../fixtures/test_registry/packs/simple_raw_exec",
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

func (_ NoopLogger) Debug(_ string)                                  {}
func (_ NoopLogger) Error(_ string)                                  {}
func (_ NoopLogger) ErrorWithContext(_ error, _ string, _ ...string) {}
func (_ NoopLogger) Info(_ string)                                   {}
func (_ NoopLogger) Trace(_ string)                                  {}
func (_ NoopLogger) Warning(_ string)                                {}
