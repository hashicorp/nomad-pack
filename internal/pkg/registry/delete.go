package registry

import (
	"errors"
	"fmt"
	"os"
	"path"
	"strings"
)

// DeleteFromCache deletes a registry from the specified global cache directory.
// If the name includes a @version component, only packs matching that version
// will be deleted. If a target is specified, only packs matching that target
// will be deleted.  Version and target are additive.
func DeleteFromCache(cacheDir, name, target string, log func(string)) error {
	var err error
	// if cache directory is empty return error
	if cacheDir == "" {
		return errors.New("cache directory is required")
	}

	// if name is empty return error
	if name == "" {
		return errors.New("registry name is required")
	}

	// parse the pack name and version from the pack@version format.
	packName, version, err := ParsePackNameAndVersion(name)

	// Read the cache directory
	registryEntries, err := os.ReadDir(cacheDir)
	if err != nil {
		log(fmt.Sprintf("error reading cache directory %s: %s", cacheDir, err))
		return err
	}

	// Iterate over the entries in the cache directory
	for _, registryEntry := range registryEntries {
		// if not a directory, or the directory name doesn't equal the registry name, skip.
		if !registryEntry.IsDir() || registryEntry.Name() != packName {
			continue
		}

		// Set the registry directory
		registryDir := path.Join(cacheDir, packName)

		// If no specific target is set, and no version is set, delete the entire
		// registry and exit.
		if target == "" && version == "" {
			err = os.RemoveAll(registryDir)
			if err != nil {
				log(fmt.Sprintf("error deleting directory %s: %s", registryDir, err))
				return err
			}

			log(fmt.Sprintf("deleted pack %s", packName))
			return nil
		}

		err = deletePacks(registryDir, target, version, log)
		if err != nil {
			return err
		}
	}

	return nil
}

func deletePacks(registryDir, target, version string, log func(string)) error {
	// load the pack entries for the registry
	packEntries, err := os.ReadDir(registryDir)
	if err != nil {
		log(fmt.Sprintf("error reading registry directory %s: %s", registryDir, err))
		return err
	}

	deleteCount := 0

	// iterate over each pack
	for _, packEntry := range packEntries {
		// if not a directory, then skip.
		if !packEntry.IsDir() {
			continue
		}

		// if the pack entry contains both the target and the version then
		// delete it. By this point one or the other must be set, so this
		// actually handles the case where only one is set also, because
		// strings.Contains will evaluate to true on an empty string.
		if strings.Contains(packEntry.Name(), target) && strings.Contains(packEntry.Name(), version) {
			packDir := path.Join(registryDir, packEntry.Name())
			err = os.RemoveAll(packDir)
			if err != nil {
				log(fmt.Sprintf("error deleting pack %s: %s", packEntry.Name(), err))
				return err
			}

			log(fmt.Sprintf("deleted pack %s", packDir))
			deleteCount += 1
		}
	}

	if deleteCount == 0 {
		return errors.New("error deleting packs - no packs found matching arguments")
	}
	return nil
}
