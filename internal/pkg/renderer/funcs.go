package renderer

import (
	"context"
	"fmt"
	"os"
	"text/template"

	"github.com/Masterminds/sprig/v3"
	"github.com/davecgh/go-spew/spew"
	v1client "github.com/hashicorp/nomad-openapi/clients/go/v1"
	v1 "github.com/hashicorp/nomad-openapi/v1"
)

// funcMap instantiates our default template function map with populated
// functions for use within text.Template.
func funcMap(nomadClient *v1.Client) template.FuncMap {

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
func nomadNamespaces(client *v1.Client) func() (*[]v1client.Namespace, error) {
	return func() (*[]v1client.Namespace, error) {
		out, _, err := client.Namespaces().GetNamespaces(context.Background())
		return out, err
	}
}

// nomadNamespace performs a query against the passed namespace.
func nomadNamespace(client *v1.Client) func(string) (*v1client.Namespace, error) {
	return func(ns string) (*v1client.Namespace, error) {
		out, _, err := client.Namespaces().GetNamespace(context.Background(), ns)
		return out, err
	}
}

// nomadRegions performs a listing of the Nomad regions from the Nomad API. It
// returns these within a list along with any error whilst performing the API
// call.
func nomadRegions(client *v1.Client) func() (*[]string, error) {
	return func() (*[]string, error) { return client.Regions().GetRegions(context.Background()) }
}

// toStringList takes a list of string and returns the HCL equivalent which is
// useful when templating jobs and params such as datacenters.
func toStringList(l []interface{}) (string, error) {
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
func withIndent(in string, s *spew.ConfigState) interface{} {
	s.Indent = in
	return s
}

func withMaxDepth(in int, s *spew.ConfigState) interface{} {
	s.MaxDepth = in
	return s
}

func withDisableMethods(s *spew.ConfigState) interface{} {
	s.DisableMethods = true
	return s
}

func withDisablePointerMethods(s *spew.ConfigState) interface{} {
	s.DisablePointerMethods = true
	return s
}

func withDisablePointerAddresses(s *spew.ConfigState) interface{} {
	s.DisablePointerAddresses = true
	return s
}

func withDisableCapacities(s *spew.ConfigState) interface{} {
	s.DisableCapacities = true
	return s
}

func withContinueOnMethod(s *spew.ConfigState) (interface{}, error) {
	s.ContinueOnMethod = true
	return s, nil
}

func withSortKeys(s *spew.ConfigState) interface{} {
	s.SortKeys = true
	return s
}

func withSpewKeys(s *spew.ConfigState) interface{} {
	s.SpewKeys = true
	return s
}
