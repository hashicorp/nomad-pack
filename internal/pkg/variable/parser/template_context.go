package parser

import (
	"errors"
	"fmt"
	"strings"
	"text/template"

	"github.com/hashicorp/nomad-pack/sdk/pack"
	"golang.org/x/exp/slices"
)

// PackTemplateContext v2
//
// A v2 PackTemplateContext is a tree of PackData elements organized by name and
// dependency depth. Consider the following Pack structure:
//
// A Pack named "a", which will be the pack that is being targeted by the user
// via the nomad-pack command. The "a" pack's metadata points to two dependency
// packs: "d1" aliased to "c1" and "d2" aliased to "c2". Lastly, the "d2" pack's
// metadata points to a singular dependency named "dep", which is left unaliased.
//
// The generated template context would match the following diagram.
//
// ┌────────────────────────────────────────────────────────┐
// │ PackTemplateContext                                    │
// │ map[string]PackContextable                             │
// ├───────┬────────────────────────────────────────────────┤
// │  KEY  │ VALUE                                          │
// ├───────┼────────────────────────────────────────────────┤
// │       │ ┌────────────────────┐                         │
// │ _self │ │PackData            │                         │
// │       │ ├────────────────────┤                         │
// │       │ │Pack *pack.Pack     │                         │
// │       │ │meta map[string]any │                         │
// │       │ │vars map[string]any │                         │
// │       │ └────────────────────┘                         │
// ├───────┼────────────────────────────────────────────────┤
// │       │ ┌────────────────────────────────┐             │
// │   c1  │ │ PackTemplateContext            │             │
// │       │ │ map[string]PackContextable     │             │
// │       │ ├───────┬────────────────────────┤             │
// │       │ │  KEY  │ VALUE                  │             │
// │       │ ├───────┼────────────────────────┤             │
// │       │ │       │ ┌────────────────────┐ │             │
// │       │ │ _self │ │PackData            │ │             │
// │       │ │       │ ├────────────────────┤ │             │
// │       │ │       │ │Pack *pack.Pack     │ │             │
// │       │ │       │ │meta map[string]any │ │             │
// │       │ │       │ │vars map[string]any │ │             │
// │       │ │       │ └────────────────────┘ │             │
// │       │ └───────┴────────────────────────┘             │
// ├───────┼────────────────────────────────────────────────┤
// │       │ ┌────────────────────────────────────────────┐ │
// │  c2   │ │ PackTemplateContext                        │ │
// │       │ │ map[string]PackContextable                 │ │
// │       │ ├───────┬────────────────────────────────────┤ │
// │       │ │  KEY  │ VALUE                              │ │
// │       │ ├───────┼────────────────────────────────────┤ │
// │       │ │       │ ┌────────────────────┐             │ │
// │       │ │ _self │ │PackData            │             │ │
// │       │ │       │ ├────────────────────┤             │ │
// │       │ │       │ │Pack *pack.Pack     │             │ │
// │       │ │       │ │meta map[string]any │             │ │
// │       │ │       │ │vars map[string]any │             │ │
// │       │ │       │ └────────────────────┘             │ │
// │       │ ├───────┼────────────────────────────────────┤ │
// │       │ │       │ ┌────────────────────────────────┐ │ │
// │       │ │  dep  │ │ PackTemplateContext            │ │ │
// │       │ │       │ │ map[string]PackContextable     │ │ │
// │       │ │       │ ├───────┬────────────────────────┤ │ │
// │       │ │       │ │  KEY  │ VALUE                  │ │ │
// │       │ │       │ ├───────┼────────────────────────┤ │ │
// │       │ │       │ │       │ ┌────────────────────┐ │ │ │
// │       │ │       │ │ _self │ │PackData            │ │ │ │
// │       │ │       │ │       │ ├────────────────────┤ │ │ │
// │       │ │       │ │       │ │Pack *pack.Pack     │ │ │ │
// │       │ │       │ │       │ │meta map[string]any │ │ │ │
// │       │ │       │ │       │ │vars map[string]any │ │ │ │
// │       │ │       │ │       │ └────────────────────┘ │ │ │
// │       │ │       │ └───────┴────────────────────────┘ │ │
// │       │ └───────┴────────────────────────────────────┘ │
// └───────┴────────────────────────────────────────────────┘
//
// This organization scheme allows users to select a pack using a dotted descent
// into the context map and to access the pack's data using functions that then
// read the PackData at `_self`

type PackTemplateContext map[string]PackContextable

type PackContextable interface {
	getPack() PackData
	getVars() map[string]any
	getMetas() map[string]any
}

const CurrentPackKey = "_self"

// getVars retrieves the `vars` map of the PackData at the `CurrentPackKey` key
func (p PackTemplateContext) getVars() map[string]any { return p.getPack().vars }

// getMetas retrieves the `meta` map of the PackData at the `CurrentPackKey` key
func (p PackTemplateContext) getMetas() map[string]any { return p.getPack().meta }

// getPack retrieves the PackData at the `CurrentPackKey` key
func (p PackTemplateContext) getPack() PackData { return p[CurrentPackKey].(PackData) }

// PackData is the currently selected Pack's metadata and variables, normally
// stored at `CurrentPackKey` in a PackTemplateContext.
type PackData struct {
	Pack *pack.Pack
	meta map[string]any
	vars map[string]any
}

// getVars returns the vars value from a PackData
func (p PackData) getVars() map[string]any { return p.vars }

// getMetas returns the meta value from a PackData
func (p PackData) getMetas() map[string]any { return p.meta }

// getPack returns this PackData
func (p PackData) getPack() PackData { return p }

//
// Template function implementations
//

// PackTemplateContextFuncs returns a text/template FuncMap that are necessary
// to access the variables and metadata information stored in the template
// context
func PackTemplateContextFuncs(isV1 bool) template.FuncMap {
	if isV1 {
		return PackTemplateContextFuncsV1()
	}
	return PackTemplateContextFuncsV2()
}

// PackTemplateContextFuncsV1 returns the a funcMap with error-only functions
// with the same names as v2 ones, so users are presented with more informative
// errors than the generic go-template ones.
func PackTemplateContextFuncsV1() template.FuncMap {
	fm := PackTemplateContextFuncsV2()
	for k := range fm {
		k := k
		fm[k] = func(_ ...any) (string, error) {
			return "", fmt.Errorf("%s is not implemented for nomad-pack's v1 syntax", k)
		}
	}
	return fm
}

// PackTemplateContextFuncsV2 returns the funcMap for the V2 Pack template
// context. These are added to other template functions provided in the
// Renderer
func PackTemplateContextFuncsV2() template.FuncMap {
	return template.FuncMap{
		"vars":      getPackVars,
		"var":       getPackVar,
		"must_var":  mustGetPackVar,
		"metas":     getPackMetas,
		"meta":      getPackMeta,
		"must_meta": mustGetPackMeta,
		"deps":      getPackDeps,
		"deps_tree": getPackDepTree,
	}
}

// getPackVars is the underlying implementation for the `vars` template func
func getPackVars(p PackContextable) map[string]any { return p.getVars() }

// getPackVar is the underlying implementation for the `var` template func
func getPackVar(k string, p PackContextable) any {
	if v, err := mustGetPackVar(k, p); err == nil {
		return v
	} else {
		return ""
	}
}

// mustGetPackVar is the underlying implementation for the `must_var` template
// func
func mustGetPackVar(k string, p PackContextable) (any, error) {
	return mustGetPackVarR(strings.Split(k, "."), p.getVars())
}

// mustGetPackMetaR recursively descends into a pack's variable map to collect
// the values.
func mustGetPackVarR(keys []string, p map[string]any) (any, error) {
	if len(keys) > 0 {
		np, found := p[keys[0]]
		if !found {
			// TODO: This should probably be the full traversal to this point accumulated.
			return nil, fmt.Errorf("var key %s not found", keys[0])
		}

		if found && len(keys) == 1 {
			return np, nil
		}

		// If we're here, there's more than one key remaining in the traversal.
		// See if we can continue
		switch item := np.(type) {
		case string:
			return nil, fmt.Errorf("encountered non-traversable key while traversing")

		case map[string]any:
			return mustGetPackVarR(keys[1:], item)
		}
	}

	return nil, errors.New("var key not found")
}

// getPackMetas is the underlying implementation for the `metas` template func
func getPackMetas(p PackContextable) map[string]any { return p.getMetas() }

// mustGetPackMeta is the underlying implementation for the `must_meta` template
// func
func mustGetPackMeta(k string, p PackContextable) (any, error) {
	return mustGetPackMetaR(strings.Split(k, "."), p.getMetas())
}

// mustGetPackMetaR recursively descends into a pack's metadata map to collect
// the values.
func mustGetPackMetaR(keys []string, p map[string]any) (any, error) {
	if len(keys) == 0 {
		return nil, errors.New("end of traversal")
	}
	np, found := p[keys[0]]
	if !found {
		return nil, fmt.Errorf("meta key %s not found", keys[0])
	}

	switch item := np.(type) {
	case string:
		if len(keys) == 1 {
			return item, nil
		}
		return nil, fmt.Errorf("encountered non-map key while traversing")
	case map[string]any:
		if len(keys) == 1 {
			return nil, fmt.Errorf("traversal ended on non-metadata item key")
		}
		return mustGetPackMetaR(keys[1:], item)
	default:
		return nil, fmt.Errorf("meta key not found and hit non-traversible type (%T)", np)
	}
}

// getPackMeta is the underlying implementation for the `meta` template func
func getPackMeta(k string, p PackContextable) any {
	if v, err := mustGetPackMeta(k, p); err == nil {
		return v
	} else {
		return ""
	}
}

func getPackDeps(p PackTemplateContext) PackTemplateContext {
	out := make(PackTemplateContext, len(p)-1)
	for k, v := range p {
		if k != "_self" {
			out[k] = v
		}
	}
	return out
}

func getPackDepTree(p PackTemplateContext) []string {
	if len(p) <= 1 {
		return []string{}
	}

	pAcc := new([]string)

	for _, k := range p.depKeys() {
		v := p[k]
		path := "." + k
		*pAcc = append(*pAcc, path)
		ptc := v.(PackTemplateContext)
		getPackDepTreeR(ptc, path, pAcc)

	}
	return *pAcc
}

func getPackDepTreeR(p PackTemplateContext, path string, pAcc *[]string) {
	if len(p) <= 1 {
		return
	}

	for _, k := range p.depKeys() {
		v := p[k]
		path = path + "." + k
		*pAcc = append(*pAcc, path)
		ptc := v.(PackTemplateContext)
		getPackDepTreeR(ptc, path, pAcc)
	}
}

func (p PackTemplateContext) depKeys() []string {
	out := make([]string, 0, len(p)-1)
	for k := range p {
		if k == "_self" {
			continue
		}
		out = append(out, k)
	}
	slices.Sort(out)
	return out
}

func (p PackTemplateContext) Name() string {
	return p.getPack().Pack.Name()
}
