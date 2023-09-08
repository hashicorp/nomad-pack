// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package renderer

import (
	"fmt"
	"path"
	"strings"
	"text/template"

	"github.com/hashicorp/hcl/v2/hclwrite"
	"github.com/hashicorp/nomad/api"

	"github.com/hashicorp/nomad-pack/sdk/pack"
)

// Renderer provides template rendering functionality using the text/template
// package.
type Renderer struct {

	// Strict determines the template rendering missingkey option setting. If
	// set to true error will be used, otherwise zero is used.
	Strict bool

	// Client is the Nomad API client used when running the Nomad template
	// functions. It can potentially be nil, therefore care should be taken
	// when accessing it.
	Client *api.Client

	// RenderAuxFiles determines whether we should render auxiliary files found
	// in template/ or not
	RenderAuxFiles bool

	// Format determines whether we should format templates before rendering them
	// or not
	Format bool

	// stores the pack information, variables and tpl, so we can perform the
	// output template rendering after pack deployment.
	pack      *pack.Pack
	variables map[string]any
	tpl       *template.Template
}

// toRender details an individual template to render along with it's scoped
// variables.
type toRender struct {
	content   string
	variables map[string]any
}

const (
	leftTemplateDelim  = "[["
	rightTemplateDelim = "]]"
)

// Render is responsible for iterating the pack and rendering each defined
// template using the parsed variable map.
func (r *Renderer) Render(p *pack.Pack, variables map[string]any) (*Rendered, error) {

	// filesToRender stores all the templates and auxiliary files that should be
	// rendered
	filesToRender := map[string]toRender{}
	prepareFiles(p, filesToRender, variables, r.RenderAuxFiles)

	// Set up our new template, add the function mapping, and set the
	// delimiters.
	tpl := template.New("tpl").Funcs(funcMap(r.Client)).Delims(leftTemplateDelim, rightTemplateDelim)

	// Control the behaviour of rendering when it encounters an element
	// referenced which doesn't exist within the variable mapping.
	if r.Strict {
		tpl.Option("missingkey=error")
	} else {
		tpl.Option("missingkey=zero")
	}

	for name, src := range filesToRender {
		if tpl.Lookup(name) == nil {
			if _, err := tpl.New(name).Parse(src.content); err != nil {
				return nil, err
			}
		}
	}

	// Generate our output structure.
	rendered := &Rendered{
		parentRenders:    make(map[string]string),
		dependentRenders: make(map[string]string),
	}

	for name, src := range filesToRender {

		// Skip the helper templates as we don't need to render these. They are
		// called and used from within full templates.
		if strings.Contains(name, "templates/_") {
			continue
		}

		// Execute the template render and add this to the output unless there
		// is an error.
		var buf strings.Builder

		if err := tpl.ExecuteTemplate(&buf, name, src.variables); err != nil {
			return nil, fmt.Errorf("failed to render %s: %v", name, err)
		}

		// Even when using "missingkey=zero", missing values will be rendered
		// when "<no value>" rather than an empty string. This modifies that
		// behaviour.
		replacedTpl := strings.ReplaceAll(buf.String(), "<no value>", "")

		// Split the name so the element at index zero becomes the pack name.
		nameSplit := strings.Split(name, "/")

		// If we encounter a template that's empty (just renders to whitespace),
		// we skip it.
		if len(strings.TrimSpace(replacedTpl)) == 0 {
			continue
		}

		if r.Format {
			// hclfmt the templates
			f := hclwrite.Format([]byte(replacedTpl))
			replacedTpl = string(f)
		}

		// Add the rendered pack template to our output, depending on whether
		// it's name matches that of our parent.
		if nameSplit[0] == p.Name() {
			rendered.parentRenders[name] = replacedTpl
		} else {
			rendered.dependentRenders[name] = replacedTpl
		}
	}

	r.variables = variables
	r.pack = p
	r.tpl = tpl

	return rendered, nil
}

// RenderOutput performs the output template rendering.
func (r *Renderer) RenderOutput() (string, error) {

	// If we don't have a template file, return early.
	if r.pack.OutputTemplateFile == nil {
		return "", nil
	}

	if _, err := r.tpl.New(r.pack.OutputTemplateFile.Name).Parse(string(r.pack.OutputTemplateFile.Content)); err != nil {
		return "", err
	}

	var buf strings.Builder
	if err := r.tpl.ExecuteTemplate(&buf, r.pack.OutputTemplateFile.Name, r.variables); err != nil {
		return "", fmt.Errorf("failed to render %s: %v", r.pack.OutputTemplateFile.Name, err)
	}

	return buf.String(), nil
}

// prepareFiles recurses the pack and its dependencies and returns a map
// with the templates/auxiliary files to render along with the variables which
// correspond.
func prepareFiles(p *pack.Pack,
	files map[string]toRender,
	variables map[string]any,
	renderAuxFiles bool,
) {

	newVars := make(map[string]any)

	// If the pack is a dependency, it only has access to its namespaced
	// variables. If the pack is the parent/root pack, then it has access to
	// all.
	if p.HasParent() {
		if v, ok := variables[p.Name()]; ok {
			newVars["my"] = v
			newVars[p.Name()] = v
		}
	} else {
		newVars = variables
	}

	// Add the pack's metadata to the variable mapping.
	newVars = p.Metadata.AddToInterfaceMap(newVars)

	// Make the `my` alias for the parent pack.
	if !p.HasParent() {
		newVars["my"] = newVars[p.Name()]
	}
	// Iterate the dependencies and prepareTemplates for each.
	for _, child := range p.Dependencies() {
		prepareFiles(child, files, newVars, renderAuxFiles)
	}

	// Add each template within the pack with scoped variables.
	for _, t := range p.TemplateFiles {
		files[path.Join(p.Name(), t.Name)] = toRender{content: string(t.Content), variables: newVars}
	}

	if renderAuxFiles {
		// Add each aux file within the pack with scoped variables.
		for _, f := range p.AuxiliaryFiles {
			files[path.Join(p.Name(), f.Name)] = toRender{content: string(f.Content), variables: newVars}
		}
	}
}

// Rendered encapsulates all the rendered template files associated with the
// pack. It splits them based on whether they belong to the parent or a
// dependency.
type Rendered struct {
	parentRenders    map[string]string
	dependentRenders map[string]string
}

// ParentRenders returns a map of rendered templates belonging to the parent
// pack. The map key represents the path and file name of the template.
func (r *Rendered) ParentRenders() map[string]string { return r.parentRenders }

// LenParentRenders returns the number of parent rendered templates that are
// stored.
func (r *Rendered) LenParentRenders() int { return len(r.parentRenders) }

// DependentRenders returns a map of rendered templates belonging to the
// dependent packs of the parent template. The map key represents the path and
// file name of the template.
func (r *Rendered) DependentRenders() map[string]string { return r.dependentRenders }

// LenDependentRenders returns the number of dependent rendered templates that
// are stored.
func (r *Rendered) LenDependentRenders() int { return len(r.dependentRenders) }
