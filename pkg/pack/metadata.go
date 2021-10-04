package pack

import (
	"errors"
)

// Metadata is the contents of the Pack metadata.hcl file. It contains
// high-level information about the pack which is useful for operators and is
// also exposed as template variables during rendering.
type Metadata struct {
	App          *MetadataApp  `hcl:"app,block"`
	Pack         *MetadataPack `hcl:"pack,block"`
	Dependencies []*Dependency `hcl:"dependency,block"`
}

// MetadataApp contains information regarding the application that the pack is
// focussed around.
type MetadataApp struct {

	// URL is the HTTP(S) url to the homepage of the application to provide a
	// quick reference to the documentation and help pages.
	URL string `hcl:"url"`

	// Author is an identifier to the author and maintainer of the pack such as
	// HashiCorp or James Rasell
	Author string `hcl:"author"`
}

// MetadataPack contains information regarding the pack itself.
type MetadataPack struct {

	// Name of the pack which acts as a convenience for use within template
	// rendering.
	Name string `hcl:"name"`

	// Description is a small overview of the application that is deployed by
	// the pack.
	Description string `hcl:"description,optional"`
}

// ConvertToMapInterface returns a map[string]interface{} representation of the
// metadata object. The conversion doesn't take into account empty values and
// will add them.
func (md *Metadata) ConvertToMapInterface() map[string]interface{} {
	return map[string]interface{}{
		"nomad_pack": map[string]interface{}{
			"app": map[string]interface{}{
				"url":    md.App.URL,
				"author": md.App.Author,
			},
			"pack": map[string]interface{}{
				"name":        md.Pack.Name,
				"description": md.Pack.Description,
			},
		},
	}
}

// AddToInterfaceMap adds the metadata information to the provided map as a new
// entry under the "nom" key. This is useful for adding this information to the
// template rendering data.
func (md *Metadata) AddToInterfaceMap(m map[string]interface{}) map[string]interface{} {
	m["nomad_pack"] = map[string]interface{}{
		"app": map[string]interface{}{
			"url":    md.App.URL,
			"author": md.App.Author,
		},
		"pack": map[string]interface{}{
			"name":        md.Pack.Name,
			"description": md.Pack.Description,
		},
	}
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
	return nil
}

// validate the MetadataPack object to ensure it meets requirements and doesn't
// contain invalid or incorrect data.
func (mp *MetadataPack) validate() error {
	return nil
}
