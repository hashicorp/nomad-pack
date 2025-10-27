// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package pack

import (
	"errors"
)

// Metadata is the contents of the Pack metadata.hcl file. It contains
// high-level information about the pack which is useful for operators and is
// also exposed as template variables during rendering.
type Metadata struct {
	App          *MetadataApp         `hcl:"app,block"`
	Pack         *MetadataPack        `hcl:"pack,block"`
	Integration  *MetadataIntegration `hcl:"integration,block"`
	Dependencies []*Dependency        `hcl:"dependency,block"`
}

// MetadataApp contains information regarding the application that the pack is
// focussed around.
type MetadataApp struct {

	// URL is the HTTP(S) url to the homepage of the application to provide a
	// quick reference to the documentation and help pages.
	URL string `hcl:"url"`

	// Author is an identifier to the author and maintainer of the pack such as
	// HashiCorp or James Rasell
	//
	// Deprecated: Nomad Pack tech preview 4 removes this field, we keep it here for
	// backwards compatibility only.
	Author string `hcl:"author,optional"`

	// TODO: Add Version here, may need to be a block or series of entries to
	// support packs that contain multiple apps.
}

// MetadataPack contains information regarding the pack itself.
type MetadataPack struct {

	// Name of the pack which acts as a convenience for use within template
	// rendering.
	Name string `hcl:"name"`

	// Alias will optionally override the provided Pack name value when set
	Alias string `hcl:"alias,optional"`

	// Description is a small overview of the application that is deployed by
	// the pack.
	Description string `hcl:"description,optional"`

	// URL is the HTTP(S) url of the pack which is acts as a convenience when
	// managing packs within a registry.
	//
	// Deprecated: Nomad Pack tech preview 4 removes this field, we keep it here for
	// backwards compatibility only.
	URL string `hcl:"url,optional"`

	// Version is the version of the pack which is acts as a convenience when
	// managing packs within a registry.
	Version string `hcl:"version"`
}

// MetadataIntegration contains information pertaining to the HashiCorp
// Developer (https://developer.hashicorp.com/) Integrations Library.
// This block is only needed for packs that are to be displayed in the
// integrations library.
//
// Note: Currently, the integrations library is in closed beta, so you
// will not be able to register an integration without support from a
// HashiCorp team member. Furthermore, you may not be able to access
// some of the links specified below in this structs documentation.
type MetadataIntegration struct {

	// Identifier is a unique identifier that points to a specific integration
	// registered in the HashiCorp Developer Integrations Library.
	Identifier string `hcl:"identifier"`

	// Flags is an array of strings referencing various booleans you
	// can enable for your pack as it will display in the integrations
	// library. All flag options are specified within this file:
	// https://github.com/hashicorp/integrations/blob/main/flags.hcl
	Flags []string `hcl:"flags,optional"`

	// You can optionally override the pack.name value here to adjust
	// the name that will be displayed in HashiCorp Developer. For example,
	// your pack name may be "hello_world", whereas on HashiCorp Developer
	// you would like the name to render as "Hello World".
	Name string `hcl:"name,optional"`
}

// ConvertToMapInterface returns a map[string]any representation of the
// metadata object. The conversion doesn't take into account empty values and
// will add them.
func (md *Metadata) ConvertToMapInterface() map[string]any {
	m := map[string]any{
		"app": map[string]any{
			"url": md.App.URL,
		},
		"pack": map[string]any{
			"name":        md.Pack.Name,
			"description": md.Pack.Description,
			"version":     md.Pack.Version,
		},
		"dependencies": []map[string]any{},
	}
	if md.Integration != nil {
		m["integration"] = map[string]any{
			"identifier": md.Integration.Identifier,
			"flags":      md.Integration.Flags,
			"name":       md.Integration.Name,
		}
	}

	dSlice := make([]map[string]any, len(md.Dependencies))
	for i, d := range md.Dependencies {
		dSlice[i] = map[string]any{
			d.AliasOrName(): map[string]any{
				"id":      d.AliasOrName(),
				"name":    d.Name,
				"alias":   d.Alias,
				"source":  d.Source,
				"enabled": d.Enabled,
			},
		}
	}
	m["dependencies"] = dSlice

	return m
}

// Validate the entire Metadata object to ensure it meets requirements and
// doesn't contain invalid or incorrect data.
func (md *Metadata) Validate() error {

	if md == nil {
		return errors.New("pack metadata is required")
	}

	if err := md.App.validate(); err != nil {
		return err
	}

	if err := md.Pack.validate(); err != nil {
		return err
	}

	for _, dep := range md.Dependencies {
		if err := dep.validate(); err != nil {
			return err
		}
	}
	return nil
}

// validate the MetadataApp object to ensure it meets requirements and doesn't
// contain invalid or incorrect data.
func (ma *MetadataApp) validate() error {
	if ma == nil {
		return errors.New("app is uninitialized")
	}
	return nil
}

// validate the MetadataPack object to ensure it meets requirements and doesn't
// contain invalid or incorrect data.
func (mp *MetadataPack) validate() error {
	if mp == nil {
		return errors.New("Pack metadata is uninitialized")
	}
	return nil
}

// AddToInterfaceMap adds the metadata information to the provided map as a new
// entry under the "nomad_pack" key. This is useful for adding this information
// to the template rendering data.  Used in the deprecated V1 Renderer
func (md *Metadata) AddToInterfaceMap(m map[string]any) map[string]any {
	m["nomad_pack"] = md.ConvertToMapInterface()
	return m
}
