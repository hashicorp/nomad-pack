package registry

import (
	"errors"
	"fmt"
	"strings"
)

// ParseVersionFromGitURL will extract a version (tag, release, SHA, latest) from
// a git URL. If no version is specified, it defaults to latest.
func ParseVersionFromGitURL(url string) (string, error) {
	// if not version specifier, return latest
	if !strings.Contains(url, "@") {
		return "latest", nil
	}

	// split the url on the version specifier character
	urlSegments := strings.Split(url, "@")
	if len(urlSegments) != 2 {
		// if multiple version specifiers in url, throw error
		return "", errors.New("invalid version specification")
	}

	// get the second segment to extract version
	return urlSegments[1], nil
}

// parseRegistrySlug converts a registry URL to a slug which can be used as the default registry name.
func parseRegistrySlug(url string) (string, error) {
	// parse the registry url
	slug, err := parseRegistryURL(url)
	if err != nil {
		return "", err
	}

	// split the url into segments
	urlParts := strings.SplitN(slug, "/", 1)
	if len(urlParts) > 1 {
		slug = urlParts[1]
	}

	// replace periods with hyphens
	slug = strings.Replace(slug, ".", "-", -1)

	// replace slashes with hyphens to create a slug
	return strings.Replace(slug, "/", "-", -1), nil
}

// parses the registry URL from a pack URL.
func parseRegistryURL(url string) (string, error) {
	// Throw error if url does not start with github.com - this will
	// undoubtedly change over time, but meets our supported use case for
	// initial launch.
	if !strings.HasPrefix(url, "github.com") {
		return "", fmt.Errorf("url %s must start with %q", url, "github.com")
	}

	// check if a version has been passed and remove it if so
	segments := strings.Split(url, "@")

	// Return the parsed url adding the packs subdirectory in the manner
	// GoGetter expects.
	return segments[0] + "//packs", nil
}

// ParsePackNameAndVersion parses the registry name and version from a
func ParsePackNameAndVersion(name string) (packName, version string, err error) {
	if name == "" {
		err = errors.New("invalid pack name")
		return
	}
	// split the registry name from the version
	segments := strings.Split(name, "@")

	// Set the registry name to the first segment
	packName = segments[0]

	// if somehow they passed more than one @ symbol return error
	if len(segments) > 2 {
		err = fmt.Errorf("invalid registry name %s", name)
		return
	}

	// Initialize the version to nothing
	version = ""
	if len(segments) == 2 {
		version = segments[1]
	}

	return
}
