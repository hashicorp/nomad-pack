// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package errors

import (
	"errors"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"text/template"

	"github.com/hashicorp/nomad-pack/internal/pkg/variable/parser"
)

// PackTemplateErrors are designed to progressively enhance the errors returned
// from go template rendering. Implements `error`
type PackTemplateError struct {
	Filename    string   // template filename returned
	Line        int      // the line in the template if found
	StartChar   int      // the character number in the template if found
	EndChar     int      // the character number calculated as the end if an "at" is found
	Err         error    // the last element in given error text when split by ": "
	Details     string   // some additional help text for specific known error patterns
	Suggestions []string // some suggestions to add to the error context
	Extra       []string // remaining splits between the beginning elements and the last one which is the error

	origErr    error
	badElement string
	at         string
	tplctx     parser.PackTemplateContext
}

// Error implements the `error` interface using a value receiver so it works
// with PackTemplateError values or pointers.
func (p PackTemplateError) Error() string {
	return p.Err.Error()
}

// ParseTemplateError returns a PackTemplate error that wraps and attempts to
// enhance errors returned from go template.
func ParseTemplateError(tplCtx parser.PackTemplateContext, err error) *PackTemplateError {
	out := &PackTemplateError{Err: err, origErr: err, tplctx: tplCtx}

	var execErr template.ExecError
	if errors.As(err, &execErr) {
		out.parseExecError(execErr)
	}

	return out
}

// ToWrappedUIContext converts a PackTemplateError into a WrappedUIContext for
// display to the CLI
func (p *PackTemplateError) ToWrappedUIContext() *WrappedUIContext {
	errCtx := NewUIErrorContext()
	if p.Details != "" {
		errCtx.Add(UIContextErrorDetail, p.Details)
	}

	errCtx.Add(UIContextErrorFilename, p.Filename)
	errCtx.Add(UIContextErrorPosition, p.pos())
	if len(p.Suggestions) > 0 {
		errCtx.Add(UIContextErrorSuggestion, strings.Join(p.Suggestions, "; "))
	}
	return &WrappedUIContext{
		Err:     p,
		Subject: "error executing template",
		Context: errCtx,
	}
}

func (p *PackTemplateError) pos() string {
	var out string
	if p.Line == 0 {
		return ""
	}
	out = fmt.Sprintf("Ln %v", p.Line)
	start := p.StartChar
	if p.StartChar == 0 {
		p.StartChar = 1
	}
	out += fmt.Sprintf(", Col %d", start)
	return out
}

// parseExecError attempts to decode the textual representation of a go
// template ExecError. To quote the Go text/template source:
//
// > "TODO: It would be nice if ExecError was more broken down, but
// the way ErrorContext embeds the template name makes the
// processing too clumsy."
func (p *PackTemplateError) parseExecError(execErr template.ExecError) {
	// Exchange wrapped error for unwrapped one, in case we bail out early
	p.Err = execErr
	p.Filename = execErr.Name

	// If there is a source at the beginning of the error, enhanceSource will
	// parse it into the struct and then pop it off.
	p.extractSource()

	// We should be able to split off the last `: ` and have it be the error proper.
	if parts := strings.Split(p.Err.Error(), ": "); len(parts) > 1 {
		p.Extra = parts
	}

	// Tee up a reasonable error value. We'll try to enhance it, but on any error
	// after here we'll return this since it's "good enough"
	p.Err = errors.New(p.Extra[len(p.Extra)-1])

	// Maybe we can do better on the "parser.PackContextable" bit if it shows up
	if strings.Contains(p.Err.Error(), "parser.PackContextable") {
		p.fixupPackContextable()
	}

	p.enhance()
}

func (p *PackTemplateError) extractSource() {
	var a, b string
	found := true

	in := p.Err.Error() // in is mutated to over time

	for found {
		if b, a, found = strings.Cut(in, ": "); found {
			// the first element is "template"
			if b == "template" {
				in = a
				continue
			}
			// the filename component might have line and character details
			if parts := strings.Split(b, ":"); len(parts) > 1 {
				if len(parts) == 3 {
					if charInt, err := strconv.Atoi(parts[2]); err == nil {
						p.StartChar = charInt
					}
				}
				if len(parts) >= 2 {
					if lineInt, err := strconv.Atoi(parts[1]); err == nil {
						p.Line = lineInt
					}
				}
				p.Filename = parts[0]
				in = a
				continue
			}
		}
	}
}

func (p *PackTemplateError) enhance() {
	if p.isNPE() {
		p.enhanceNPE()
	}
	if p.isV2Error() {
		p.enhanceV2Error()
	}
	if ok, ce := p.hasCallingError(); ok {
		// In this case, the calling error makes more sense as the given error
		// and the template runtime error is more of a detail--switch them around.
		p.Details = p.Error()
		p.Err = fmt.Errorf("%v", ce)
	}
}

func (p *PackTemplateError) hasCallingError() (bool, string) {
	const errorCallingPrefix = "error calling "
	for _, e := range p.Extra {
		if strings.HasPrefix(e, errorCallingPrefix) {
			return true,
				fmt.Sprintf(
					"%s`%s`",
					errorCallingPrefix,
					strings.TrimPrefix(e, errorCallingPrefix),
				)
		}
	}
	return false, ""
}

func (p *PackTemplateError) isNPE() bool {
	return strings.HasPrefix(p.Err.Error(), "nil pointer")
}

func (p *PackTemplateError) enhanceNPE() {
	// Nil pointer exceptions could have some _somewhat_ common reasons
	if strings.HasPrefix(p.badElement, ".") {
		// If they are trying to access a context element starting with a dot
		// directly, then they are probably using the old syntax. Since I'm not
		// 100% on this yet, I'm going to reorganize the error message a little
		// and add some information to DidYouMean
		p.Err = fmt.Errorf("Pack %q not found when accessing %q", strings.TrimPrefix(p.badElement, "."), p.at)
		p.Details = "The referenced pack was not found in the template context."
		if parts := strings.Split(p.at, "."); len(parts) == 3 && parts[0] == "" {
			// This case very much looks like the old .packname.varname. Let's try to
			// make a sugestion with the details.
			atPackName := parts[1]
			rootPackName := p.tplctx.Name()

			if atPackName == rootPackName || atPackName == "my" {
				atPackName = ""
			}

			p.Suggestions = []string{
				// TODO: This error will need to be modified if parser-v1 becomes unsupported.
				fmt.Sprintf(
					"The legacy %q syntax should be updated to use `var %q .%s`. You can run legacy packs unmodified by using the `--parser-v1` flag",
					p.at,
					parts[2],
					atPackName,
				),
			}
		}
	}
}

func (p *PackTemplateError) isV2Error() bool {
	return strings.HasSuffix(p.Err.Error(), "not implemented for nomad-pack's v1 syntax")
}

func (p *PackTemplateError) enhanceV2Error() {
	p.Suggestions = []string{"Verify that the `--parser-v1` flag is not set when running this pack."}

}

func (p *PackTemplateError) fixupPackContextable() {
	const typeConst = "parser.PackContextable"
	errStr := p.Err.Error()

	// attempt to extract the "parser.PackContextable.blah" part.
	varRefStr := ""

	// Since there's no variable component to the regex, we can use MustCompile
	pRE := regexp.MustCompile(`(?m)^.*(parser\.PackContextable\.[\w]+)(?:[[:space:]]|$)`)

	// If there's a match, FindStringSubmatch will return 2 matches: one for the
	// whole string and one for the capture group
	if matches := pRE.FindStringSubmatch(errStr); len(matches) == 2 {
		varRefStr = strings.TrimPrefix(matches[1], typeConst)
	} else {
		// If we can't extract the variable reference from the error, then bail out
		return
	}

	atRE := regexp.MustCompile(`^executing .* at <(.+)>$`)

	for _, e := range p.Extra {
		// if it matches, there should be 2 items in the matches slice
		if matches := atRE.FindStringSubmatch(e); len(matches) == 2 {
			// We have an "at" token value at this point. This should provide some
			// context with which to replace the text "variable.PackContextable",
			// potentially other type values too.
			p.at = matches[1]
			p.EndChar = p.StartChar + len(p.at)

			// At this point, we should have the variable reference and an at
			// reference. The combination of these two items _should_ be able
			// to make a rational description of what they were accessing
			// without exposing the variable.PackContextable type component.
			if bad, _, found := strings.Cut(p.at, varRefStr); found {
				p.badElement = bad
				break
			}
		}
	}
	p.Err = errors.New(strings.ReplaceAll(errStr, typeConst+varRefStr, p.badElement))
}
