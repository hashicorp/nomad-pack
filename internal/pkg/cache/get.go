// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package cache

import (
	"os"
	"path"
	"strings"

	"github.com/hashicorp/nomad-pack/internal/pkg/errors"
)

// Get loads a Registry struct and all its packs from a registry path.
func (c *Cache) Get(opts *GetOpts) (registry *Registry, err error) {
	logger := c.cfg.Logger
	opts.cachePath = c.cfg.Path

	// Set up the error context with values from opts.
	c.ErrorContext.Add(errors.RegistryContextPrefixRegistryName, opts.RegistryName)
	c.ErrorContext.Add(errors.RegistryContextPrefixRef, opts.Ref)
	if opts.PackName != "" {
		c.ErrorContext.Add(errors.RegistryContextPrefixPackName, opts.PackName)
	}

	// Set mandatory defaults
	if opts.RegistryName == "" {
		opts.RegistryName = DefaultRegistryName
	}

	// If no errors, allocate the instance
	registry = &Registry{
		Name:  opts.RegistryName,
		Ref:   opts.Ref,
		Packs: make([]*Pack, 0),
	}

	err = registry.get(opts, c)
	if err != nil {
		logger.ErrorWithContext(err, "error getting registry packs", c.ErrorContext.GetAll()...)
		return
	}

	return
}

// GetOpts are the arguments are required to get a registry or pack from the cache.
type GetOpts struct {
	// Path to the cache containing the registry. Must be set by cache after opts
	// are passed.
	cachePath string
	// Optional Name or alias of the registry the get operation will be performed
	// against.
	RegistryName string
	// Optional name of pack to get from cache
	PackName string
	// Optional ref ov pack or registry to get from the cache.
	Ref string
}

// RegistryPath fulfills the cacheOperationProvider interface for GetOpts
func (opts *GetOpts) RegistryPath() string {
	return path.Join(opts.cachePath, opts.RegistryName, opts.Ref)
}

// PackPath fulfills the cacheOperationProvider interface for GetOpts
func (opts *GetOpts) PackPath() string {
	return path.Join(opts.cachePath, opts.RegistryName, opts.PackDir())
}

// PackDir fulfills the cacheOperationProvider interface for GetOpts
func (opts *GetOpts) PackDir() string {
	if opts.Ref != "" {
		return AppendRef(opts.PackName, opts.Ref)
	}
	return opts.PackName
}

// AtRevision fulfills the cacheOperationProvider interface for GetOpts
func (opts *GetOpts) AtRef() string {
	return opts.Ref
}

// ForPackName fulfills the cacheOperationProvider interface for GetOpts
func (opts *GetOpts) ForPackName() string {
	return opts.PackName
}

// IsLatest fulfills the RegistryOptsProviderInterface for GetOpts
func (opts *GetOpts) IsLatest() bool {
	return opts.Ref == "" || opts.Ref == "latest"
}

// IsTarget fulfills the RegistryOptsProviderInterface for GetOpts
func (opts *GetOpts) IsTarget(dirEntry os.DirEntry) bool {
	// TODO: Test with file paths.
	// If pack name is empty, everything at revision is a target.
	if opts.PackName == "" && strings.Contains(dirEntry.Name(), opts.Ref) {
		return true
	}

	// Otherwise, it's a target if the dirEntry.Name equals the formatted PackDir.
	return dirEntry.Name() == opts.PackDir()
}

// TODO: See if there is a better way
// This is a really hacky cheat because we may be doing a get without a pack name
// specified. So to avoid a bunch of string slicing, we rely on the IsTarget to
// have correctly evaluated that we should in fact load this.
func (opts *GetOpts) toPackDir(packEntry os.DirEntry) string {
	return path.Join(opts.RegistryPath(), packEntry.Name())
}
