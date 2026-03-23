// Copyright IBM Corp. 2021, 2025
// SPDX-License-Identifier: MPL-2.0

package parser

import (
	"os"
	"path/filepath"
	"testing"

	sdkpack "github.com/hashicorp/nomad-pack/sdk/pack"
	"github.com/hashicorp/nomad-pack/sdk/pack/variables"
	"github.com/shoenig/test/must"
)

func TestFileRelativeContents(t *testing.T) {
	// Create a temp directory with a test file to simulate a pack root.
	tmpDir := t.TempDir()
	subDir := filepath.Join(tmpDir, "config")
	must.NoError(t, os.MkdirAll(subDir, 0755))

	fileContent := "hello from relative file"
	relPath := filepath.Join("config", "data.txt")
	must.NoError(t, os.WriteFile(filepath.Join(tmpDir, relPath), []byte(fileContent), 0644))

	// Helper to build a PackData whose metadata includes the given path.
	makePackData := func(packPath string) PackData {
		return PackData{
			meta: map[string]any{
				"pack": map[string]any{
					"name":    "test",
					"version": "0.0.1",
					"path":    packPath,
				},
			},
			vars: map[string]any{},
		}
	}

	t.Run("reads file relative to pack path", func(t *testing.T) {
		pd := makePackData(tmpDir)
		got, err := fileRelativeContents(relPath, pd)
		must.NoError(t, err)
		must.Eq(t, fileContent, got)
	})

	t.Run("errors when file does not exist", func(t *testing.T) {
		pd := makePackData(tmpDir)
		_, err := fileRelativeContents("nonexistent.txt", pd)
		must.Error(t, err)
		must.StrContains(t, err.Error(), "fileRelative: failed to read")
	})

	t.Run("errors when pack.path not set in metadata", func(t *testing.T) {
		pd := PackData{
			meta: map[string]any{
				"pack": map[string]any{
					"name":    "test",
					"version": "0.0.1",
					// no "path" key
				},
			},
			vars: map[string]any{},
		}
		_, err := fileRelativeContents("any.txt", pd)
		must.Error(t, err)
		must.StrContains(t, err.Error(), "pack path not set in metadata")
	})

	t.Run("errors when pack metadata missing", func(t *testing.T) {
		pd := PackData{
			meta: map[string]any{
				// no "pack" key
			},
			vars: map[string]any{},
		}
		_, err := fileRelativeContents("any.txt", pd)
		must.Error(t, err)
		must.StrContains(t, err.Error(), "pack metadata not available")
	})

	t.Run("works via PackTemplateContext (getPack delegation)", func(t *testing.T) {
		pd := makePackData(tmpDir)
		ctx := PackTemplateContext{
			CurrentPackKey: pd,
		}
		got, err := fileRelativeContents(relPath, ctx)
		must.NoError(t, err)
		must.Eq(t, fileContent, got)
	})
}

// TestPackPathInMetadata verifies that after ToPackTemplateContext is called
// the pack path is accessible through the "pack.path" metadata key.
func TestPackPathInMetadata(t *testing.T) {
	// Build a minimal Pack with a known path.
	p := testpack("mypkg")
	p.Path = "/some/absolute/path"
	p.Metadata.App = &sdkpack.MetadataApp{}
	p.Metadata.Pack.Version = "0.1.0"

	pv := &ParsedVariables{}
	must.NoError(t, pv.LoadV2Result(map[sdkpack.ID]map[variables.ID]*variables.Variable{}))

	ctx, diags := pv.ToPackTemplateContext(p)
	must.False(t, diags.HasErrors())

	pd := ctx.getPack()
	packMeta, ok := pd.meta["pack"].(map[string]any)
	must.True(t, ok)
	must.Eq(t, "/some/absolute/path", packMeta["path"])
}
