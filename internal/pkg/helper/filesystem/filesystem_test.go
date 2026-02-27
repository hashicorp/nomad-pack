// Copyright IBM Corp. 2021, 2025
// SPDX-License-Identifier: MPL-2.0

package filesystem

import (
	"os"
	"path"
	"testing"

	"github.com/hashicorp/nomad-pack/internal/pkg/logging"
	"github.com/shoenig/test/must"
)

func TestRenameAll(t *testing.T) {
	t.Parallel()

	oldDir := t.TempDir()
	newDir := t.TempDir()

	err := os.Mkdir(path.Join(oldDir, "test"), 0755)
	must.NoError(t, err)

	err = os.WriteFile(path.Join(oldDir, "test", "test.txt"), []byte("test"), 0755)
	must.NoError(t, err)

	logger := logging.TestLogger{}

	err = CopyDir(oldDir, path.Join(newDir, "test"), false, &logger)
	must.NoError(t, err)

	dirEntries, err := os.ReadDir(newDir)
	must.NoError(t, err)

	for _, dirEntry := range dirEntries {
		must.Eq(t, "test", dirEntry.Name())

		subDirEntries, err := os.ReadDir(path.Join(oldDir, "test"))
		must.NoError(t, err)
		for _, subDirEntry := range subDirEntries {
			must.Eq(t, "test.txt", subDirEntry.Name())
		}
	}
}

// TestCopyDirSkipsGitDir verifies that CopyDir does not copy .git directories.
func TestCopyDirSkipsGitDir(t *testing.T) {
	t.Parallel()

	srcDir := t.TempDir()
	dstDir := path.Join(t.TempDir(), "dest")

	// Create a regular subdirectory with a file.
	must.NoError(t, os.MkdirAll(path.Join(srcDir, "subdir"), 0755))
	must.NoError(t, os.WriteFile(path.Join(srcDir, "subdir", "file.txt"), []byte("hello"), 0644))

	// Create a .git directory with a file to simulate a cloned repo.
	must.NoError(t, os.MkdirAll(path.Join(srcDir, ".git", "objects"), 0755))
	must.NoError(t, os.WriteFile(path.Join(srcDir, ".git", "HEAD"), []byte("ref: refs/heads/main"), 0644))

	logger := logging.TestLogger{}
	must.NoError(t, CopyDir(srcDir, dstDir, false, &logger))

	// The regular subdirectory and its contents must be copied.
	_, err := os.Stat(path.Join(dstDir, "subdir", "file.txt"))
	must.NoError(t, err)

	// The .git directory must not be present in the destination.
	_, err = os.Stat(path.Join(dstDir, ".git"))
	must.True(t, os.IsNotExist(err))
}
