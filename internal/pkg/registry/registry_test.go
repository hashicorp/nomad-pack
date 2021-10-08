package registry

import (
	"fmt"
	"os"
	"path"
	"testing"

	"github.com/stretchr/testify/require"
)

var testRegistryURL = "github.com/hashicorp/nomad-pack-community-registry"
var log = func(t *testing.T) func(string) {
	return func(message string) {
		t.Log(message)
	}
}

func testPackCount(t *testing.T, version string) (int, error) {
	packCount := 0
	url := testRegistryURL

	if version != "" {
		url = fmt.Sprintf("%s@%s", testRegistryURL, version)
	}

	cacheDir := t.TempDir()

	_, err := AddFromGitURL(cacheDir, url, "", "", log(t))
	if err != nil {
		return 0, err
	}

	dirEntries, err := os.ReadDir(cacheDir)
	if err != nil {
		return 0, err
	}

	for _, dirEntry := range dirEntries {
		if dirEntry.IsDir() {
			packEntries, err := os.ReadDir(path.Join(cacheDir, dirEntry.Name()))
			if err != nil {
				return 0, err
			}
			for _, packEntry := range packEntries {
				if packEntry.IsDir() {
					packCount += 1
				}
			}
		}
	}

	return packCount, nil
}

func TestListRegistries(t *testing.T) {
	t.Parallel()

	cacheDir := t.TempDir()

	_, err := AddFromGitURL(cacheDir, testRegistryURL, "", "", log(t))
	require.NoError(t, err)

	registries, err := LoadAllFromCache(cacheDir)
	require.NoError(t, err)

	expected, err := testPackCount(t, "")
	require.NoError(t, err)
	require.Len(t, registries[0].Packs, expected)
}

func TestParseRegistryURL(t *testing.T) {
	t.Parallel()

	url, err := parseRegistryURL(fmt.Sprintf("%s@%s", testRegistryURL, "1234567"))
	require.NoError(t, err)
	require.Equal(t, testRegistryURL, url)
}

func TestAddRegistry(t *testing.T) {
	t.Parallel()

	cacheDir := t.TempDir()

	registry, err := AddFromGitURL(cacheDir, testRegistryURL, "", "", log(t))
	require.NoError(t, err)
	require.NotNil(t, registry)
	require.NotEqual(t, 0, len(registry.Packs))
}

func TestAddRegistryPacksAtMultipleVersions(t *testing.T) {
	t.Parallel()

	cacheDir := t.TempDir()

	registry, err := AddFromGitURL(cacheDir, fmt.Sprintf("%s@%s", testRegistryURL, "5e564c9"), "version-test", "", log(t))
	require.NoError(t, err)
	require.NotNil(t, registry)

	registry, err = AddFromGitURL(cacheDir, fmt.Sprintf("%s@%s", testRegistryURL, "v0.0.1"), "version-test", "", log(t))
	require.NoError(t, err)
	require.NotNil(t, registry)

	// test that registry still exists
	registryEntries, err := os.ReadDir(cacheDir)
	require.NoError(t, err)
	require.Len(t, registryEntries, 1)

	// test that multiple versions of pack exist
	packEntries, err := os.ReadDir(path.Join(cacheDir, "version-test"))
	require.NoError(t, err)

	// expected len should be 2 * testPackCount
	expected, err := testPackCount(t, "5e564c9")
	require.NoError(t, err)
	expected = expected * 2

	require.Len(t, packEntries, expected)
}

func TestAddRegistryWithTarget(t *testing.T) {
	t.Parallel()

	cacheDir := t.TempDir()

	registry, err := AddFromGitURL(cacheDir, testRegistryURL, "", "traefik", log(t))
	require.NoError(t, err)
	require.NotNil(t, registry)

	require.Len(t, registry.Packs, 1)
}

func TestAddRegistryWithSHA(t *testing.T) {
	t.Parallel()

	cacheDir := t.TempDir()

	registry, err := AddFromGitURL(cacheDir, fmt.Sprintf("%s@5e564c9", testRegistryURL), "", "", log(t))
	require.NoError(t, err)
	require.NotNil(t, registry)

	expected, err := testPackCount(t, "5e564c9")
	require.NoError(t, err)

	require.Len(t, registry.Packs, expected)
}

func TestAddRegistryWithVersionAndTarget(t *testing.T) {
	t.Parallel()

	cacheDir := t.TempDir()

	registry, err := AddFromGitURL(cacheDir, fmt.Sprintf("%s@v0.0.1", testRegistryURL), "", "traefik", log(t))
	require.NoError(t, err)
	require.NotNil(t, registry)

	require.Len(t, registry.Packs, 1)
}

func TestAddRegistryNoCacheDir(t *testing.T) {
	t.Parallel()

	registry, err := AddFromGitURL("", "", "", "", log(t))
	require.Error(t, err)
	require.Nil(t, registry)
	require.Equal(t, "cache directory is required", err.Error())
}

func TestAddRegistryNoFrom(t *testing.T) {
	t.Parallel()

	cacheDir := t.TempDir()

	registry, err := AddFromGitURL(cacheDir, "", "", "", log(t))
	require.Error(t, err)
	require.Nil(t, registry)
	require.Equal(t, "registry url is required", err.Error())
}

func TestAddRegistryWithProtocol(t *testing.T) {
	t.Parallel()

	cacheDir := t.TempDir()

	registry, err := AddFromGitURL(cacheDir, "https://github.com/hashicorp/nomad-pack-registry", "", "", log(t))
	require.Error(t, err)
	require.Nil(t, registry)
	require.Contains(t, err.Error(), "must start with")
}

func TestDeleteRegistry(t *testing.T) {
	t.Parallel()

	cacheDir := t.TempDir()

	registry, err := AddFromGitURL(cacheDir, testRegistryURL, "", "", log(t))
	require.NoError(t, err)
	require.NotNil(t, registry)

	name, err := parseRegistrySlug(testRegistryURL)
	require.NoError(t, err)
	require.NotEmpty(t, name)

	err = DeleteFromCache(cacheDir, name, "", log(t))
	require.NoError(t, err)

	// test that registry is gone
	registryEntries, err := os.ReadDir(cacheDir)
	require.NoError(t, err)
	require.Len(t, registryEntries, 0)
}

func TestDeleteRegistryByAlias(t *testing.T) {
	t.Parallel()

	cacheDir := t.TempDir()

	registry, err := AddFromGitURL(cacheDir, testRegistryURL, "test", "", log(t))
	require.NoError(t, err)
	require.NotNil(t, registry)

	err = DeleteFromCache(cacheDir, "test", "", log(t))
	require.NoError(t, err)

	// test that registry is gone
	registryEntries, err := os.ReadDir(cacheDir)
	require.NoError(t, err)
	require.Len(t, registryEntries, 0)
}

func TestDeletePackByTarget(t *testing.T) {
	t.Parallel()

	cacheDir := t.TempDir()

	registry, err := AddFromGitURL(cacheDir, testRegistryURL, "", "", log(t))
	require.NoError(t, err)
	require.NotNil(t, registry)

	name, err := parseRegistrySlug(testRegistryURL)
	require.NoError(t, err)
	require.NotEmpty(t, name)

	err = DeleteFromCache(cacheDir, name, "traefik", log(t))
	require.NoError(t, err)

	// test that registry still exists
	registryEntries, err := os.ReadDir(cacheDir)
	require.NoError(t, err)
	require.Len(t, registryEntries, 1)

	// test that pack is gone but other packs are still there
	expected, err := testPackCount(t, "")
	require.NoError(t, err)

	packEntries, err := os.ReadDir(path.Join(cacheDir, name))
	require.NoError(t, err)
	require.Len(t, packEntries, expected-1)
}

func TestDeleteRegistryPacksByVersionAndTarget(t *testing.T) {
	t.Parallel()

	cacheDir := t.TempDir()

	// Add latest
	registry, err := AddFromGitURL(cacheDir, testRegistryURL, "", "", log(t))
	require.NoError(t, err)
	require.NotNil(t, registry)

	// Add a specific version
	registry, err = AddFromGitURL(cacheDir, testRegistryURL+"@v0.0.1", "", "", log(t))
	require.NoError(t, err)
	require.NotNil(t, registry)

	// get the registry name
	name, err := parseRegistrySlug(testRegistryURL)
	require.NoError(t, err)
	require.NotEmpty(t, name)

	// Delete one pack
	err = DeleteFromCache(cacheDir, fmt.Sprintf("%s@v0.0.1", name), "traefik", log(t))
	require.NoError(t, err)

	// test that registry still exists
	registryEntries, err := os.ReadDir(cacheDir)
	require.NoError(t, err)
	require.Len(t, registryEntries, 1)

	// test that pack is gone, but other packs still exist

	// Get the number of packs at the version
	expectedAtVersion, err := testPackCount(t, "v0.0.1")
	require.NoError(t, err)

	// Get the number of packs at latest
	expectedAtLatest, err := testPackCount(t, "v0.0.1")
	require.NoError(t, err)

	// Add the pack counts and subtract one to make sure one got deleted.
	expected := expectedAtVersion + expectedAtLatest - 1

	packEntries, err := os.ReadDir(path.Join(cacheDir, name))
	require.NoError(t, err)
	require.Len(t, packEntries, expected)
}
