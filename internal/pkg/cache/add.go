// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package cache

import (
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path"
	"strings"
	"time"

	gg "github.com/hashicorp/go-getter"

	"github.com/hashicorp/nomad-pack/internal/pkg/errors"
	"github.com/hashicorp/nomad-pack/internal/pkg/helper/filesystem"
	pkgVersion "github.com/hashicorp/nomad-pack/internal/pkg/version"
)

const tmpDir = "nomad-pack-tmp"

// Add adds a registry to a cache from the passed config.
func (c *Cache) Add(opts *AddOpts) (cachedRegistry *Registry, err error) {
	// Throw error if cache path not defined
	if c.cfg.Path == "" {
		err = errors.ErrCachePathRequired
		return
	}

	opts.cachePath = c.cfg.Path

	// Set default if registry name is not defined.
	if opts.RegistryName == "" {
		opts.RegistryName = DefaultRegistryName
	}

	// Setup error context with input args
	c.ErrorContext.Add(errors.RegistryContextPrefixCachePath, c.cfg.Path)
	c.ErrorContext.Add(errors.RegistryContextPrefixRegistrySource, opts.Source)
	c.ErrorContext.Add(errors.RegistryContextPrefixRegistryName, opts.RegistryName)
	c.ErrorContext.Add(errors.RegistryContextPrefixRef, opts.Ref)
	c.ErrorContext.Add(errors.RegistryContextPrefixPackName, opts.PackName)

	// TODO: Ideally, if they've already added the registry, we should be able
	// to look up the source URL, but invalid packs metadata can mess that up.
	// Throw error if registry source is not defined
	if opts.Source == "" {
		err = errors.ErrRegistrySourceRequired
		return
	}

	cachedRegistry, err = c.addFromURI(opts)

	return
}

// addFromURI loads a registry from a remote git repository. If addToCache is
// true, the registry will also be added to the global cache. The cache directory
// must be specified to allow user customization of cache location. If a name is
// specified, the registry will be added with that alias, otherwise the registry
// URL slug will be used.
func (c *Cache) addFromURI(opts *AddOpts) (cachedRegistry *Registry, err error) {
	// Set the logger instance to reduce boilerplate.
	logger := c.cfg.Logger

	// Set default revision if not defined
	if opts.Ref == "" {
		opts.Ref = DefaultRef
	}

	// Set up a defer function so that the temp directory always gets removed
	defer func() {
		// remove the tmp directory
		if _, sErr := os.Stat(c.clonePath()); errors.Is(sErr, os.ErrNotExist) {
			return // there's nothing to clean up
		}

		err = os.RemoveAll(c.clonePath())
		if err != nil {
			logger.Debug(fmt.Sprintf("add completed with errors - %s directory not deleted: %s", c.clonePath(), err.Error()))
		}
		logger.Info("temp directory deleted")
	}()

	err = c.cloneRemoteGitRegistry(opts)
	if err != nil {
		return
	}

	logger.Debug(fmt.Sprintf("Processing pack entries at %s", c.clonePath()))

	// Move the cloned registry packs to the global cache.
	packEntries, err := os.ReadDir(c.clonedPacksPath())
	for _, packEntry := range packEntries {
		// Don't process the .git folder or any files
		// TODO: Handle symlinks
		if !opts.IsTarget(packEntry) {
			continue
		}

		logger.Debug(fmt.Sprintf("found pack entry %s", packEntry.Name()))

		// Make a new add opts for each pack so that we don't end up corrupting
		// the original opts.
		packOpts := &AddOpts{
			cachePath:    opts.cachePath,
			RegistryName: opts.RegistryName,
			PackName:     packEntry.Name(),
			Ref:          opts.Ref,
		}

		err = c.processPackEntry(packOpts, packEntry)
		if err != nil {
			logger.ErrorWithContext(err, "error processing pack entry", c.ErrorContext.GetAll()...)
			return
		}
	}

	cachedRegistry, err = c.Get(&GetOpts{
		RegistryName: opts.RegistryName,
		PackName:     opts.PackName,
		Ref:          opts.Ref,
	})
	if err != nil {
		logger.ErrorWithContext(err, "error getting registry after add", c.ErrorContext.GetAll()...)
		return
	}

	return
}

// cloneRemoteGitRegistry clones a remote git repository to the cache.
func (c *Cache) cloneRemoteGitRegistry(opts *AddOpts) (err error) {
	logger := c.cfg.Logger

	url := opts.Source

	// Append the pack name to the go-getter url if a pack name was specified
	if opts.PackName != "" {
		src := strings.TrimSuffix(opts.Source, ".git") // to make the next command work consistently
		url = fmt.Sprintf("%s.git//packs/%s", src, opts.PackName)
	}

	// If ref is set, add query string variable
	if !opts.IsLatest() {
		url = fmt.Sprintf("%s?ref=%s", url, opts.Ref)
	}

	logger.Debug(fmt.Sprintf("go-getter URL is %s", url))

	clonePath := c.clonePath()
	// If pack name is set, add an intermediary "packs" and pack dir manually.
	if opts.PackName != "" {
		clonePath = path.Join(clonePath, "packs", opts.PackName)
	}
	err = gg.Get(clonePath, fmt.Sprintf("git::%s", url))
	if err != nil {
		logger.ErrorWithContext(err, "could not install registry", c.ErrorContext.GetAll()...)
		return
	}

	// Get ref of our local repo clone and store it
	cmd := exec.Command("git", "rev-parse", "--short", "HEAD")
	cmd.Dir = clonePath
	stdout, err := cmd.Output()
	if err != nil {
		logger.ErrorWithContext(err, "could not get ref of a cloned repository", c.ErrorContext.GetAll()...)
		return
	}
	opts.LocalRef = string(stdout)

	logger.Debug(fmt.Sprintf("Registry successfully cloned at %s", c.clonePath()))

	return
}

func (c *Cache) processPackEntry(opts *AddOpts, packEntry os.DirEntry) error {
	logger := c.cfg.Logger
	logger.Debug(fmt.Sprintf("Processing pack %s@%s", packEntry.Name(), opts.Ref))

	// Check if folder exists
	_, err := os.Stat(opts.PackPath())
	if !errors.Is(err, fs.ErrNotExist) {
		logger.ErrorWithContext(err, "error checking pack directory", c.ErrorContext.GetAll()...)
		return err
	}

	// Here we could have err=fs.ErrNotExist or err=nil
	// Only look for latest when the pack path is found.
	if err == nil && !opts.IsLatest() {
		// If ref target is not latest, continue to next entry because ref already exists
		logger.Debug("Pack already exists at specified ref - skipping")
		return nil
	}

	logger.Debug("Updating pack")

	// If we are getting latest, backup previous safely so that we can keep the latest.log.
	if opts.IsLatest() {
		err := c.removePreviousLatest(opts)
		if err != nil {
			return err
		}
	}

	logger.Debug(fmt.Sprintf("Writing pack to %s", opts.PackPath()))

	if err := filesystem.CopyDir(opts.clonedPackPath(c), opts.PackPath(), c.cfg.Logger); err != nil {
		logger.ErrorWithContext(err, fmt.Sprintf("error copying cloned pack %s to %s", opts.clonedPackPath(c), opts.PackPath()))
		return err
	}

	// Load the pack to the output registry
	logger.Debug(fmt.Sprintf("Loading cloned pack from %s", opts.PackPath()))

	// log a history of the latest ref downloads - convenient for enabling users
	// to trace download of last known good ref of latest. If ref is
	// not latest, logLatest will exit without error.

	if err := c.logLatest(opts); err != nil {
		return err
	}

	return nil
}

// Safely removes the previous latest ref while preserving the log file
func (c *Cache) removePreviousLatest(opts *AddOpts) (err error) {
	logger := c.cfg.Logger

	logger.Debug("Removing previous latest")

	err = c.backupLatestLogFile(opts)
	if err != nil {
		return
	}

	// Remove the current latest directory
	err = os.RemoveAll(opts.PackPath())
	if err != nil {
		logger.ErrorWithContext(err, "error removing previous latest directory", c.ErrorContext.GetAll()...)
		return
	}
	return
}

// Backup the latest log file, if it exists, so it can be updated
// later - will get copied back later
func (c *Cache) backupLatestLogFile(opts *AddOpts) (err error) {
	logger := c.cfg.Logger
	latestLogFilePath := path.Join(opts.RegistryPath(), "latest.log")

	_, err = os.Stat(latestLogFilePath)
	if err == nil {
		// TODO: Verify this works as expected
		err = filesystem.CopyFile(latestLogFilePath, path.Join(c.clonePath(), opts.PackDir(), "latest.log"), logger)
		if err != nil {
			logger.ErrorWithContext(err, "error backing up latest log", c.ErrorContext.GetAll()...)
			return err
		}
	} else if !os.IsNotExist(err) {
		// If some other error, rethrow
		logger.ErrorWithContext(err, "error checking latest log file", c.ErrorContext.GetAll()...)
		return err
	}

	return nil
}

// Logs the history of latest updates so user can find last known good
// ref more easily
func (c *Cache) logLatest(opts *AddOpts) (err error) {
	logger := c.cfg.Logger

	// only log for latest
	if !opts.IsLatest() {
		return nil
	}

	// Open the log for appending, and create it if it doesn't exist
	logFile, err := os.OpenFile(path.Join(opts.PackPath(), "latest.log"), os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0755)
	if err != nil {
		logger.ErrorWithContext(err, "error open latest log file", c.ErrorContext.GetAll()...)
		return
	}
	// Set up a defer function to close the file on function exit
	defer func() {
		err = logFile.Close()
	}()

	// Calculate the SHA of the target pack
	logger.Debug("Calculating SHA for latest")
	// TODO: Test this is the right path after refactor.
	currentSHA, err := pkgVersion.GitSHA(c.clonePath())
	if err != nil {
		logger.Debug("error calculating SHA")
		return
	}

	// Format a log entry with SHA and timestamp
	logEntry := fmt.Sprintf("SHA %s downloaded at UTC %s\n", currentSHA, time.Now().UTC())

	// Write log entry to file
	if _, err = logFile.WriteString(logEntry); err != nil {
		logger.ErrorWithContext(err, "error appending to latest.log", c.ErrorContext.GetAll()...)
		return
	}

	return
}

// AddOpts are the arguments that are required to add a registry or pack to the cache.
type AddOpts struct {
	// Required cache patch. Must be set by cache after opts are passed.
	cachePath string
	// Required name for the registry. Used when managing a registry by a user defined name.
	RegistryName string
	// The well known location of a registry. Used when adding a registry. URL
	// or file directory currently supported.
	Source string
	// Optional target pack. Used when managing a specific pack within a registry.
	PackName string
	// Optional ref of pack or registry at which to add. Ignored it not
	// specifying a git source. Defaults to latest.
	Ref string
	// Ref of a locally cloned repository.
	LocalRef string
	// Optional username for basic auth to a registry that requires authentication.
	Username string
	// Optional password for basic auth to a registry that requires authentication.
	Password string
}

// RegistryPath fulfills the cacheOperationProvider interface for AddOpts
func (opts *AddOpts) RegistryPath() string {
	return path.Join(opts.cachePath, opts.RegistryName)
}

// PackPath fulfills the cacheOperationProvider interface for AddOpts
func (opts *AddOpts) PackPath() string {
	return path.Join(opts.cachePath, opts.RegistryName, opts.PackDir())
}

// PackDir fulfills the cacheOperationProvider interface for AddOpts
func (opts *AddOpts) PackDir() string {
	if opts.Ref != "" {
		return AppendRef(opts.PackName, opts.Ref)
	}
	return opts.PackName
}

// AtRef fulfills the cacheOperationProvider interface for AddOpts
func (opts *AddOpts) AtRef() string {
	return opts.Ref
}

// ForPackName fulfills the cacheOperationProvider interface for AddOpts
func (opts *AddOpts) ForPackName() string {
	return opts.PackName
}

// IsLatest fulfills the RegistryOptsProviderInterface for AddOpts
func (opts *AddOpts) IsLatest() bool {
	return opts.Ref == "" || opts.Ref == "latest"
}

// IsTarget fulfills the RegistryOptsProviderInterface for AddOpts
func (opts *AddOpts) IsTarget(dirEntry os.DirEntry) bool {
	// Not a target it's not a directory, or it's the .git directory.
	if !dirEntry.IsDir() || dirEntry.Name() == ".git" {
		return false
	}

	// If pack name is empty everything is a target, during add.
	if opts.PackName == "" {
		return true
	}

	// Otherwise, it's a target if the dirEntry.Name equals the formatted PackDir.
	return dirEntry.Name() == opts.PackName
}

// clonedPackPath is a helper that consistently resolves the clone location of
// pack within a cache.
func (opts *AddOpts) clonedPackPath(c *Cache) string {
	// Don't use PackDir here because we won't have the Revision on the cloned
	// directory name, thought we will append it to the registry entry if it is set.
	return path.Join(c.clonedPacksPath(), opts.PackName)
}
