// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package renderer

import (
	"fmt"
	"os"
	"text/template"

	"github.com/Masterminds/sprig/v3"
	"github.com/davecgh/go-spew/spew"
	"github.com/hashicorp/nomad/api"
)

// funcMap instantiates our default template function map with populated
// functions for use within text.Template.
func funcMap(nomadClient *api.Client) template.FuncMap {

	// Sprig defines our base map.
	f := sprig.TxtFuncMap()

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

	if nomadClient != nil {
		f["nomadNamespaces"] = nomadNamespaces(nomadClient)
		f["nomadNamespace"] = nomadNamespace(nomadClient)
		f["nomadRegions"] = nomadRegions(nomadClient)
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
func toStringList(l []any) (string, error) {
	var out string
	for i := range l {
		if i > 0 && i < len(l) {
			out += ", "
		}
		out += fmt.Sprintf("%q", l[i])
	}
	return "[" + out + "]", nil
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
