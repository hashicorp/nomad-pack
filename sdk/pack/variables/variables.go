package variables

import (
	"fmt"
	"strings"

	"github.com/hashicorp/nomad-pack/internal/pkg/errors/packdiags"
	"github.com/hashicorp/nomad-pack/sdk/pack"

	"github.com/hashicorp/hcl/v2"
	"github.com/mitchellh/go-wordwrap"
	"github.com/zclconf/go-cty/cty"
	"github.com/zclconf/go-cty/cty/convert"
)

type PackIDKeyedVarMap map[pack.ID][]*Variable

type ID string

func (p ID) String() string { return string(p) }

// Variable encapsulates a single variable as defined within a block according
// to variableFileSchema and variableBlockSchema.
type Variable struct {

	// Name is the variable label. This is used to identify variables being
	// overridden and during templating.
	Name ID

	// Description is an optional field which provides additional context to
	// users identifying what the variable is used for.
	Description    string
	hasDescription bool

	// Default is an optional field which provides a default value to be used
	// in the absence of a user-provided value. It is only in this struct for
	// documentation purposes
	Default    cty.Value
	hasDefault bool

	// Type represents the concrete cty type of this variable. If the type is
	// unable to be parsed into a cty type, it is invalid.
	Type    cty.Type
	hasType bool

	// Value stores the variable value and is used when converting the cty type
	// value into a Go type value.
	Value cty.Value

	// DeclRange is the position marker of the variable within the file it was
	// read from. This is used for diagnostics.
	DeclRange hcl.Range
}

func (v *Variable) SetDescription(d string) { v.Description = d; v.hasDescription = true }
func (v *Variable) SetDefault(d cty.Value)  { v.Default = d; v.hasDefault = true }
func (v *Variable) SetType(t cty.Type)      { v.Type = t; v.hasType = true }

func (v *Variable) Equal(ivp *Variable) bool {
	if v == ivp {
		return true
	}
	cv, ov := *v, *ivp
	eq := cv.Name == ov.Name &&
		cv.Description == ov.Description &&
		cv.hasDescription == ov.hasDescription &&
		cv.Default == ov.Default &&
		cv.hasDefault == ov.hasDefault &&
		cv.Type == ov.Type &&
		cv.hasType == ov.hasType &&
		cv.Value == ov.Value

	return eq
}

func (v *Variable) AsOverrideString(pID pack.ID) string {
	var out strings.Builder
	out.WriteString(fmt.Sprintf(`# variable "%s"`, v.Name))
	out.WriteByte('\n')
	if v.hasDescription {
		tmp := "description: " + v.Description
		wrapped := wordwrap.WrapString(tmp, 80)
		lines := strings.Split(wrapped, "\n")
		for i, l := range lines {
			lines[i] = "#   " + l
		}
		wrapped = strings.Join(lines, "\n")
		out.WriteString(wrapped)
		out.WriteString("\n")
	}
	if v.hasType {
		out.WriteString(fmt.Sprintf("#   type: %s\n", printType(v.Type)))
	}

	if v.hasDefault {
		out.WriteString(fmt.Sprintf("#   default: %s\n", printDefault(v.Default)))
	}

	if v.Value.Equals(v.Default).True() {
		out.WriteString(fmt.Sprintf("#\n# %s=%s\n\n", v.Name, printDefault(v.Default)))
	} else {
		out.WriteString(fmt.Sprintf("#\n%s=%s\n\n", v.Name, printDefault(v.Value)))
	}

	out.WriteString("\n")
	return out.String()
}

func (v *Variable) Merge(in *Variable) hcl.Diagnostics {
	var diags hcl.Diagnostics
	if in.Default != cty.NilVal {
		v.hasDefault = in.hasDefault
		v.Default = in.Default
	}

	if in.Value != cty.NilVal {
		v.Value = in.Value
	}

	if in.Type != cty.NilType {
		v.hasType = in.hasType
		v.Type = in.Type
	}

	if v.Value != cty.NilVal {
		val, err := convert.Convert(v.Value, v.Type)
		if err != nil {
			switch {
			case in.Type != cty.NilType && in.Value == cty.NilVal:
				diags = diags.Append(packdiags.DiagInvalidDefaultValue(
					fmt.Sprintf("Overriding this variable's type constraint has made its default value invalid: %s.", err),
					in.DeclRange.Ptr(),
				))
			case in.Type == cty.NilType && in.Value != cty.NilVal:
				diags = diags.Append(packdiags.DiagInvalidDefaultValue(
					fmt.Sprintf("The overridden default value for this variable is not compatible with the variable's type constraint: %s.", err),
					in.DeclRange.Ptr(),
				))
			default:
				diags = diags.Append(packdiags.DiagInvalidDefaultValue(
					fmt.Sprintf("This variable's default value is not compatible with its type constraint: %s.", err),
					in.DeclRange.Ptr(),
				))
			}
		} else {
			v.Value = val
		}
	}

	return diags
}
