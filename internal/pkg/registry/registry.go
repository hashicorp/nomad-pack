package registry

import (
	"errors"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"strings"

	gg "github.com/hashicorp/go-getter"
	"github.com/hashicorp/nomad-pack/internal/pkg/loader"
	"github.com/hashicorp/nomad-pack/pkg/pack"
)

// Registry represents a registry definition.
type Registry struct {
	Name  string
	URL   string
	Packs []*pack.Pack
}

// LoadAllFromCache loads a list of registries from a cache path. It assumes each
// directory in the specified path cache is a registry.
func LoadAllFromCache(cachePath string) ([]*Registry, error) {
	if cachePath == "" {
		return nil, errors.New("cachePath is required")
	}

	registryEntries, err := os.ReadDir(cachePath)
	if err != nil {
		return nil, err
	}

	var registries []*Registry

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
		registry, err := LoadFromPath(path.Join(cachePath, registryEntry.Name()))
		if err != nil {
			return nil, err
		}

		registries = append(registries, registry)
	}

	return registries, nil
}

// LoadFromPath loads a Registry struct and all its packs from a registry path.
func LoadFromPath(registryPath string) (*Registry, error) {
	// Load the list of packs for the registry.
	packEntries, err := os.ReadDir(registryPath)
	if err != nil {
		return nil, err
	}

	registry := &Registry{
		Name:  filepath.Base(registryPath),
		Packs: make([]*pack.Pack, 0),
	}

	// Iterate over the packs in the registry and load each pack so that
	// we can extract information from the metadata.
	for _, packEntry := range packEntries {
		// Skip files and the .git folder since this will be a git repo
		if !packEntry.IsDir() || packEntry.Name() == ".git" {
			continue
		}

		// Load the pack
		registryPack, err := loader.Load(path.Join(registryPath, packEntry.Name()))
		if err != nil {
			return nil, err
		}

		// Add pack to registry
		registry.Packs = append(registry.Packs, registryPack)
	}

	// throw error if registry contains no packs
	if len(registry.Packs) == 0 {
		return nil, fmt.Errorf("registry %s contains no packs", registry.Name)
	}

	// Set the registry URL from the first pack's URL
	registry.URL = registryURL(registry.Packs[0].Metadata.Pack.URL)

	return registry, nil
}

// LoadFromURL loads a registry from a remote git repository. If addToCache is
// true, the registry will also be added to the global cache. The cache directory
// must be specified to allow user customization of cacheLocation. If a name is
// specified, the registry will be added with that alias, otherwise the registry
// URL slug will be used.
func LoadFromURL(url string, cacheDir, name string) (*Registry, error) {
	if cacheDir == "" {
		return nil, errors.New("cacheDir is required")
	}

	if name == "" {
		name = slugifyRegistryURL(url)
	}

	// TODO: Implement versioning on folder names - must detect // for subdirectories
	err := gg.Get(cacheDir, url)
	if err != nil {
		return nil, fmt.Errorf("could not install %s registry: %s", name, url)
	}

	return nil, errors.New("Registry.LoadFromURL is not implemented")
}

// Converts a registry URL to a slug which can be used as the default registry name.
func slugifyRegistryURL(url string) string {
	slug := registryURL(url)
	urlParts := strings.SplitN(slug, "/", 1)
	if len(urlParts) > 1 {
		slug = urlParts[1]
	}
	return strings.Replace(slug, "/", "-", -1)
}

// Naively returns a URL string from a pack URL. Assumes the last segment is the
// pack name and the registry URL is everything before that.
func registryURL(url string) string {
	return url[:strings.LastIndex(url, "/")]
}
