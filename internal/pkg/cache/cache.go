// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package cache

import (
	"fmt"
	"os"
	"path"
	"strings"

	"github.com/go-git/go-git/v5"

	"github.com/hashicorp/nomad-pack/internal/pkg/errors"
	"github.com/hashicorp/nomad-pack/internal/pkg/helper/filesystem"
	"github.com/hashicorp/nomad-pack/internal/pkg/logging"
)

const (
	DefaultRegistryName = "default"
	DefaultRef          = "latest"
	DevRegistryName     = "<<local folder>>"
	DevRef              = "<<none>>"
	DefaultDirPerms     = 0700
)

// NewCache instantiates a new cache instance with the specified config. If no
// config is provided, the cache is initialized with default configuration.
func NewCache(cfg *CacheConfig) (cache *Cache, err error) {
	if cfg == nil {
		cfg = defaultCacheConfig()
	}

	cache = &Cache{
		cfg:          cfg,
		ErrorContext: errors.NewErrorContext(),
	}

	cache.ErrorContext.Add(errors.RegistryContextPrefixCachePath, cfg.Path)

	err = cache.ensureGlobalCache()
	if err != nil {
		return
	}

	if cfg.Eager {
		err = cache.Load()
	}

	return
}

func (c *Cache) ensureGlobalCache() error {
	return filesystem.MaybeCreateDestinationDir(c.cfg.Path)
}

// DefaultCachePath returns the default cache path.
func DefaultCachePath() string {
	cacheDir, err := os.UserCacheDir()
	if err != nil {
		homeDir, err := os.UserHomeDir()
		if err != nil {
			homeDir = "~"
		}
		return path.Join(homeDir, ".nomad/packs")
	}
	return path.Join(cacheDir, "nomad/packs")
}

func defaultCacheConfig() *CacheConfig {
	return &CacheConfig{
		Path:   DefaultCachePath(),
		Logger: logging.Default(),
	}
}

// Cache encapsulates the state and functionality of a Cache of registries
type Cache struct {
	cfg        *CacheConfig
	registries []*Registry
	// latestSHA keeps the ref to the last clone operation (if any)
	latestSHA string
	// ErrorContext stores any errors that were encountered along the way so that
	// error handling can be dealt with in one place.
	ErrorContext *errors.ErrorContext
}

// CacheConfig encapsulates the configuration options for a cache instance.
type CacheConfig struct {
	Path   string
	Eager  bool
	Logger logging.Logger
}

// cacheOperationProvider provides an interface for the Opts family of structs
// that are used to perform cache operations. The logic may vary slightly based
// on the operation being performed. See IsTarget for good example of variance.
type cacheOperationProvider interface {
	RegistryPath() string
	PackPath() string
	PackDir() string
	ForPackName() string
	AtRef() string
	IsLatest() bool
	IsTarget(entry os.DirEntry) bool
}

// clonePath returns the path where remote repositories will be cloned to during
// download processing.
func (c *Cache) clonePath() string {
	return path.Join(c.cfg.Path, tmpDir)
}

// clonedPacksPath returns the path where remote repository packs have been cloned
// to during download processing. This enforces the hard convention that there
// must be a packs directory in the registry.
func (c *Cache) clonedPacksPath() string {
	return path.Join(c.cfg.Path, tmpDir, "packs")
}

// Registries is an accessor for the cached registries contain within the cache instance.
func (c *Cache) Registries() []*Registry {
	if len(c.registries) == 0 {
		err := c.Load()
		if err != nil {
			c.cfg.Logger.ErrorWithContext(err, "error loading Registries", c.ErrorContext.GetAll()...)
		}
	}
	return c.registries
}

// Packs is an accessor for the cached packs contains within the cache instance.
func (c *Cache) Packs() (packs []*Pack) {
	packs = make([]*Pack, 0)

	for _, registry := range c.Registries() {
		packs = append(packs, registry.Packs...)
	}

	return
}

// Load loads a list of registries from a cache path. It assumes each
// directory in the specified path cache is a registry.
func (c *Cache) Load() (err error) {
	c.ErrorContext.Add(errors.RegistryContextPrefixCachePath, c.cfg.Path)

	if c.cfg.Path == "" {
		err = errors.ErrCachePathRequired
		return
	}

	// Load the list of registry entries
	registryEntries, err := os.ReadDir(c.cfg.Path)
	if err != nil {
		return
	}

	// Initialize an opts flyweight
	opts := &GetOpts{
		cachePath: c.cfg.Path,
	}

	// Iterate over the registries and build a registry/pack for each entry at each ref.
	for _, registryEntry := range registryEntries {
		// ignore the .git folder which will be present since these are all git repos
		if registryEntry.Name() == ".git" {
			continue
		}

		// Don't process files in the registry folder e.g. README.md
		if !registryEntry.IsDir() {
			continue
		}

		// Process all refs
		registryRefs, err2 := os.ReadDir(path.Join(c.cfg.Path, registryEntry.Name()))
		if err2 != nil {
			return
		}
		for _, registryRef := range registryRefs {
			opts.RegistryName = registryEntry.Name()
			opts.Ref = registryRef.Name()

			// Load the registry from the path
			var registry *Registry
			registry, err = c.Get(opts)
			if err != nil {
				return
			}

			c.registries = append(c.registries, registry)
		}
	}

	return
}

// VerifyPackExists verifies that a pack exists at the specified path.
func VerifyPackExists(cfg *PackConfig, errCtx *errors.UIErrorContext, logger logging.Logger) (err error) {
	if _, err = os.Stat(cfg.Path); os.IsNotExist(err) {
		logger.ErrorWithContext(err, "failed to find pack", errCtx.GetAll()...)
		return
	}

	return
}

// AppendRef is a utility function to format a pack name at a specific ref.
func AppendRef(name, ref string) string {
	if ref == "" || ref == DevRef {
		return name
	}
	return fmt.Sprintf("%s@%s", name, ref)
}

// This is a utility method to parse the ref from the pack entry
func refFromPackEntry(packEntry os.DirEntry) (ref string) {
	ref = "unknown"

	segments := strings.Split(packEntry.Name(), "@")
	if len(segments) == 2 {
		ref = segments[1]
	}

	return
}

// getGitHeadRef is a helper method that takes a directory which is a git
// repository, and returns the SHA of the git HEAD of that repository.
func getGitHeadRef(clonePath string) (string, error) {
	r, err := git.PlainOpen(clonePath)
	if err != nil {
		return "n/a", fmt.Errorf("could not read cloned repository: %v", err)
	}
	head, err := r.Head()
	if err != nil {
		return "n/a", fmt.Errorf("could not get ref of a cloned repository: %v", err)
	}

	return head.Hash().String(), nil
}
