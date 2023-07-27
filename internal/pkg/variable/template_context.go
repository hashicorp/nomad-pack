package variable

import (
	"fmt"
	"text/template"

	"github.com/hashicorp/nomad-pack/sdk/pack"
	"golang.org/x/exp/slices"
)

type PackData struct {
	Pack *pack.Pack
	meta map[string]string
	vars map[string]any
}

func (p PackData) getVars() map[string]any { return p.vars }

type PackTemplateContext map[string]PackContextable

func (p PackTemplateContext) getVars() map[string]any { return p.getPack().vars }
func (p PackTemplateContext) getPack() PackData       { return p["_self"].(PackData) }

type PackContextable interface{ getVars() map[string]any }

func getPackVars(p PackContextable) map[string]any { return p.getVars() }

func mustGetPackVar(k string, p PackContextable) (any, error) {
	if v, ok := p.getVars()[k]; ok {
		return v, nil
	} else {
		return nil, fmt.Errorf("variable %q not found", k)
	}
}

func getPackVar(k string, p PackContextable) any {
	if v, err := mustGetPackVar(k, p); err == nil {
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
		path := path + "." + k
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
	o := p["_self"].(PackData).Pack
	return o.Name()
}

func PackTemplateContextFuncs() template.FuncMap {
	return template.FuncMap{
		"vars":      getPackVars,
		"var":       getPackVar,
		"must_var":  mustGetPackVar,
		"deps":      getPackDeps,
		"deps_tree": getPackDepTree,
	}
}
