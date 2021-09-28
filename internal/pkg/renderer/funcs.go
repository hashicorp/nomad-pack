package renderer

import (
	"context"
	"fmt"
	"io/ioutil"
	"text/template"

	"github.com/Masterminds/sprig"
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
	f["spewDump"] = spewDump
	f["spewPrintf"] = spewPrintf

	if nomadClient != nil {
		f["nomadNamespaces"] = nomadNamespaces(nomadClient)
		f["nomadNamespace"] = nomadNamespace(nomadClient)
		f["nomadRegions"] = nomadRegions(nomadClient)
	}

	// Add additional custom functions.
	f["fileContents"] = fileContents

	return f
}

//
func fileContents(file string) (string, error) {
	content, err := ioutil.ReadFile(file)
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

// spewDump dumps the entire contents of the interface as a string. The output
// includes the content types and values and is extremely useful for debugging.
func spewDump(a interface{}) string { return spew.Sdump(a) }

// spewPrintf dumps the supplied arguments into a string according to the
// supplied format. This is useful when debugging.
func spewPrintf(format string, args ...interface{}) string { return spew.Sprintf(format, args) }
