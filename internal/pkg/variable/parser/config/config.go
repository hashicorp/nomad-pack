// Copyright IBM Corp. 2021, 2025
// SPDX-License-Identifier: MPL-2.0

package config

import (
	"github.com/hashicorp/nomad-pack/sdk/pack"
)

type ParserVersion int

const (
	VUnknown ParserVersion = iota
	V1
	V2
)

// ParserConfig contains details of the numerous sources of variables which
// should be parsed and merged according to the expected strategy.
type ParserConfig struct {

	// ParserVersion determines which variable parser is loaded to create the
	// template context and parse the overrides files.
	Version ParserVersion

	// ParentName is the name of the parent pack. Used for deprecated ParserV1.
	ParentName string

	// ParentPack is a pointer to the parent pack. Used for ParserV2
	ParentPack *pack.Pack

	// RootVariableFiles contains a map of root variable files, keyed by their
	// absolute pack name. "«root pack name».«child pack».«grandchild pack»"
	RootVariableFiles map[pack.ID]*pack.File

	// EnvOverrides are key=value variables and take the lowest precedence of
	// all sources. If the same key is supplied twice, the last wins.
	EnvOverrides map[string]string

	// FileOverrides is a list of files which contain variable overrides in the
	// form key=value. The files will be stored before processing to ensure a
	// consistent processing experience. Overrides here will replace any
	// default root declarations.
	FileOverrides []string

	// FlagOverrides are key=value variables and take the highest precedence of
	// all sources. If the same key is supplied twice, the last wins.
	FlagOverrides map[string]string

	// IgnoreMissingVars determines whether we error or not on variable overrides
	// that don't have corresponding vars in the pack.
	IgnoreMissingVars bool
}
