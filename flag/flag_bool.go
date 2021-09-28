package flag

import (
	"os"
	"strconv"

	"github.com/posener/complete"
)

// -- BoolVar  and boolValue
type BoolVar struct {
	Name       string
	Aliases    []string
	Usage      string
	Default    bool
	Hidden     bool
	EnvVar     string
	Target     *bool
	Completion complete.Predictor
	SetHook    func(val bool)
}

type BoolVarP struct {
	*BoolVar
	Shorthand string
}

// optional interface to indicate boolean flags that can be
// supplied without "=value" text
type boolFlag interface {
	String() string
	Set(string) error
	Type() string
	IsBoolFlag() bool
}

func (f *Set) BoolVar(i *BoolVar) {
	f.BoolVarP(&BoolVarP{
		BoolVar:   i,
		Shorthand: "",
	})
}

func (f *Set) BoolVarP(i *BoolVarP) {
	def := i.Default
	if v, exist := os.LookupEnv(i.EnvVar); exist {
		if b, err := strconv.ParseBool(v); err == nil {
			def = b
		}
	}

	f.VarFlagP(&VarFlagP{
		VarFlag: &VarFlag{
			Name:       i.Name,
			Aliases:    i.Aliases,
			Usage:      i.Usage,
			Default:    strconv.FormatBool(i.Default),
			EnvVar:     i.EnvVar,
			Value:      newBoolValue(i, def, i.Target, i.Hidden),
			Completion: i.Completion,
		},
		Shorthand: i.Shorthand,
	})

	// Manually set no option default values so the user can pass the flag without
	// having to also pass in the boolean arg. Pflag automatically takes care of this
	// with boolean flags, but since everything in the internal flag pkg calls var
	// flags, we need to set the value ourselves
	f.unionSet.Lookup(i.Name).NoOptDefVal = "true"
}

type boolValue struct {
	v      *BoolVarP
	hidden bool
	target *bool
}

func newBoolValue(v *BoolVarP, def bool, target *bool, hidden bool) *boolValue {
	*target = def

	return &boolValue{
		v:      v,
		hidden: hidden,
		target: target,
	}
}

func (b *boolValue) Set(s string) error {
	v, err := strconv.ParseBool(s)
	if err != nil {
		return err
	}

	*b.target = v

	if b.v.SetHook != nil {
		b.v.SetHook(v)
	}

	return nil
}

func (b *boolValue) Get() interface{} { return *b.target }
func (b *boolValue) String() string   { return strconv.FormatBool(*b.target) }
func (b *boolValue) Example() string  { return "" }
func (b *boolValue) Hidden() bool     { return b.hidden }
func (b *boolValue) IsBoolFlag() bool { return true }
func (b *boolValue) Type() string     { return "bool" }
