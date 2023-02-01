// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package filesystem

import (
	"os"
	"path"
	"testing"

	"github.com/hashicorp/nomad-pack/internal/pkg/logging"
	"github.com/stretchr/testify/require"
)

func TestRenameAll(t *testing.T) {
	t.Parallel()

	oldDir := t.TempDir()
	newDir := t.TempDir()

	err := os.Mkdir(path.Join(oldDir, "test"), 0755)
	require.NoError(t, err)

	err = os.WriteFile(path.Join(oldDir, "test", "test.txt"), []byte("test"), 0755)
	require.NoError(t, err)

	logger := logging.TestLogger{}

	err = CopyDir(oldDir, path.Join(newDir, "test"), &logger)
	require.NoError(t, err)

	dirEntries, err := os.ReadDir(newDir)
	require.NoError(t, err)

	for _, dirEntry := range dirEntries {
		require.Equal(t, "test", dirEntry.Name())

		subDirEntries, err := os.ReadDir(path.Join(oldDir, "test"))
		require.NoError(t, err)
		for _, subDirEntry := range subDirEntries {
			require.Equal(t, "test.txt", subDirEntry.Name())
		}
	}
}
