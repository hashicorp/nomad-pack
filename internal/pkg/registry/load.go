package registry

import (
	"errors"
	"os"
	"path"
	"path/filepath"
)

// LoadAllFromCache loads a list of registries from a cache path. It assumes each
// directory in the specified path cache is a registry.
func LoadAllFromCache(cachePath string) ([]*CachedRegistry, error) {
	if cachePath == "" {
		return nil, errors.New("cachePath is required")
	}

	// Load the list of registry entries
	registryEntries, err := os.ReadDir(cachePath)
	if err != nil {
		return nil, err
	}

	var registries []*CachedRegistry
	// Iterate over the registries and build a registry/pack for each entry at each version.
	for _, registryEntry := range registryEntries {
		// ignore the .git folder which will be present since these are all git repos
		if registryEntry.Name() == ".git" {
			continue
		}

		// Don't process files in the registry folder e.g. README.md
		if !registryEntry.IsDir() {
			continue
		}

		// Load the registry from the path
		registry, err := LoadFromCache(path.Join(cachePath, registryEntry.Name()))
		if err != nil {
			return nil, err
		}

		registries = append(registries, registry)
	}

	return registries, nil
}

// LoadFromCache loads a CachedRegistry struct and all its packs from a registry path.
func LoadFromCache(registryPath string) (*CachedRegistry, error) {
	// Load the list of packs for the registry.
	packEntries, err := os.ReadDir(registryPath)
	if err != nil {
		return nil, err
	}

	cachedRegistry := &CachedRegistry{
		Name:  filepath.Base(registryPath),
		Packs: make([]*CachedPack, 0),
	}

	// Iterate over the packs in the registry and load each pack so that
	// we can extract information from the metadata.
	for _, packEntry := range packEntries {
		// Skip files and the .git folder since this will be a git repo
		if !packEntry.IsDir() || packEntry.Name() == ".git" {
			continue
		}

		// Load the pack to registry
		err = cachedRegistry.LoadCachedPack(path.Join(registryPath, packEntry.Name()))
		if err != nil {
			return nil, err
		}
	}

	// Set the registry URL from the first pack's URL if a pack exists
	cachedRegistry.setURLFromPacks()

	return cachedRegistry, nil
}
