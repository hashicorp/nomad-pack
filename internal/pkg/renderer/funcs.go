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
	}

	if r != nil && r.PackPath != "" {
		f["packPath"] = func() (string, error) {
			return r.PackPath, nil
		}
	}

	// Add additional custom functions.
	f["fileContents"] = fileContents
	f["toStringList"] = toStringList

	return f
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
