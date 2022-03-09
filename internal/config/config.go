package config

import (
	"context"

	"github.com/hashicorp/nomad-pack/terminal"
)

const (
	FileNameMetadata  = "metadata.hcl"
	FileNameOutputs   = "outputs.tpl"
	FileNameReadme    = "README.md"
	FileNameChangelog = "CHANGELOG.md"
	FileNameVariables = "variables.hcl"

	FolderNameTemplates = "templates"
)

type PackConfig struct {
	UI terminal.UI

	PackName     string
	RegistryName string
	Ref          string

	Plain        bool
	OutPath      string
	AutoApproved bool

	// Used for the "registry create" command
	CreateSamplePack bool

	CacheConfig CacheConfig
	NomadConfig NomadConfig
}

func NewPackConfig() PackConfig {
	return PackConfig{
		NomadConfig: NomadConfig{},
	}
}

func (c *PackConfig) GetUI() terminal.UI {
	// If the UI is set, return it.
	if c.UI != nil {
		return c.UI
	}
	// No UI has been attached to the Pack Config, make one.
	if c.Plain {
		c.UI = terminal.NonInteractiveUI(context.TODO())
	} else {
		c.UI = terminal.GlintUI(context.TODO())
	}
	return c.UI
}
