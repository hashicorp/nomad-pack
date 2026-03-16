// Copyright IBM Corp. 2021, 2025
// SPDX-License-Identifier: MPL-2.0

package renderer

import (
	"fmt"
	"os"
	"strings"
	"text/template"

	"github.com/Masterminds/sprig/v3"
	"github.com/davecgh/go-spew/spew"
	"github.com/hashicorp/nomad-pack/internal/pkg/variable/parser"
	"github.com/hashicorp/nomad/api"
	"golang.org/x/exp/maps"
)

// funcMap instantiates our default template function map with populated
// functions for use within text.Template.
func funcMap(r *Renderer) template.FuncMap {

	// The base of the funcmap comes from the template context funcs
	f := make(template.FuncMap)
	if r != nil && r.pv != nil {
		maps.Copy(f, parser.PackTemplateContextFuncs(r.pv.IsV1()))
	}

	// Copy the sprig funcs into the funcmap.
	maps.Copy(f, sprig.TxtFuncMap())

	// Add debugging functions. These are useful when debugging templates and
	// variables.
	f["spewDump"] = spew.Sdump
	f["spewPrintf"] = spew.Sprintf
	f["customSpew"] = spew.NewDefaultConfig
	f["withIndent"] = withIndent
	f["withMaxDepth"] = withMaxDepth
	f["withDisableMethods"] = withDisableMethods
	f["withDisablePointerMethods"] = withDisablePointerMethods
	f["withDisablePointerAddresses"] = withDisablePointerAddresses
	f["withDisableCapacities"] = withDisableCapacities
	f["withContinueOnMethod"] = withContinueOnMethod
	f["withDisableMethods"] = withDisableMethods
	f["withSortKeys"] = withSortKeys
	f["withSpewKeys"] = withSpewKeys

	if r != nil && r.Client != nil {
		f["nomadNamespaces"] = nomadNamespaces(r.Client)
		f["nomadNamespace"] = nomadNamespace(r.Client)
		f["nomadRegions"] = nomadRegions(r.Client)
		f["nomadVariables"] = nomadVariables(r.Client)
		f["nomadVariable"] = nomadVariable(r.Client)
	}

	if r != nil && r.PackPath != "" {
		f["packPath"] = func() (string, error) {
			return r.PackPath, nil
		}
	}

	// Add additional custom functions.
	f["fileContents"] = fileContents
	f["toStringList"] = toStringList
	f["tpl"] = tplFunc(r)

	return f
}

// tplFunc returns a function which can be used as a template function to render
// a template string within a template. It uses the Renderer to access the parent
// template and render with the same FuncMap and variables as the parent template.
// This is useful for rendering nested templates, such as when using the tpl
// function within a pack template.
func tplFunc(r *Renderer) func(string, interface{}) (string, error) {
	return func(tpl string, vals interface{}) (string, error) {
		// Clone the parent template so that we can add the tpl string as a new
		// template to this clone without affecting the parent template.
		t, err := r.tpl.Clone()
		if err != nil {
			return "", fmt.Errorf("cannot clone template: %w", err)
		}

		// Control the behaviour of rendering when it encounters an element
		// referenced which doesn't exist within the variable mapping.
		if r.Strict {
			t.Option("missingkey=error")
		} else {
			t.Option("missingkey=zero")
		}

		// New() is required: Parse() won't replace a template's body if the
		// content is empty/whitespace (e.g., empty string, or pure define blocks).
		// Without New(), Execute() would run the clone's original body (parent's
		// content) causing infinite recursion. New() ensures we execute only tpl.
		// See: https://pkg.go.dev/text/template#Template.Parse
		t, err = t.New(r.tpl.Name()).Parse(tpl)
		if err != nil {
			return "", fmt.Errorf("cannot parse template %w", err)
		}

		var buf strings.Builder
		if err := t.Execute(&buf, vals); err != nil {
			return "", fmt.Errorf("error during tpl function execution: %w", err)
		}

		// See comment in renderer explaining the <no value> hack.
		return strings.ReplaceAll(buf.String(), "<no value>", ""), nil
	}
}

// fileContents reads the passed path and returns the content as a string.
func fileContents(file string) (string, error) {
	content, err := os.ReadFile(file)
	if err != nil {
		return "", fmt.Errorf("failed to read %s: %v", file, err)
	}
	return string(content), nil
}

// nomadNamespaces performs a Nomad API query against the namespace endpoint to
// list the namespaces.
func nomadNamespaces(client *api.Client) func() ([]*api.Namespace, error) {
	return func() ([]*api.Namespace, error) {
		out, _, err := client.Namespaces().List(&api.QueryOptions{})
		return out, err
	}
}

// nomadNamespace performs a query against the passed namespace.
func nomadNamespace(client *api.Client) func(string) (*api.Namespace, error) {
	return func(ns string) (*api.Namespace, error) {
		out, _, err := client.Namespaces().Info(ns, &api.QueryOptions{})
		return out, err
	}
}

// nomadRegions performs a listing of the Nomad regions from the Nomad API. It
// returns these within a list along with any error whilst performing the API
// call.
func nomadRegions(client *api.Client) func() ([]string, error) {
	return func() ([]string, error) { return client.Regions().List() }
}

// nomadVariables lists all variables in the specified namespace, optionally filtered by prefix
func nomadVariables(client *api.Client) func(string, ...string) ([]*api.VariableMetadata, error) {
	return func(namespace string, prefix ...string) ([]*api.VariableMetadata, error) {
		opts := &api.QueryOptions{Namespace: namespace}

		// If prefix is provided and not empty, add it to query options
		if len(prefix) > 0 {
			opts.Prefix = prefix[0]
		}

		out, _, err := client.Variables().List(opts)
		return out, err
	}
}

// nomadVariable retrieves a specific variable by path and namespace
func nomadVariable(client *api.Client) func(string, string) (*api.Variable, error) {
	return func(path string, namespace string) (*api.Variable, error) {
		out, _, err := client.Variables().Read(path, &api.QueryOptions{Namespace: namespace})
		return out, err
	}
}

// toStringList takes a list of string and returns the HCL equivalent which is
// useful when templating jobs and params such as datacenters.
func toStringList(l any) (string, error) {
	var out strings.Builder
	out.WriteRune('[')
	switch tl := l.(type) {
	case []any:
		// If l is a []string, then the caller probably wants that printed
		// as a list of quoted elements, JSON style.
		for i, v := range tl {
			if i > 0 {
				out.WriteString(", ")
			}
			out.WriteString(fmt.Sprintf("%q", v))
		}
	default:
		out.WriteString(fmt.Sprintf("%q", l))
	}
	out.WriteRune(']')
	o := out.String()
	return o, nil
}

// Spew helper funcs
func withIndent(in string, s *spew.ConfigState) any {
	s.Indent = in
	return s
}

func withMaxDepth(in int, s *spew.ConfigState) any {
	s.MaxDepth = in
	return s
}

func withDisableMethods(s *spew.ConfigState) any {
	s.DisableMethods = true
	return s
}

func withDisablePointerMethods(s *spew.ConfigState) any {
	s.DisablePointerMethods = true
	return s
}

func withDisablePointerAddresses(s *spew.ConfigState) any {
	s.DisablePointerAddresses = true
	return s
}

func withDisableCapacities(s *spew.ConfigState) any {
	s.DisableCapacities = true
	return s
}

func withContinueOnMethod(s *spew.ConfigState) (any, error) {
	s.ContinueOnMethod = true
	return s, nil
}

func withSortKeys(s *spew.ConfigState) any {
	s.SortKeys = true
	return s
}

func withSpewKeys(s *spew.ConfigState) any {
	s.SpewKeys = true
	return s
}
