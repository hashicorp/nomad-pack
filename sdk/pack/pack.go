// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package pack

import (
	"errors"
	"strings"
)

type ID string

func (p ID) String() string { return string(p) }

// Join returns a new ID with the child path appended to it.
func (p ID) Join(child ID) ID { return ID(string(p) + "." + string(child)) }

// AsPath returns a string with the dot delimiters converted to `/` for use with
// file system paths.
func (p ID) AsPath() string { return strings.ReplaceAll(string(p), ".", "/") }

// File is an individual file component of a Pack.
type File struct {

	// Name represents the name of the file as a reference from the pack
	// directory.
	Name string

	// Path is the absolute path of the file in question.
	Path string

	// Content is the file contents as a byte array.
	Content []byte
}

// Pack is a single nomad-pack package and contains all the required information to
// successfully interrogate and render the pack.
type Pack struct {

	// Metadata is the contents of the Pack metadata.hcl file. It contains
	// high-level information about the pack which is useful for operators and
	// is also exposed as template variables during rendering.
	Metadata *Metadata

	// TemplateFiles are the templated files which constitute this Pack. The
	// list includes both helper templates and Nomad resource templates and all
	// files within the list will be processed by the rendering engine.
	TemplateFiles []*File

	// AuxiliaryFiles are the files included in the "templates" directory of the
	// Pack that will also be rendered, but not run.
	AuxiliaryFiles []*File

	// RootVariableFile is the file which contains the root variables that can
	// include a description, type, and default value. This is parsed along
	// with any override variables and stored within Variables.
	RootVariableFile *File

	// OutputTemplateFile contains the optional output template file. If this
	// string is empty, it is assumed there is no output template to render and
	// print.
	OutputTemplateFile *File

	// dependencies are the packs that this pack depends on. There is no
	// guarantee that this is populated. This is a private field so access can
	// be controlled by the appropriate functions.
	dependencies []*Pack

	// parent tracks the parent pack for dependencies. In the case that this is
	// the root pack, this will be nil.
	parent *Pack

	// alias tracks the name assigned by the parent pack for any dependencies.
	// In the case that this is the parent pack, this will be nil.
	alias string
}

// Name returns the name of the pack. The canonical value for this comes from
// the Pack.Name Metadata struct field.
func (p *Pack) Name() string {
	return p.Metadata.Pack.Name
}

// Alias returns the alias assigned to the pack. The canonical value for this
// comes from the alias on a running pack with a fallback to the Pack.Alias
// Metadata struct field.
func (p *Pack) Alias() string {
	if p.alias != "" {
		return p.alias
	}
	return p.Metadata.Pack.Alias
}

// AliasOrName returns the pack's Alias or the pack's Name, preferring the
// Alias when set.
func (p *Pack) AliasOrName() string {
	if p.Alias() == "" {
		return p.Name()
	}
	return p.Alias()
}

// ID returns the identifier for the pack. The function returns a ID
// which implements the Stringer interface
func (p *Pack) ID() ID {
	return ID(p.AliasOrName())
}

// HasParent reports whether this pack has a parent or can be considered the
// top level pack.
func (p *Pack) HasParent() bool { return p.parent != nil }

// AddDependency to the pack, correctly setting their parent pack identifier and
// alias.
func (p *Pack) AddDependency(alias ID, pack *Pack) {
	pack.parent = p
	pack.alias = alias.String()
	p.dependencies = append(p.dependencies, pack)
}

// AddDependencies to the pack, correctly setting their parent pack identifier.
func (p *Pack) AddDependencies(packs ...*Pack) {
	for i, depPack := range packs {
		packs[i].parent = p
		p.dependencies = append(p.dependencies, depPack)
	}
}

// Dependencies returns the list of dependencies the Pack has.
func (p *Pack) Dependencies() []*Pack { return p.dependencies }

// RootVariableFiles generates a mapping of all root variable files for the
// pack and all dependencies.
func (p *Pack) RootVariableFiles() map[ID]*File {

	// Set up the base output that include the top level packs root variable
	// file entry.
	out := map[ID]*File{p.ID(): p.RootVariableFile}

	// Iterate the dependency packs and add entries into the variable file
	// mapping for each.
	for _, dep := range p.dependencies {
		dep.rootVariableFiles(p.ID(), &out)
	}

	return out
}

func (p *Pack) rootVariableFiles(parentID ID, acc *map[ID]*File) {
	depID := parentID.Join(p.ID())
	(*acc)[depID] = p.RootVariableFile
	for _, dep := range p.dependencies {
		dep.rootVariableFiles(depID, acc)
	}
}

// Validate the pack for terminal problems that can easily be detected at this
// stage. Anything that has potential to cause a panic should ideally be caught
// here.
func (p *Pack) Validate() error {

	if p.RootVariableFile == nil {
		return errors.New("root variable file is required")
	}

	if err := p.Metadata.Validate(); err != nil {
		return err
	}

	return nil
}

func (p *Pack) VariablesPath() ID {
	parts := variablesPathR(p, []string{})
	// Since variablesPathR is depth-first, we need
	// to reverse it before joining it together
	reverse(parts)
	out := ID(strings.Join(parts, "."))
	return out
}

func variablesPathR(p *Pack, in []string) []string {
	if p.parent == nil {
		return append(in, p.AliasOrName())
	}
	return variablesPathR(p.parent, append(in, p.AliasOrName()))
}

func reverse[T any](s []T) {
	for first, last := 0, len(s)-1; first < last; first, last = first+1, last-1 {
		s[first], s[last] = s[last], s[first]
	}
}
