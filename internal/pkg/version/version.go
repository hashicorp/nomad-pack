// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package version

import (
	"bytes"
	"fmt"
	"time"
)

var (
	// BuildDate is the time of the git commit used to build the program,
	// in RFC3339 format. It is filled in by the compiler via makefile.
	BuildDate string

	// GitCommit and GitDescribe are filled by the compiler using ldflags to
	// provide useful Git information.
	GitCommit   string
	GitDescribe string

	// Version is the semantic version number describing the current state of
	// nomad-pack.
	Version = "0.0.1"

	// Prerelease designates whether the current version is within a prerelease
	// phase. Typically, this will be "dev" to signify a development cycle or a
	// release candidate phase such as alpha, beta.1, rc.1, or such.
	VersionPrerelease = "techpreview.4"

	// Metadata allows us to provide additional metadata information to the
	// version identifier. This is typically used to identify enterprise builds
	// using the "ent" metadata string.
	VersionMetadata = ""
)

// VersionInfo
type VersionInfo struct {
	BuildDate         time.Time
	Revision          string
	Version           string
	VersionPrerelease string
	VersionMetadata   string
}

func (v *VersionInfo) Copy() *VersionInfo {
	if v == nil {
		return nil
	}

	nv := *v
	return &nv
}

func GetVersion() *VersionInfo {
	ver := Version
	rel := VersionPrerelease
	md := VersionMetadata
	if GitDescribe != "" {
		ver = GitDescribe
	}
	if GitDescribe == "" && rel == "" && VersionPrerelease != "" {
		rel = "dev"
	}

	// on parse error, will be zero value time.Time{}
	built, _ := time.Parse(time.RFC3339, BuildDate)

	return &VersionInfo{
		BuildDate:         built,
		Revision:          GitCommit,
		Version:           ver,
		VersionPrerelease: rel,
		VersionMetadata:   md,
	}
}

func (c *VersionInfo) VersionNumber() string {
	version := c.Version

	if c.VersionPrerelease != "" {
		version = fmt.Sprintf("%s-%s", version, c.VersionPrerelease)
	}

	if c.VersionMetadata != "" {
		version = fmt.Sprintf("%s+%s", version, c.VersionMetadata)
	}

	return version
}

func (c *VersionInfo) FullVersionNumber(rev bool) string {
	var versionString bytes.Buffer

	fmt.Fprintf(&versionString, "Nomad v%s", c.Version)
	if c.VersionPrerelease != "" {
		fmt.Fprintf(&versionString, "-%s", c.VersionPrerelease)
	}

	if c.VersionMetadata != "" {
		fmt.Fprintf(&versionString, "+%s", c.VersionMetadata)
	}

	if !c.BuildDate.IsZero() {
		fmt.Fprintf(&versionString, "\nBuildDate %s", c.BuildDate.Format(time.RFC3339))
	}

	if rev && c.Revision != "" {
		fmt.Fprintf(&versionString, "\nRevision %s", c.Revision)
	}

	return versionString.String()
}
