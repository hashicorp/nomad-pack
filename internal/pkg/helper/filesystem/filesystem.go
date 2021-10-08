package filesystem

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
)

// CopyFile copies a file from one path to another
func CopyFile(sourcePath, destinationPath string, log func(string)) error {
	var err error

	// Open the source file
	sourceFile, err := os.Open(sourcePath)
	if err != nil {
		log(fmt.Sprintf("error opening source file: %s", err))
		return err
	}
	// Set up a deferred close handler
	defer func() {
		if err = sourceFile.Close(); err != nil {
			log(fmt.Sprintf("error closing source file: %s", err))
		}
	}()

	// Open the destination file
	destinationFile, err := os.Create(destinationPath)
	if err != nil {
		log(fmt.Sprintf("error opening destination file: %s", err))
		return err
	}
	// Set up a deferred close handler
	defer func() {
		if err = destinationFile.Close(); err != nil {
			log(fmt.Sprintf("error closing destination file: %s", err))
		}
	}()

	// Copy the file
	_, err = io.Copy(destinationFile, sourceFile)
	if err != nil {
		log(fmt.Sprintf("error copying file: %s", err))
		return err
	}

	// Sync the file contents
	err = destinationFile.Sync()
	if err != nil {
		log(fmt.Sprintf("error syncing destination file: %s", err))
		return err
	}

	// Get the source file info so we can copy the permissions
	sourceFileInfo, err := os.Stat(sourcePath)
	if err != nil {
		log(fmt.Sprintf("error getting source file info: %s", err))
		return err
	}

	// Set the destination file permissions from the source file mode
	err = os.Chmod(destinationPath, sourceFileInfo.Mode())
	if err != nil {
		log(fmt.Sprintf("error getting setting destination file permissions: %s", err))
		return err
	}

	// Give the defer functions a chance to set this variable
	return err
}

// CopyDir recursively copies a directory.
func CopyDir(sourceDir string, destinationDir string, log func(string)) error {
	// Clean the directory paths
	sourceDir = filepath.Clean(sourceDir)
	destinationDir = filepath.Clean(destinationDir)

	// Get the source directory info to validate that it is a directory
	sourceDirInfo, err := os.Stat(sourceDir)
	if err != nil {
		log(fmt.Sprintf("error getting source directory info: %s", err))
		return err
	}

	// Throw error if not a directory
	if !sourceDirInfo.IsDir() {
		return fmt.Errorf("source is not a directory")
	}

	// Make sure the destination directory doesn't already exist
	_, err = os.Stat(destinationDir)
	if err != nil && !os.IsNotExist(err) {
		log(fmt.Sprintf("error getting destination file info: %s", err))
		return err
	}
	// throw error if it does exist
	if err == nil {
		return fmt.Errorf("destination already exists")
	}

	// Make the destination direction and copy the file permissions
	err = os.MkdirAll(destinationDir, sourceDirInfo.Mode())
	if err != nil {
		log(fmt.Sprintf("error creating destination directory: %s", err))
		return err
	}

	// Read the contents of the source directory
	sourceEntries, err := os.ReadDir(sourceDir)
	if err != nil {
		log(fmt.Sprintf("error reading source directory entries: %s", err))
		return err
	}

	// Iterate over all the directory entries and copy them
	for _, sourceEntry := range sourceEntries {
		// Build the source and destination paths
		sourcePath := filepath.Join(sourceDir, sourceEntry.Name())
		destinationPath := filepath.Join(destinationDir, sourceEntry.Name())

		// If a directory, then recurse, else copy all files
		if sourceEntry.IsDir() {
			err = CopyDir(sourcePath, destinationPath, log)
			if err != nil {
				return err
			}
		} else {
			// Skip symlinks.
			if sourceEntry.Type()&os.ModeSymlink != 0 {
				continue
			}

			// Copy file from source directory to destination directory
			err = CopyFile(sourcePath, destinationPath, log)
			if err != nil {
				return err
			}
		}
	}

	return nil
}
