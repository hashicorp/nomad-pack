// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package filesystem

import (
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/hashicorp/nomad-pack/internal/pkg/logging"
)

// CopyFile copies a file from one path to another
func CopyFile(sourcePath, destinationPath string, logger logging.Logger) (err error) {
	// Open the source file
	sourceFile, err := os.Open(sourcePath)
	if err != nil {
		logger.Debug(fmt.Sprintf("error opening source file: %s", err))
		return
	}

	// Set up a deferred close handler
	defer func() {
		if err = sourceFile.Close(); err != nil {
			logger.Debug(fmt.Sprintf("error closing source file: %s", err))
		}
	}()

	// Open the destination file
	destinationFile, err := os.Create(destinationPath)
	if err != nil {
		logger.Debug(fmt.Sprintf("error opening destination file: %s", err))
		return
	}
	// Set up a deferred close handler
	defer func() {
		if err = destinationFile.Close(); err != nil {
			logger.Debug(fmt.Sprintf("error closing destination file: %s", err))
		}
	}()

	// Copy the file
	_, err = io.Copy(destinationFile, sourceFile)
	if err != nil {
		logger.Debug(fmt.Sprintf("error copying file: %s", err))
		return
	}

	// Sync the file contents
	err = destinationFile.Sync()
	if err != nil {
		logger.Debug(fmt.Sprintf("error syncing destination file: %s", err))
		return
	}

	// Get the source file info so we can copy the permissions
	sourceFileInfo, err := os.Stat(sourcePath)
	if err != nil {
		logger.Debug(fmt.Sprintf("error getting source file info: %s", err))
		return
	}

	// Set the destination file permissions from the source file mode
	err = os.Chmod(destinationPath, sourceFileInfo.Mode())
	if err != nil {
		logger.Debug(fmt.Sprintf("error getting setting destination file permissions: %s", err))
		return
	}

	// Give the defer functions a chance to set this variable
	return
}

// CopyDir recursively copies a directory.
func CopyDir(sourceDir string, destinationDir string, logger logging.Logger) (err error) {
	// Clean the directory paths
	sourceDir = filepath.Clean(sourceDir)
	destinationDir = filepath.Clean(destinationDir)

	// Get the source directory info to validate that it is a directory
	sourceDirInfo, err := os.Stat(sourceDir)
	if err != nil {
		logger.Debug(fmt.Sprintf("error getting source directory info: %s", err))
		return
	}

	// Throw error if not a directory
	// TODO: Might need to handle symlinks.
	if !sourceDirInfo.IsDir() {
		err = fmt.Errorf("source is not a directory")
		logger.Debug(err.Error())
		return
	}

	// Make sure the destination directory doesn't already exist
	_, err = os.Stat(destinationDir)
	if err != nil && !os.IsNotExist(err) {
		logger.Debug(fmt.Sprintf("error getting destination file info: %s", err))
		return
	}
	// throw error if it does exist
	if err == nil {
		err = fmt.Errorf("destination already exists")
		logger.Debug(err.Error())
		return
	}

	// Make the destination direction and copy the file permissions
	err = MaybeCreateDestinationDir(
		destinationDir,
		WithFileMode(sourceDirInfo.Mode()),
		ErrOnExists(),
	)

	if err != nil {
		logger.Debug(fmt.Sprintf("error creating destination directory: %s", err))
		return
	}

	// Read the contents of the source directory
	sourceEntries, err := os.ReadDir(sourceDir)
	if err != nil {
		logger.Debug(fmt.Sprintf("error reading source directory entries: %s", err))
		return
	}

	// Iterate over all the directory entries and copy them
	for _, sourceEntry := range sourceEntries {
		// Build the source and destination paths
		sourcePath := filepath.Join(sourceDir, sourceEntry.Name())
		destinationPath := filepath.Join(destinationDir, sourceEntry.Name())

		// If a directory, then recurse, else copy all files
		if sourceEntry.IsDir() {
			err = CopyDir(sourcePath, destinationPath, logger)
			if err != nil {
				return
			}
		} else {
			// Skip symlinks.
			if sourceEntry.Type()&os.ModeSymlink != 0 {
				continue
			}

			// Copy file from source directory to destination directory
			err = CopyFile(sourcePath, destinationPath, logger)
			if err != nil {
				return
			}
		}
	}

	return nil
}
func MaybeCreateDestinationDir(path string, opts ...CreateOption) error {
	co := &createOpts{
		perms: 0755,
	}

	for _, opt := range opts {
		opt(co)
	}

	_, err := os.Stat(path)

	if err == nil && co.errOnExists {
		return &fs.PathError{
			Op:   "MaybeCreateDestinationDir",
			Path: path,
			Err:  fs.ErrExist,
		}
	}
	// If the directory doesn't exist, create it.
	if errors.Is(err, fs.ErrNotExist) {
		err := os.MkdirAll(path, co.perms)
		if err != nil {
			return err
		}
	}

	return nil
}

func WithFileMode(m fs.FileMode) CreateOption {
	return func(c *createOpts) {
		c.perms = m
	}
}

func ErrOnExists() CreateOption {
	return func(c *createOpts) {
		c.errOnExists = true
	}
}

type CreateOption func(c *createOpts)

type createOpts struct {
	errOnExists bool
	perms       fs.FileMode
}
