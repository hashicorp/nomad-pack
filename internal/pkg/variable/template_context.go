package variable

import (
	"fmt"
	"text/template"

	"github.com/hashicorp/nomad-pack/sdk/pack"
	"golang.org/x/exp/slices"
)

type PackData struct {
	Pack *pack.Pack
	meta map[string]any
	vars map[string]any
}

func (p PackData) getVars() map[string]any  { return p.vars }
func (p PackData) getMetas() map[string]any { return p.meta }
func (p PackData) getPack() PackData        { return p }

type PackTemplateContext map[string]PackContextable

func (p PackTemplateContext) getVars() map[string]any  { return p.getPack().vars }
func (p PackTemplateContext) getMetas() map[string]any { return p.getPack().meta }
func (p PackTemplateContext) getPack() PackData        { return p["_self"].(PackData) }

type PackContextable interface {
	getVars() map[string]any
	getMetas() map[string]any
}

// getPackVars is the underlying implementation for the `vars` template func
func getPackVars(p PackContextable) map[string]any { return p.getVars() }

// mustGetPackVar is the underlying implementation for the `must_var` template
// func
func mustGetPackVar(k string, p PackContextable) (any, error) {
	if v, ok := p.getVars()[k]; ok {
		return v, nil
	} else {
		return nil, fmt.Errorf("variable %q not found", k)
	}
}

// getPackVar is the underlying implementation for the `var` template func
func getPackVar(k string, p PackContextable) any {
	if v, err := mustGetPackVar(k, p); err == nil {
		return v
	} else {
		return ""
	}
}

// getPackMetas is the underlying implementation for the `metas` template func
func getPackMetas(p PackContextable) map[string]any { return p.getMetas() }

// mustGetPackMeta is the underlying implementation for the `must_meta` template
// func
func mustGetPackMeta(k string, p PackContextable) (any, error) {
	if v, ok := p.getVars()[k]; ok {
		return v, nil
	} else {
		return nil, fmt.Errorf("variable %q not found", k)
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

// mustGetPackVar is the underlying implementation for the `must_var` template
// func
func mustGetPackDep(k string, p PackContextable) (any, error) {
	if v, ok := p.getVars()[k]; ok {
		return v, nil
	} else {
		return nil, fmt.Errorf("variable %q not found", k)
	}
}
func getPackDep(k string, p PackTemplateContext) PackTemplateContext {
	if v, ok := p[k].(PackTemplateContext); ok {
		return v
	}
	return nil
}

func getPackDepTree(p PackTemplateContext) []string {
	if len(p) <= 1 {
		fmt.Println("getPackDepTree: base case")
		return []string{}
	}

	pAcc := new([]string)

	for _, k := range p.depKeys() {
		v := p[k]
		path := "." + k
		*pAcc = append(*pAcc, path)
		ptc := v.(PackTemplateContext)
		getPackDepTreeR(k, ptc, path, pAcc)

	}
	return *pAcc
}

func getPackDepTreeR(k string, p PackTemplateContext, path string, pAcc *[]string) {
	if len(p) <= 1 {
		return
	}

	for _, k := range p.depKeys() {
		v := p[k]
		path = path + "." + k
		*pAcc = append(*pAcc, path)
		ptc := v.(PackTemplateContext)
		getPackDepTreeR(k, ptc, path, pAcc)
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

func PackTemplateContextFuncs() template.FuncMap {
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
