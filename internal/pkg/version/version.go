package version

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"strings"
)

var (
	// GitCommit and GitDescribe are filled by the compiler using ldflags to
	// provide useful Git information.
	GitCommit   string
	GitDescribe string

	// Version is the semantic version number describing the current state of
	// NOM.
	Version = "0.0.1"

	// Prerelease designates whether the current version is within a prerelease
	// phase. Typically, this will be "dev" to signify a development cycle or a
	// release candidate phase such as alpha, beta, rc1, or such.
	Prerelease = "alpha"

	// Metadata allows us to provide additional metadata information to the
	// version identifier. This is typically used to identify enterprise builds
	// using the "ent" metadata string.
	Metadata = ""
)

// HumanVersion composes the parts of the version in a way that's suitable for
// displaying to humans.
func HumanVersion() string {
	version := Version
	release := Prerelease

	if GitDescribe != "" {
		version = GitDescribe
	} else {
		if release == "" {
			release = "dev"
		}

		if release != "" && !strings.HasSuffix(version, "-"+release) {
			// if we tagged a prerelease version then the release is in the version
			// already.
			version += fmt.Sprintf("-%s", release)
		}

		if Metadata != "" {
			version += fmt.Sprintf("+%s", Metadata)
		}
	}

	// Add the commit hash at the very end of the version.
	if GitCommit != "" {
		version += fmt.Sprintf(" (%s)", GitCommit)
	}

	// Add v as prefix if not present
	if !strings.HasPrefix(version, "v") {
		version = fmt.Sprintf("v%s", version)
	}

	// Strip off any single quotes added by the git information.
	return strings.Replace(version, "'", "", -1)
}

// GitSHA gets the git sha of a pack by directory. Requires git
// to be installed, pathPath to exist, and for packPath to be part of a git
// repository.
func GitSHA(packPath string) (string, error) {
	if _, err := os.Stat(packPath); os.IsNotExist(err) {
		return "", err
	}

	// TODO: OS Platform and security review
	// Find the path of the git binary
	gitPath, err := exec.LookPath("git")
	if err != nil {
		return "", err
	}

	// configure command to find the git sha of package directory
	c := exec.Command(gitPath, "rev-list", "-1", "HEAD")
	c.Dir = packPath
	var out bytes.Buffer
	c.Stdout = &out

	err = c.Run()
	if err != nil {
		return "", err
	}

	return out.String()[:7], nil
}
