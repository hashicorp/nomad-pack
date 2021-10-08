package registry

import (
	"errors"
	"fmt"
	"os"
	"path"
	"time"

	gg "github.com/hashicorp/go-getter"
	"github.com/hashicorp/nomad-pack/internal/pkg/helper/filesystem"
	"github.com/hashicorp/nomad-pack/internal/pkg/loader"
	pkgVersion "github.com/hashicorp/nomad-pack/internal/pkg/version"
)

const tmpDir = "nomad-pack-tmp"

// AddFromGitURL loads a registry from a remote git repository. If addToCache is
// true, the registry will also be added to the global cache. The cache directory
// must be specified to allow user customization of cacheLocation. If a name is
// specified, the registry will be added with that alias, otherwise the registry
// URL slug will be used.
func AddFromGitURL(cacheDir, url, alias, target string, log func(string)) (*CachedRegistry, error) {
	var err error
	// Throw error if cacheDir not defined
	if cacheDir == "" {
		return nil, errors.New("cache directory is required")
	}

	if url == "" {
		return nil, errors.New("registry url is required")
	}

	log(fmt.Sprintf("Attempting to add remote registry %s", url))

	if target != "" {
		log(fmt.Sprintf("Targeting specific pack %s", target))
	}

	// Resolve registryName to either alias or url slug
	registryName, err := resolveRegistryName(alias, url)
	if err != nil {
		log("error resolving default registry name")
		return nil, err
	}

	log(fmt.Sprintf("Changes will be applied to registry named %s", registryName))

	// Parse the registry url so that we can set it on the return registry
	registryURL, err := parseRegistryURL(url)
	if err != nil {
		log("error parsing registry url")
		return nil, err
	}

	log(fmt.Sprintf("CachedRegistry base URL is %s", registryURL))

	// Parse version from git url
	version, err := ParseVersionFromGitURL(url)
	if err != nil {
		log("error parsing version from git URL")
		return nil, err
	}

	log(fmt.Sprintf("Target version is %s", version))

	cloneDir, err := CloneRemoteGithubRegistry(cacheDir, registryURL, target, version, log)
	if err != nil {
		return nil, err
	}

	// Set up a defer function so that the temp directory always gets removed
	defer func() {
		// remove the tmp directory
		err = os.RemoveAll(cloneDir)
		if err != nil {
			log(fmt.Sprintf("add completed with errors - %s directory not deleted: %s", cloneDir, err.Error()))

		}
		log("temp directory deleted")
	}()

	registryDir, err := buildRegistryDir(cacheDir, registryName, log)
	if err != nil {
		log(fmt.Sprintf("error building registy directory: %s", err))
		return nil, err
	}

	log(fmt.Sprintf("Processing pack entries at %s", cloneDir))

	// Move the cloned registry packs to the global cache with the version
	// as part of the folder registryName - appends the target if set to ensure
	// correct path
	packEntries, err := os.ReadDir(cloneDir)
	for _, packEntry := range packEntries {
		// Don't process the .git folder or any files
		if packEntry.Name() == ".git" || !packEntry.IsDir() {
			continue
		}

		log(fmt.Sprintf("found pack entry %s", packEntry.Name()))

		err = processPackEntry(registryDir, cloneDir, version, packEntry, log)
		if err != nil {
			log(fmt.Sprintf("error processing pack entry: %s", err))
			return nil, err
		}
	}

	cachedRegistry, err := LoadFromCache(registryDir)
	if err != nil {
		return nil, fmt.Errorf("error adding registry: %s", err)
	}

	return cachedRegistry, nil
}

// CloneRemoteGithubRegistry clones a remote git repository to the specified target directory
func CloneRemoteGithubRegistry(cacheDir, url, target, version string, log func(string)) (string, error) {
	var err error
	// Clone the repo to a temp directory so that we can stamp each dir with the version.
	cloneDir := path.Join(cacheDir, tmpDir)

	// Append the target pack to the go-getter url if a target was specified
	if target != "" {
		url = fmt.Sprintf("%s//%s", url, target)
	}

	// If version is set, add query string variable
	if version != "" && version != "latest" {
		url = fmt.Sprintf("%s?ref=%s", url, version)
	}

	log(fmt.Sprintf("go-getter URL is %s", url))

	// TODO: Review this go-getter URL requirement and test different scenarios.
	// Append the target if it exists so that the clone directory will maintain
	// the folder structure of the remote repository.
	err = gg.Get(path.Join(cloneDir, target), fmt.Sprintf("git::%s", url))
	if err != nil {
		log(fmt.Sprintf("error cloning registry %s to path %s", url, cloneDir))
		return "", fmt.Errorf("could not install registry %s to path %s: %s", url, cloneDir, err)
	}

	log(fmt.Sprintf("CachedRegistry successfully cloned at %s", cloneDir))

	// Return err so defer has a chance to set it
	return cloneDir, err
}

// Ensure that the registry directory exists, and return the registryName name joined to the cacheDir
func buildRegistryDir(cacheDir string, registryName string, log func(string)) (string, error) {
	registryDir := path.Join(cacheDir, registryName)

	if _, err := os.Stat(registryDir); err != nil {
		// If registry directory does not exist, create it
		if os.IsNotExist(err) {
			log(fmt.Sprintf("CachedRegistry directory not detected - creating at %s", registryDir))
			// Create the directory so that the owner can read and write. All other
			// users can only read.
			err = os.Mkdir(registryDir, 0755)
			if err != nil {
				log(fmt.Sprintf("error creating registry directory: %s", err))
				return "", err
			}
		} else {
			// If some other error return
			log(fmt.Sprintf("error checking for registry directory %s", registryDir))
			return "", err
		}
	}

	return registryDir, nil
}

func processPackEntry(registryDir, cloneDir, version string, packEntry os.DirEntry, log func(string)) error {
	log(fmt.Sprintf("Processing pack %s@%s", packEntry.Name(), version))

	// Set the final pack directory name to include the version stamp
	finalPackDir := path.Join(registryDir, fmt.Sprintf("%s@%s", packEntry.Name(), version))

	// Check if folder exists
	_, err := os.Stat(finalPackDir)
	if err != nil {
		// If an error other than not exists is thrown, rethrow it,
		// else the CopyDir operation below will create it.
		if !os.IsNotExist(err) {
			log(fmt.Sprintf("error checking pack directory: %s", err))
			return err
		}
	} else {
		// If version target is not latest, continue to next entry because version already exists
		if version != "latest" {
			log("Pack already exists at specified version - skipping")
			return nil
		}
	}

	log("Updating pack")

	// Build the tmpPackDir path
	tmpPackDir := path.Join(cloneDir, packEntry.Name())

	if version == "latest" {
		err = removePreviousLatest(finalPackDir, tmpPackDir, log)
		if err != nil {
			return err
		}
	}

	// Rename to move to registry directory with
	log(fmt.Sprintf("Writing pack to %s", finalPackDir))

	err = filesystem.CopyDir(tmpPackDir, finalPackDir, log)
	if err != nil {
		log(fmt.Sprintf("error copying cloned pack %s to %s", tmpPackDir, finalPackDir))
		return err
	}

	// Load the pack to the output registry
	log(fmt.Sprintf("Loading cloned pack from %s", finalPackDir))

	_, err = loader.Load(finalPackDir)
	if err != nil {
		log("error loading cloned pack")
	}

	// log a history of the latest version downloads - convenient for enabling users
	// to trace download of last known good version of latest. If version is
	// not latest, logLatest will exit without error.
	err = logLatest(version, finalPackDir, cloneDir, log)
	if err != nil {
		log("add completed with errors - unable to log latest history")
		// intentionally don't rethrow logging errors.
		// TODO: Support log levels
	}

	return nil
}

// Safely removes the previous latest version while preserving the log file
func removePreviousLatest(registryDir string, cloneDir string, log func(string)) error {
	log("Removing previous latest")

	err := backupLatestLogFile(registryDir, cloneDir, log)
	if err != nil {
		log("error backing up latest log file")
		return err
	}

	// Remove the current latest directory
	err = os.RemoveAll(registryDir)
	if err != nil {
		log("error removing previous latest directory")
		return err
	}
	return nil
}

// Backup the latest log file, if it exists, so it can be updated
// later - will get copied back later
func backupLatestLogFile(registryDir string, backupDir string, log func(string)) error {
	var err error
	latestLogFilePath := path.Join(registryDir, "latest.log")

	_, err = os.Stat(latestLogFilePath)
	if err == nil {
		err = filesystem.CopyFile(latestLogFilePath, path.Join(backupDir, "latest.log"), log)
		if err != nil {
			log("error backing up latest log")
			return err
		}
	} else if !os.IsNotExist(err) {
		// If some other error, rethrow
		log("error checking latest log file")
		return err
	}

	return nil
}

// Logs the history of latest updates so user can find last known good
// versions more easily
func logLatest(version, packDir, cloneDir string, log func(string)) error {
	var err error
	// only log for latest
	if version != "latest" {
		return nil
	}

	// Open the log for appending, and create it if it doesn't exist
	logFile, err := os.OpenFile(path.Join(packDir, "latest.log"), os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0755)
	if err != nil {
		log("error open latest log file")
		return err
	}
	// Set up a defer function to close the file on function exit
	defer func() {
		err = logFile.Close()
	}()

	// Calculate the SHA of the target pack
	log("Calculating SHA for latest")
	currentSHA, err := pkgVersion.GitSHA(cloneDir)
	if err != nil {
		log("error logging calculating SHA")
		return err
	}

	// Format a log entry with SHA and timestamp
	logEntry := fmt.Sprintf("SHA %s downloaded at UTC %s\n", currentSHA, time.Now().UTC())

	// Write log entry to file
	if _, err = logFile.WriteString(logEntry); err != nil {
		log("error appending to log")
		return err
	}

	// Return err so defer has a chance to set
	return err
}

// Resolve the registry name to either the alias or the url slug
func resolveRegistryName(alias, url string) (string, error) {
	if alias != "" {
		return alias, nil
	}
	return parseRegistrySlug(url)
}
