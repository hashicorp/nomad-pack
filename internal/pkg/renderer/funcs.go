package renderer

import (
	"context"
	"fmt"
	"io/ioutil"
	"strconv"
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
func withIndent(in string, v interface{}) (interface{}, error) {
	act := "withIndent"
	spew, err := parseSpewParam(act, v)
	if err != nil {
		return nil, err
	}
	spew.Indent = in
	return spew, nil
}

func withMaxDepth(in, inS interface{}) (interface{}, error) {
	act := "withMaxDepth"
	i, err := parseIntParam(act, in)
	if err != nil {
		return nil, err
	}

	s, err := parseSpewParam(act, inS)
	if err != nil {
		return nil, err
	}
	s.MaxDepth = i
	return s, nil
}

func withDisableMethods(in, inS interface{}) (interface{}, error) {
	act := "withDisableMethods"
	b, err := parseBoolParam(act, in)
	if err != nil {
		return nil, err
	}

	s, err := parseSpewParam(act, inS)
	if err != nil {
		return nil, err
	}
	s.DisableMethods = b
	return s, nil
}

func withDisablePointerMethods(in, inS interface{}) (interface{}, error) {
	act := "withDisablePointerMethods"
	b, err := parseBoolParam(act, in)
	if err != nil {
		return nil, err
	}

	s, err := parseSpewParam(act, inS)
	if err != nil {
		return nil, err
	}
	s.DisablePointerMethods = b
	return s, nil
}

func withDisablePointerAddresses(in, inS interface{}) (interface{}, error) {
	act := "withDisablePointerAddresses"
	b, err := parseBoolParam(act, in)
	if err != nil {
		return nil, err
	}

	s, err := parseSpewParam(act, inS)
	if err != nil {
		return nil, err
	}
	s.DisablePointerAddresses = b
	return s, nil
}

func withDisableCapacities(in, inS interface{}) (interface{}, error) {
	act := "withDisableCapacities"
	b, err := parseBoolParam(act, in)
	if err != nil {
		return nil, err
	}

	s, err := parseSpewParam(act, inS)
	if err != nil {
		return nil, err
	}
	s.DisableCapacities = b
	return s, nil
}

func withContinueOnMethod(in, inS interface{}) (interface{}, error) {
	act := "withContinueOnMethod"
	b, err := parseBoolParam(act, in)
	if err != nil {
		return nil, err
	}

	s, err := parseSpewParam(act, inS)
	if err != nil {
		return nil, err
	}
	s.ContinueOnMethod = b
	return s, nil
}

func withSortKeys(in, inS interface{}) (interface{}, error) {
	act := "withSortKeys"
	b, err := parseBoolParam(act, in)
	if err != nil {
		return nil, err
	}

	s, err := parseSpewParam(act, inS)
	if err != nil {
		return nil, err
	}
	s.SortKeys = b
	return s, nil
}

func withSpewKeys(in, inS interface{}) (interface{}, error) {
	act := "withSpewKeys"
	b, err := parseBoolParam(act, in)
	if err != nil {
		return nil, err
	}

	s, err := parseSpewParam(act, inS)
	if err != nil {
		return nil, err
	}
	s.SpewKeys = b
	return s, nil
}

type ErrSpewConfig struct {
	act    string
	got    string
	expect string
}

func newErrSpewConfig(act, expect string, got interface{}) ErrSpewConfig {
	return ErrSpewConfig{
		act:    act,
		expect: expect,
		got:    fmt.Sprintf("%T", got),
	}
}

func (e ErrSpewConfig) Error() string {
	return fmt.Sprintf("invalid parameter: expected %s, received %s", e.expect, e.got)
}

func parseBoolParam(act string, in interface{}) (bool, error) {
	var b bool
	switch in := in.(type) {
	case bool:
		b = in
	case string:
		var err error
		b, err = strconv.ParseBool(in)
		if err != nil {
			return false, newErrSpewConfig(act, "bool or bool-like string", in)
		}
	default:
		return false, newErrSpewConfig(act, "bool or bool-like string", in)
	}
	return b, nil
}

func parseIntParam(act string, in interface{}) (int, error) {
	var i int
	switch in := in.(type) {
	case int:
		i = in
	case string:
		var err error
		pi, err := strconv.ParseInt(in, 0, 32)
		i = int(pi)
		if err != nil {
			return -1, newErrSpewConfig(act, "int or int-like string", in)
		}
	default:
		return -1, newErrSpewConfig(act, "int or int-like string", in)
	}
	return i, nil
}

func parseSpewParam(act string, in interface{}) (*spew.ConfigState, error) {
	if spew, ok := in.(*spew.ConfigState); ok {
		return spew, nil
	} else {
		return nil, newErrSpewConfig(act, "*spew.ConfigState", spew)
	}
}
