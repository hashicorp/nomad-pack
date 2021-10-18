package cache

import (
	"os"
	"path"
	"testing"

	"github.com/hashicorp/nomad-pack/internal/pkg/errors"
	"github.com/hashicorp/nomad-pack/internal/pkg/logging"
	"github.com/stretchr/testify/require"
)

var testRegistryURL = "github.com/hashicorp/nomad-pack-community-registry"

func testAddOpts(registryName string) *AddOpts {
	return &AddOpts{
		Source:       testRegistryURL,
		RegistryName: registryName,
		PackName:     "",
		Ref:          "",
		Username:     "",
		Password:     "",
	}
}

func testPackCount(t *testing.T, opts cacheOperationProvider) int {
	packCount := 0

	dirEntries, err := os.ReadDir(opts.RegistryPath())
	require.NoError(t, err)

	for _, dirEntry := range dirEntries {
		if opts.IsTarget(dirEntry) {
			packCount += 1
		}
	}

	return packCount
}

func TestListRegistries(t *testing.T) {
	cacheDir := t.TempDir()
	opts := testAddOpts("list-registries")

	cache, err := NewCache(&CacheConfig{
		Path:   cacheDir,
		Logger: logging.NewTestLogger(t.Log),
	})
	require.NoError(t, err)
	require.NotNil(t, cache)

	registry, err := cache.Add(opts)
	require.NoError(t, err)
	require.NotNil(t, registry)

	expected := testPackCount(t, opts)
	require.Equal(t, expected, len(registry.Packs))
}

func TestAddRegistry(t *testing.T) {
	cacheDir := t.TempDir()
	opts := testAddOpts("add-registry")

	cache, err := NewCache(&CacheConfig{
		Path:   cacheDir,
		Logger: logging.NewTestLogger(t.Log),
	})
	require.NoError(t, err)
	require.NotNil(t, cache)

	registry, err := cache.Add(opts)
	require.NoError(t, err)
	require.NotNil(t, registry)

	expected := testPackCount(t, opts)
	require.NoError(t, err)
	require.Equal(t, expected, len(registry.Packs))
}

func TestAddRegistryPacksAtMultipleRefs(t *testing.T) {
	cacheDir := t.TempDir()
	testOpts := testAddOpts("multiple-refs")
	// Set opts at sha ref
	addOpts := &AddOpts{
		cachePath:    cacheDir,
		RegistryName: "multiple-refs",
		Source:       testRegistryURL,
		Ref:          "a74b4e1",
	}

	cache, err := NewCache(&CacheConfig{
		Path:   cacheDir,
		Logger: logging.NewTestLogger(t.Log),
	})
	require.NoError(t, err)
	require.NotNil(t, cache)

	// Add at ref
	registry, err := cache.Add(addOpts)
	require.NoError(t, err)
	require.NotNil(t, registry)

	// Add at latest
	registry, err = cache.Add(testOpts)
	require.NoError(t, err)
	require.NotNil(t, registry)

	expected := testPackCount(t, testOpts)

	// test that registry still exists
	registryEntries, err := os.ReadDir(cacheDir)
	require.NoError(t, err)
	require.Equal(t, 1, len(registryEntries))

	// test that multiple refs of pack exist
	packEntries, err := os.ReadDir(path.Join(cacheDir, "multiple-refs"))
	require.NoError(t, err)

	require.Equal(t, expected, len(packEntries))
}

func TestAddRegistryWithTarget(t *testing.T) {
	cacheDir := t.TempDir()

	// Set opts at sha ref
	addOpts := &AddOpts{
		cachePath:    cacheDir,
		RegistryName: "with-target",
		PackName:     "traefik",
		Source:       testRegistryURL,
	}

	cache, err := NewCache(&CacheConfig{
		Path:   cacheDir,
		Logger: logging.NewTestLogger(t.Log),
	})
	require.NoError(t, err)
	require.NotNil(t, cache)

	// Add at ref
	registry, err := cache.Add(addOpts)
	require.NoError(t, err)
	require.NotNil(t, registry)

	require.Len(t, registry.Packs, 1)
}

func TestAddRegistryWithSHA(t *testing.T) {
	cacheDir := t.TempDir()
	// Set opts at sha ref
	addOpts := &AddOpts{
		cachePath:    cacheDir,
		RegistryName: "with-sha",
		Source:       testRegistryURL,
		Ref:          "a74b4e1",
	}

	cache, err := NewCache(&CacheConfig{
		Path:   cacheDir,
		Logger: logging.NewTestLogger(t.Log),
	})
	require.NoError(t, err)
	require.NotNil(t, cache)

	// Add at SHA
	registry, err := cache.Add(addOpts)
	require.NoError(t, err)
	require.NotNil(t, registry)

	// expected testCount
	expected := testPackCount(t, addOpts)

	require.Equal(t, expected, len(registry.Packs))
}

func TestAddRegistryWithRefAndPackName(t *testing.T) {
	cacheDir := t.TempDir()
	// Set opts at sha ref
	addOpts := &AddOpts{
		cachePath:    cacheDir,
		RegistryName: "with-ref-and-pack-name",
		PackName:     "traefik",
		Source:       testRegistryURL,
		Ref:          "a74b4e1",
	}

	cache, err := NewCache(&CacheConfig{
		Path:   cacheDir,
		Logger: logging.NewTestLogger(t.Log),
	})
	require.NoError(t, err)
	require.NotNil(t, cache)

	// Add Ref and PackName
	registry, err := cache.Add(addOpts)
	require.NoError(t, err)
	require.NotNil(t, registry)

	require.Len(t, registry.Packs, 1)
}

func TestAddRegistryNoCacheDir(t *testing.T) {
	opts := testAddOpts("no-cache-dir")

	cache, err := NewCache(&CacheConfig{
		Path:   "",
		Logger: logging.NewTestLogger(t.Log),
	})
	require.Error(t, err)

	registry, err := cache.Add(opts)

	require.Error(t, err)
	require.Nil(t, registry)
	require.Equal(t, errors.ErrCachePathRequired, err)
}

func TestAddRegistryNoSource(t *testing.T) {
	cacheDir := t.TempDir()
	opts := testAddOpts("no-source")
	opts.Source = ""

	cache, err := NewCache(&CacheConfig{
		Path:   cacheDir,
		Logger: logging.NewTestLogger(t.Log),
	})
	require.NoError(t, err)
	require.NotNil(t, cache)

	registry, err := cache.Add(opts)

	require.Error(t, err)
	require.Nil(t, registry)
	require.Equal(t, errors.ErrRegistrySourceRequired, err)
}

func TestDeleteRegistry(t *testing.T) {
	cacheDir := t.TempDir()
	opts := testAddOpts("delete-registry")

	cache, err := NewCache(&CacheConfig{
		Path:   cacheDir,
		Logger: logging.NewTestLogger(t.Log),
	})
	require.NoError(t, err)
	require.NotNil(t, cache)

	registry, err := cache.Add(opts)
	require.NoError(t, err)
	require.NotNil(t, registry)

	deleteOpts := &DeleteOpts{
		RegistryName: opts.RegistryName,
		PackName:     "",
		Ref:          "",
	}

	err = cache.Delete(deleteOpts)
	require.NoError(t, err)

	// test that registry is gone
	registryEntries, err := os.ReadDir(cacheDir)
	require.NoError(t, err)

	for _, registryEntry := range registryEntries {
		require.NotEqual(t, registryEntry.Name(), deleteOpts.RegistryName)
	}
}

func TestDeletePack(t *testing.T) {
	cacheDir := t.TempDir()
	opts := testAddOpts("delete-pack")

	cache, err := NewCache(&CacheConfig{
		Path:   cacheDir,
		Logger: logging.NewTestLogger(t.Log),
	})
	require.NoError(t, err)
	require.NotNil(t, cache)

	registry, err := cache.Add(opts)
	require.NoError(t, err)
	require.NotNil(t, registry)

	packCount := testPackCount(t, opts)

	deleteOpts := &DeleteOpts{
		RegistryName: opts.RegistryName,
		PackName:     "traefik",
		Ref:          opts.Ref,
	}

	err = cache.Delete(deleteOpts)
	require.NoError(t, err)

	// test that pack is gone
	registryEntries, err := os.ReadDir(opts.RegistryPath())
	require.NoError(t, err)

	for _, packEntry := range registryEntries {
		require.NotEqual(t, packEntry.Name(), deleteOpts.RegistryName)
	}

	// test that registry still exists, and other packs are still  there.
	require.NotEqual(t, 0, packCount)
	require.Equal(t, packCount-1, len(registryEntries))
}

func TestDeletePackByRef(t *testing.T) {
	cacheDir := t.TempDir()
	opts := testAddOpts("delete-pack-by-ref")

	cache, err := NewCache(&CacheConfig{
		Path:   cacheDir,
		Logger: logging.NewTestLogger(t.Log),
	})
	require.NoError(t, err)
	require.NotNil(t, cache)

	registry, err := cache.Add(opts)
	require.NoError(t, err)
	require.NotNil(t, registry)

	// Now add at different ref
	opts.Ref = "a74b4e1"
	registry, err = cache.Add(opts)
	require.NoError(t, err)
	require.NotNil(t, registry)

	packCount := testPackCount(t, opts)

	deleteOpts := &DeleteOpts{
		RegistryName: opts.RegistryName,
		PackName:     "traefik",
		Ref:          opts.Ref,
	}

	err = cache.Delete(deleteOpts)
	require.NoError(t, err)

	// test that pack is gone
	registryEntries, err := os.ReadDir(opts.RegistryPath())
	require.NoError(t, err)

	for _, packEntry := range registryEntries {
		require.NotEqual(t, packEntry.Name(), deleteOpts.RegistryName)
	}

	// test that registry still exists, and other packs are still  there.
	require.NotEqual(t, 0, packCount)
	require.Equal(t, packCount-1, len(registryEntries))
}
