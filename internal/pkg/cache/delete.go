package cache

import (
	stdErrors "errors"
	"fmt"
	"os"
	"path"
	"strings"

	"github.com/hashicorp/nomad-pack/internal/pkg/errors"
)

// Delete deletes a registry from the specified global cache directory.
// If the name includes a @ref component, only packs matching that ref
// will be deleted. If a target is specified, only packs matching that target
// will be deleted.  Ref and target are additive.
func (c *Cache) Delete(opts *DeleteOpts) (err error) {
	logger := c.cfg.Logger
	opts.cachePath = c.cfg.Path

	// if cache directory is empty return error
	if opts.cachePath == "" {
		err = stdErrors.New("cache path is required")
		return
	}

	// if registry name is empty set to default
	if opts.RegistryName == "" {
		opts.RegistryName = DefaultRegistryName
	}

	c.ErrorContext.Add(errors.RegistryContextPrefixCachePath, opts.cachePath)
	c.ErrorContext.Add(errors.RegistryContextPrefixRegistryName, opts.RegistryName)
	c.ErrorContext.Add(errors.RegistryContextPrefixPackName, opts.PackName)
	c.ErrorContext.Add(errors.RegistryContextPrefixRef, opts.Ref)

	// If no pack name or revision is set, delete the whole registry and return.
	if opts.PackName == "" && opts.Ref == "" {
		err = os.RemoveAll(opts.RegistryPath())
		if err != nil {
			logger.ErrorWithContext(err, "error deleting full registry", c.ErrorContext.GetAll()...)
		}
		return
	}

	deleteCount := 0
	// Read each cached registry
	packEntries, err := os.ReadDir(opts.RegistryPath())
	if err != nil {
		logger.ErrorWithContext(err, "error deleting cached registry", c.ErrorContext.GetAll()...)
		return
	}

	// Iterate over pack entries and delete all entries targeted for deletion.
	for _, packEntry := range packEntries {
		if !opts.IsTarget(packEntry) {
			continue
		}

		err = os.RemoveAll(path.Join(opts.RegistryPath(), packEntry.Name()))
		if err != nil {
			logger.ErrorWithContext(err, "error deleting pack", c.ErrorContext.GetAll()...)
			return
		}

		logger.Debug(fmt.Sprintf("deleted pack %s", packEntry.Name()))
		deleteCount += 1
	}

	// If nothing got deleted, throw an error.
	if deleteCount == 0 {
		err = stdErrors.New("error deleting packs")
		logger.ErrorWithContext(err, "no packs found matching arguments", c.ErrorContext.GetAll()...)
		return
	}

	// Check to see if there is anything left in the directory.
	packEntries, err = os.ReadDir(opts.RegistryPath())
	if err != nil {
		logger.ErrorWithContext(err, "error reading cached registry", c.ErrorContext.GetAll()...)
		return
	}

	// If no packs are left, delete the entire registry.
	if len(packEntries) == 0 {
		err = os.RemoveAll(opts.RegistryPath())
		if err != nil {
			logger.ErrorWithContext(err, "error deleting empty registry", c.ErrorContext.GetAll()...)
		}
	}

	return
}

// DeleteOpts are the arguments that are required to delete a registry or pack
// from the cache.
type DeleteOpts struct {
	// Path to the cache containing the registry. Must be set by cache after opts
	// are passed.
	cachePath string
	// Name or alias of the registry the delete operation will be performed against.
	RegistryName string
	// Optional pack name to delete when deleting a specific pack from the cache.
	PackName string
	// Optional ref of pack or registry at which to delete. Ignored it not
	// specifying a git source. Defaults to latest.
	Ref string
}

// RegistryPath fulfills the cacheOperationProvider interface for DeleteOpts
func (opts *DeleteOpts) RegistryPath() string {
	return path.Join(opts.cachePath, opts.RegistryName)
}

// PackPath fulfills the cacheOperationProvider interface for DeleteOpts
func (opts *DeleteOpts) PackPath() (packPath string) {
	packPath = path.Join(opts.cachePath, opts.RegistryName)

	// Append the revision if set.
	if opts.Ref != "" {
		packPath = path.Join(packPath, opts.PackDir())
	}
	return
}

// PackDir fulfills the cacheOperationProvider interface for DeleteOpts
func (opts *DeleteOpts) PackDir() string {
	if opts.Ref != "" {
		return AppendRef(opts.PackName, opts.Ref)
	}
	return opts.PackName
}

// AtRevision fulfills the cacheOperationProvider interface for DeleteOpts
func (opts *DeleteOpts) AtRef() string {
	return opts.Ref
}

// ForPackName fulfills the cacheOperationProvider interface for DeleteOpts
func (opts *DeleteOpts) ForPackName() string {
	return opts.PackName
}

// IsLatest fulfills the RegistryOptsProviderInterface for DeleteOpts
func (opts *DeleteOpts) IsLatest() bool {
	return opts.Ref == "" || opts.Ref == "latest"
}

// IsTarget fulfills the RegistryOptsProviderInterface for DeleteOpts
func (opts *DeleteOpts) IsTarget(dirEntry os.DirEntry) bool {
	// If no pack name is set, then check if this directory contains the
	// @ref string. If so, it is a target.
	if opts.PackName == "" {
		return strings.Contains(dirEntry.Name(), AppendRef("", opts.Ref))
	}

	return dirEntry.Name() == opts.PackDir()
}
