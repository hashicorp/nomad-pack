package pack

import "errors"

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
	// the parent pack, this will be nil.
	parent *Pack
}

// Name returns the name of the pack. The canonical value for this comes from
// the Pack.Name Metadata struct field.
func (p *Pack) Name() string { return p.Metadata.Pack.Name }

// HasParent reports whether this pack has a parent or can be considered the
// top level pack.
func (p *Pack) HasParent() bool { return p.parent != nil }

// AddDependencies to the pack, correctly setting their parent pack identifier.
func (p *Pack) AddDependencies(packs ...*Pack) {
	for i, depPack := range packs {
		packs[i].parent = p
		p.dependencies = append(p.dependencies, depPack)
	}
}

// Dependencies returns the list of dependence the Pack has.
func (p *Pack) Dependencies() []*Pack { return p.dependencies }

// RootVariableFiles generates a mapping of all root variable files for the
// pack and all dependencies.
func (p *Pack) RootVariableFiles() map[string]*File {

	// Set up the base output that include the top level packs root variable
	// file entry.
	out := map[string]*File{p.Name(): p.RootVariableFile}

	// Iterate the dependency packs and add entries into the variable file
	// mapping for each.
	for _, dep := range p.dependencies {
		out[dep.Name()] = dep.RootVariableFile
	}

	return out
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