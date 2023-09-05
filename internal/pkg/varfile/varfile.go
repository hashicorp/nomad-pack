package varfile

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/hclsyntax"
	"github.com/hashicorp/hcl/v2/json"
	"github.com/hashicorp/nomad-pack/sdk/pack"
	"github.com/hashicorp/nomad-pack/sdk/pack/variables"
	"golang.org/x/exp/slices"
)

// Alias the SDK types that are used a lot
type PackID = pack.PackID
type VariableID = variables.VariableID

func DecodeVariableOverrides(files []*pack.File) DecodeResult {
	decodeResult := DecodeResult{}
	for _, file := range files {
		fileDecodeResult := DecodeResult{
			Overrides: make(Overrides),
		}
		fileDecodeResult.HCLFiles, fileDecodeResult.Diags = Decode(file.Name, file.Content, nil, &fileDecodeResult.Overrides)
		decodeResult.Merge(fileDecodeResult)
	}
	return decodeResult
}

// DecodeResult is returned by the
type DecodeResult struct {
	Overrides Overrides
	Diags     hcl.Diagnostics
	HCLFiles  map[string]*hcl.File
}

func (d *DecodeResult) Merge(in DecodeResult) {
	// If the incoming DecodeResult contains diags, add them to our Diags
	// collection
	d.Diags = d.Diags.Extend(in.Diags)

	// For each of the incoming DecodeResults Overrides, which is a PackID-keyed
	// map of Pack Variables and their Values.
	for packID, packOverrides := range in.Overrides {

		// Traverse the incoming pack's overrides
		for _, inOverride := range packOverrides {

			// If any existing values in the destination pack's slice conflict with
			// the current value, they will be stored here since otherwise they'd be
			// out of scope
			var match *Override

			// exactMatch is used to signal that the two Override values have the
			// same pointer address so that the code can special case that.
			var exactMatch bool

			if slices.ContainsFunc(d.Overrides[packID], func(e *Override) bool {
				// If either one of the values is nil, then it isn't a match for this
				// purpose.
				if inOverride == nil || d.Overrides[packID] == nil {
					return false
				}

				// Set the sentinel bool in the unlikely case that there is an
				// exact match on the pointer address, so that the later code
				// can choose how to deal with it.
				if exactMatch = e == inOverride; exactMatch {
					return true
				}

				// If an override has the same name and path it is a conflict
				if e.Name == inOverride.Name && e.Path == inOverride.Path {
					// Retain the conflicting existing value's pointer so it can be used
					// in generating an error
					match = e
					return true
				}

				return false
			}) {

				// Since it's an exact match, the destination already contains the
				// incoming override; skip it
				if exactMatch {
					continue
				}

				// Since there's an existing override in the destination, add an HCL
				// diagnostic to the destination
				d.Diags = d.Diags.Append(&hcl.Diagnostic{
					Severity: hcl.DiagError,
					Summary:  "Duplicate definition",
					Detail:   fmt.Sprintf("The variable %s can not be redefined. Existing definition found at %s ", inOverride.Name, match.Range),
					Subject:  &inOverride.Range,
				})

				// Continue in order to collect more diagnostics if they happen
				continue
			}

			// Add the Override to the destination Overrides struct
			d.Overrides[packID] = append(d.Overrides[packID], inOverride)
		}
	}
	if d.HCLFiles == nil && in.HCLFiles != nil {
		d.HCLFiles = in.HCLFiles
	}

	for n, f := range in.HCLFiles {
		d.HCLFiles[n] = f
	}
}

// Decode parses, decodes, and evaluates expressions in the given HCL source
// code, in a single step.
func Decode(filename string, src []byte, ctx *hcl.EvalContext, target *Overrides) (map[string]*hcl.File, hcl.Diagnostics) {
	fm, diags := decode(filename, src, ctx, target)
	var fd = fixableDiags(diags)

	fm.Fixup() // the hcl.File that we will return to the diagnostic printer will have our modifications
	fd.Fixup() // The Ranges in the diags will all be based on our modified byte slice

	return fm, hcl.Diagnostics(fd)
}

// Decode parses, decodes, and evaluates expressions in the given HCL source
// code, in a single step.
func decode(filename string, src []byte, ctx *hcl.EvalContext, target *Overrides) (diagFileMap, hcl.Diagnostics) {
	var file *hcl.File
	var diags hcl.Diagnostics

	// fm is a map of HCL filename to *hcl.File so the caller can use a
	// hcl.DiagnosticWriter and pretty print the errors with contextual
	// information.
	var fm = make(diagFileMap)

	// Select the appropriate parser based on the file's extension
	switch suffix := strings.ToLower(filepath.Ext(filename)); suffix {
	case ".hcl":
		wrapHCLBytes(&src)
		file, diags = hclsyntax.ParseConfig(src, filename, hcl.Pos{Line: 1, Column: 1})
		fm[filename] = file
	case ".json":
		wrapJSONBytes(&src)
		file, diags = json.Parse(src, filename)
		fm[filename] = file
	default:
		diags = diags.Append(&hcl.Diagnostic{
			Severity: hcl.DiagError,
			Summary:  "Unsupported file format",
			Detail:   fmt.Sprintf("Cannot read from %s: unrecognized file format suffix %q.", filename, suffix),
		})
	}

	// Any diags set at this point aren't recoverable, so return them.
	if diags.HasErrors() {
		return fm, diags
	}

	// Because we wrap the user provided values in a HCL map or JSON object both
	// named `v`, we can use JustAttributes to parse the configuration, and obtain
	// the user-supplied content getting the `v` attribute's Expression and using
	// ExprMap to convert it into a []hcl.KeyValuePair
	attrs, diags := file.Body.JustAttributes()
	expr := attrs["v"].Expr
	em, emDiags := hcl.ExprMap(expr)

	// Any diags set at this point still aren't partially usable, so return them.
	if emDiags.HasErrors() {
		return fm, diags.Extend(emDiags)
	}

	vals := make([]*Override, 0, len(em))
	for _, kv := range em {

		// Read the value. If that generates diags, collect them, and stop
		// processing this item.
		value, vDiags := kv.Value.Value(nil)
		if vDiags.HasErrors() {
			diags = diags.Extend(vDiags)
			continue
		}

		// `steps` are the path components, so named because in the HCL case, they
		// are gleaned by waking through the steps in a traversal and getting their
		// names.
		var steps []string

		// Read the key. Start by seeing if there are HCL variables in it, which is
		// what a dotted identifier looks like in the HCL syntax case.
		keyVars := kv.Key.Variables()
		switch len(keyVars) {
		case 0:
			// In the JSON case, there's no tricks necessary to get the key, but
			// we split on . to make the steps slice so the cases converge nicely.

			// I don't think there'd be a way to get diags in this case, so let's
			// ignore them for the time being.
			k, _ := kv.Key.Value(nil)
			steps = strings.Split(k.AsString(), ".")

		case 1:
			// In the HCL case, we have to read the traversal to get the path parts.
			steps = traversalToName(keyVars[0])

		default:
			// If this happens, then there's something really wrong with my algorithm
			// This might have to be relaxed after testing it a while.
			panic("Too many variables in key name")
		}

		// Create a range that represents the sum of the key and value ranges.
		oRange := hcl.RangeBetween(kv.Key.Range(), kv.Value.Range())
		fixupRange(&oRange)

		val := Override{
			Name:  VariableID(steps[len(steps)-1]),
			Path:  pack.PackID(strings.Join(steps[0:len(steps)-1], ".")),
			Value: value,
			Type:  value.Type(),
			Range: oRange,
		}
		vals = append(vals, &val)
	}

	if len(vals) > 0 {
		(*target)[PackID(filename)] = vals
	}
	return fm, diags
}

// wrapHCLBytes takes simple key-value structured HCL and converts them to HCL
// map syntax for parsing
func wrapHCLBytes(sp *[]byte) {
	wrapBytes(sp, []byte("v = {\n"), []byte("\n}"))
}

// wrapHCLBytes takes simple map structured JSON and converts it to HCL
// object syntax for parsing
func wrapJSONBytes(sp *[]byte) {
	wrapBytes(sp, []byte(`{"v":`+"\n"), []byte("\n}"))
}

// wrapBytes is a convenience function to make wrapping byte slices easier
// to read
func wrapBytes(b *[]byte, prefix, postfix []byte) {
	*b = append(append(prefix, *b...), postfix...)
}

// unwrapBytes reverses the changes made by wrapHCLBytes and wrapJSONBytes
func unwrapBytes(sp *[]byte) {
	src := *sp
	// Trim the first 6 and last 2 bytes (since we added those).
	out := slices.Clip(src[6 : len(src)-2])
	*sp = out
}

func fixupRange(r *hcl.Range) {
	fixupPos(&r.Start)
	fixupPos(&r.End)
}

func fixupPos(p *hcl.Pos) {

	// Adjust the byte position to account for the map wrapper that we have to
	// take back out
	p.Byte -= 6

	// Some ranges, especially the "Context" ones, might refer to the line we
	// insert to cheat into parsing the value as a map. Setting it to the home
	// position on line two works nicely with the subtraction we have to do in
	// all the other cases.
	if p.Line == 1 {
		p.Byte = 0   // Step on the computed Byte val, because it will be negative
		p.Line = 2   // Set to 2 since Line is always decremented by one
		p.Column = 1 // The first column aligns with the zero byte.
	}

	p.Line -= 1
}

// DiagExtraFixup is a custom type for the sentinel value stored in Extra that
// indicates whether or not the Ranges in the Diagnostic have been reset to be
// consistent with the user-supplied input.
type DiagExtraFixup struct{ Fixed bool }

// markFixed adds the sentinel value to the passed Diagnostic
func markFixed(d *hcl.Diagnostic) { d.Extra = DiagExtraFixup{Fixed: true} }

// The HCL DiagnosticsWriter can use a map of filenames to parsed HCL files to
// enrich the Diagnostic's output with actual file content.
type diagFileMap map[string]*hcl.File

// Fixup removes the data added to the original inputs so that they print out
// as they were originally provided.
func (d *diagFileMap) Fixup() {
	// We need to fix all of the byte arrays so that they have the original data
	for _, f := range *d {
		unwrapBytes(&f.Bytes)
	}
}

// fixableDiags
type fixableDiags hcl.Diagnostics

// Fixup adjusts the ranges of Diagnostics that have ranges different than they
// should because of the manipulation of the input data. For example, the Pack
// v2 variable override files are modified dynamically to transform them into
// HCL2 maps, which allows up to cheat and use dotted identifiers. However, this
// modification causes the Ranges to be incorrect on any hcl.Diagnostic based on
// them. Fixup calculates where the ranges would have originally referred to.
func (f *fixableDiags) Fixup() {
	for _, diag := range *f {
		if diag.Extra == nil {
			if diag.Subject != nil {
				fixupRange(diag.Subject)
			}
			if diag.Context != nil {
				fixupRange(diag.Context)
			}
			markFixed(diag)
		}
	}
}
